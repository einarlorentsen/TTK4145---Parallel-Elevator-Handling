package fsm

import (
	"time"

	"fmt"

	"../../constant"
	"../../master_slave_fsm"
	"../elevio"
)

/* Initialize the elevator and stop at first floor above our starting position */
func InitFSM() {
	currFloor := elevio.GetFloorInit()
	if currFloor == -1 {
		ch_floor := make(chan int)
		go elevio.PollFloorSensor(ch_floor)
		elevio.SetMotorDirection(elevio.MD_Up)
		currFloor = <-ch_floor
	}
	elevio.SetFloorIndicator(currFloor)
	elevio.SetMotorDirection(elevio.MD_Stop)
}

func initEmptyMatrixMaster() [][]int {
	matrixMaster := make([][]int, 0)
	for i := 0; i <= 2; i++ { // For 1 elevator, master is assumed alone
		matrixMaster = append(matrixMaster, make([]int, constant.FIRST_FLOOR+constant.N_FLOORS))
	}
	return matrixMaster
}

func ElevFSM(ch_matrixMasterRx <-chan [][]int, ch_cabOrderRx <-chan []int, ch_dirTx chan<- int, ch_floorTx chan<- int, ch_stateTx chan<- constant.STATE) {
	fmt.Println("ElevFSM: Initialized...")
	var lastElevDir elevio.MotorDirection
	var newElevDir elevio.MotorDirection
	var localState constant.STATE
	var matrixMaster [][]int
	matrixMaster = initEmptyMatrixMaster()
	ch_floorRx := make(chan int)
	cabOrders := make([]int, constant.N_FLOORS)
	currentFloor := elevio.GetFloorInit()
	elevio.SetFloorIndicator(currentFloor)

	lastElevDir = elevio.MD_Up // After initialization
	localState = constant.IDLE // Initialize in IDLE

	go elevio.PollFloorSensor(ch_floorRx)

	for {
		switch localState {
		case constant.IDLE:
			fmt.Println("ElevFSM: IDLE")
			elevio.SetMotorDirection(elevio.MD_Stop)
			ch_dirTx <- int(elevio.MD_Stop)
			ch_stateTx <- localState
			// Let igjennom cabOrders og masterMatrix etter bestillinger. Vi foretrekker bestillinger
			// i forrige registrerte retning.
			// Hvis vi finner en bestilling sett retning til opp eller ned utifra hvor bestillingen er
			// og hopp til MOVE state. Hvis du finner en bestiling i etasjen du allerede er i - hopp til DOORS OPEN
			var oldElevDir elevio.MotorDirection // Var for MD check

		checkIDLE:
			for {
				select {
				case updateMatrixMaster := <-ch_matrixMasterRx:
					matrixMaster = updateMatrixMaster
				case updateCabOrders := <-ch_cabOrderRx:
					cabOrders = updateCabOrders
				default:
					newElevDir = checkQueue(currentFloor, lastElevDir, matrixMaster, cabOrders)
					if newElevDir != oldElevDir { // Just a check
						fmt.Println("ElevFSM: newElevDir = ", newElevDir)
						oldElevDir = newElevDir
					}

					if newElevDir == elevio.MD_Stop {
						fmt.Println("ElevFSM: IDLE --> DOORS_OPEN")
						fmt.Println("ElemFSM: IDLE: Waiting on ch_dirTX...")
						ch_dirTx <- int(newElevDir)
						fmt.Println("ElemFSM: IDLE: Sent on ch_dirTX!")
						localState = constant.DOORS_OPEN
						break checkIDLE // Break select
					} else if newElevDir != elevio.MD_Idle {
						fmt.Println("ElevFSM: IDLE --> MOVE")
						lastElevDir = newElevDir
						fmt.Println("ElemFSM: IDLE: Waiting on ch_dirTX...")
						ch_dirTx <- int(newElevDir) // STALLS HERE
						fmt.Println("ElemFSM: IDLE: Sent on ch_dirTX!")
						localState = constant.MOVE
						break checkIDLE // Break select
					}
				}
				if localState != constant.IDLE {
					fmt.Println("ElevFSM: if localState != constant.IDLE")
					break checkIDLE // Break the for-select loop
				}
			}
			fmt.Println("ElevFSM: End IDLE")

		case constant.MOVE:
			fmt.Println("ElevFSM: MOVE")
			elevio.SetMotorDirection(newElevDir)
			ch_stateTx <- localState
			for {
				select {
				case updateMatrixMaster := <-ch_matrixMasterRx:
					matrixMaster = updateMatrixMaster
				case updateCabOrders := <-ch_cabOrderRx:
					cabOrders = updateCabOrders
				case floor := <-ch_floorRx:
					fmt.Println("ElevFSM: Arrived at floor: ", floor)
					currentFloor = floor
					go elevio.SetFloorIndicator(currentFloor)
					newElevDir = checkQueue(currentFloor, lastElevDir, matrixMaster, cabOrders)
					if newElevDir == elevio.MD_Stop {
						localState = constant.STOP
						ch_dirTx <- int(newElevDir)
					} else if newElevDir != elevio.MD_Idle {
						lastElevDir = newElevDir
						localState = constant.MOVE
						elevio.SetMotorDirection(newElevDir)
						ch_dirTx <- int(newElevDir)
					} else if newElevDir == elevio.MD_Idle {
						localState = constant.IDLE
					}
					ch_floorTx <- floor // Send floor to higher layers in the hierarchy
					// Når jeg kommer til en etasje, sjekk om jeg har en bestilling her i CAB eller matrixMaster.
					// Hvis ja - hopp til STOPP state. Hvis nei, sjekk om jeg har en bestilling videre i retningen jeg
					// kjører. Hvis ja, fortsett i MOVE med samme retning. Hvi jeg kun har en bestilling
					// i feil retning, skift retning, hvis jeg ikke har noen bestillinger, sett motorRetning
					// til stopp og hopp til IDLE state.
				}
				if localState != constant.MOVE {
					break
				}
			}

		case constant.STOP:
			fmt.Println("ElevFSM: STOP")
			newElevDir = elevio.MD_Stop
			ch_stateTx <- localState
			ch_dirTx <- int(newElevDir)
			elevio.SetMotorDirection(elevio.MD_Stop)
			localState = constant.DOORS_OPEN
			break

		case constant.DOORS_OPEN:
			fmt.Println("ElevFSM: DOORS_OPEN")
			ch_timerKill := make(chan bool)
			ch_timerFinished := make(chan bool)
			go doorTimer(ch_timerKill, ch_timerFinished)
			flagTimerActive := true
			ch_stateTx <- localState
			elevio.SetDoorOpenLamp(true)
			cabOrders[currentFloor] = 0
			index := IndexFinder(matrixMaster)
			for {
				select {
				case updateMatrixMaster := <-ch_matrixMasterRx:
					matrixMaster = updateMatrixMaster
					index = IndexFinder(matrixMaster)
				case updateCabOrders := <-ch_cabOrderRx:
					cabOrders = updateCabOrders
				case <-ch_timerFinished:
					fmt.Println("doorTimer: ch_timerFinished recieved")
					elevio.SetDoorOpenLamp(false)
					flagTimerActive = false
					localState = constant.IDLE
					ch_stateTx <- localState
					break

				default:
					if cabOrders[currentFloor] == 1 || matrixMaster[index][currentFloor] == 1 {
						cabOrders[currentFloor] = 0
						if flagTimerActive == true {
							ch_timerKill <- true
							flagTimerActive = false
						}
					}
					if cabOrders[currentFloor] == 0 && cabOrders[currentFloor] == matrixMaster[index][int(constant.FIRST_FLOOR)+currentFloor] {
						if flagTimerActive == false {
							go doorTimer(ch_timerKill, ch_timerFinished)
							flagTimerActive = true
						}
					}
				}
			} // End DOORS_OPEN
		}
	}
}

