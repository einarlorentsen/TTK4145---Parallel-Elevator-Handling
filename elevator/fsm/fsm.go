package fsm

import (
	"time"

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

func ElevFSM(ch_matrixMasterRx <-chan [][]int, ch_cabOrderRx <-chan []int, ch_dirTx chan<- int, ch_floorTx chan<- int, ch_stateTx chan<- constant.STATE) {
	var lastElevDir elevio.MotorDirection
	var newElevDir elevio.MotorDirection
	var localState constant.STATE
	var matrixMaster [][]int
	ch_floorRx := make(chan int)
	cabOrders := make([]int, constant.N_FLOORS)
	currentFloor := elevio.GetFloorInit()
	elevio.SetFloorIndicator(currentFloor)

	lastElevDir = elevio.MD_Up // After initialization

	go elevio.PollFloorSensor(ch_floorRx)

	for {
		switch localState {
		case constant.IDLE:
			elevio.SetMotorDirection(elevio.MD_Stop)
			ch_dirTx <- int(elevio.MD_Stop)
			ch_stateTx <- localState
			// Let igjennom cabOrders og masterMatrix etter bestillinger. Vi foretrekker bestillinger
			// i forrige registrerte retning.
			// Hvis vi finner en bestilling sett retning til opp eller ned utifra hvor bestillingen er
			// og hopp til MOVE state. Hvis du finner en bestiling i etasjen du allerede er i - hopp til DOORS OPEN
			for {
				select {
				case updateMatrixMaster := <-ch_matrixMasterRx:
					matrixMaster = updateMatrixMaster
				case updateCabOrders := <-ch_cabOrderRx:
					cabOrders = updateCabOrders
				default:
					newElevDir = checkQueue(currentFloor, lastElevDir, matrixMaster, cabOrders)
					if newElevDir == elevio.MD_Stop {
						ch_dirTx <- int(newElevDir)
						localState = constant.DOORS_OPEN
					} else if newElevDir != elevio.MD_Idle {
						lastElevDir = newElevDir
						ch_dirTx <- int(newElevDir)
						localState = constant.MOVE
					}
				}
				if localState != constant.IDLE {
					break // Break the for-select loop
				}
			}

		case constant.MOVE:
			elevio.SetMotorDirection(newElevDir)
			ch_stateTx <- localState
			for {
				select {
				case updateMatrixMaster := <-ch_matrixMasterRx:
					matrixMaster = updateMatrixMaster
				case updateCabOrders := <-ch_cabOrderRx:
					cabOrders = updateCabOrders
				case floor := <-ch_floorRx:
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
			newElevDir = elevio.MD_Stop
			ch_stateTx <- localState
			ch_dirTx <- int(newElevDir)
			elevio.SetMotorDirection(elevio.MD_Stop)
			localState = constant.DOORS_OPEN

		case constant.DOORS_OPEN:
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
					elevio.SetDoorOpenLamp(false)
					flagTimerActive = false
					ch_stateTx <- localState
					localState = constant.IDLE

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
			}
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

	for row := int(constant.FIRST_ELEV); row < len(matrixMaster[constant.UP_BUTTON]); row++ {
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
	timer := time.NewTimer(3 * time.Second)
	for {
		select {
		case <-timerKill:
			return
		case <-timer.C:
			timerFinished <- true
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