func checkCurrentFloor(row int, currentFloor int, matrixMaster [][]int, cabOrders []int) elevio.MotorDirection {
	if matrixMaster[row][constant.IP] == master_slave_fsm.LocalIP { //Check if order in current floor
		if matrixMaster[row][int(constant.FIRST_FLOOR)+currentFloor] == 1 || cabOrders[currentFloor] == 1 {
			return elevio.MD_Stop
		}
	}
	return elevio.MD_Idle
}

func checkQueue(currentFloor int, lastElevDir elevio.MotorDirection, matrixMaster [][]int, cabOrders []int) elevio.MotorDirection {
	var direction elevio.MotorDirection = elevio.MD_Idle
	// fmt.Println("checkQueue: mM rows: ", len(matrixMaster))
	// fmt.Println("checkQueue: mM cols: ", len(matrixMaster[0]))
	// fmt.Println("checkQueue: cab length: ", len(cabOrders))
	for row := int(constant.FIRST_ELEV); row < len(matrixMaster); row++ {
		if matrixMaster[row][constant.IP] == master_slave_fsm.LocalIP { //Check if order in current floor
			if matrixMaster[row][int(constant.FIRST_FLOOR)+currentFloor] == 1 || cabOrders[currentFloor] == 1 {
				return elevio.MD_Stop
			}
			switch {
			case lastElevDir == elevio.MD_Up: // Check above elevator
				direction = checkAbove(row, currentFloor, matrixMaster, cabOrders)
				if direction == elevio.MD_Idle {
					direction = checkBelow(row, currentFloor, matrixMaster, cabOrders)
				}
				return direction

			case lastElevDir == elevio.MD_Down: // Check below elevator
				direction = checkBelow(row, currentFloor, matrixMaster, cabOrders)
				if direction == elevio.MD_Idle {
					direction = checkAbove(row, currentFloor, matrixMaster, cabOrders)
				}
				return direction
			default:
				// Nothing
			}
		}
	}
	return direction
}

func checkAbove(row int, currentFloor int, matrixMaster [][]int, cabOrders []int) elevio.MotorDirection {
	for floor := (int(constant.FIRST_FLOOR) + currentFloor + 1); floor < len(matrixMaster); floor++ {
		if matrixMaster[row][floor] == 1 || cabOrders[floor-int(constant.FIRST_FLOOR)] == 1 {
			return elevio.MD_Up
		}
	}
	return elevio.MD_Idle
}

func checkBelow(row int, currentFloor int, matrixMaster [][]int, cabOrders []int) elevio.MotorDirection {
	for floor := (int(constant.FIRST_FLOOR) + currentFloor - 1); floor >= int(constant.FIRST_FLOOR); floor-- {
		if matrixMaster[row][floor] == 1 || cabOrders[floor-int(constant.FIRST_FLOOR)] == 1 {
			return elevio.MD_Down
		}
	}
	return elevio.MD_Idle
}

func doorTimer(timerKill <-chan bool, timerFinished chan<- bool) {
	fmt.Println("doorTimer: Initialized")
	timer := time.NewTimer(3 * time.Second)
	for {
		select {
		case <-timerKill:
			fmt.Println("doorTimer: Kill timer")
			return
		case <-timer.C:
			timerFinished <- true
			fmt.Println("doorTimer: Timer finished")
			return
		}
	}
}

func IndexFinder(matrixMaster [][]int) int {
	rows := len(matrixMaster)
	for index := 0; index < rows; index++ {
		if matrixMaster[index][constant.IP] == master_slave_fsm.LocalIP {
			return index
		}
	}
	return -1
}
