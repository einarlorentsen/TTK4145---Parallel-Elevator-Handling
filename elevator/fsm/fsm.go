package fsm

import (
	"time"

	"../../constant"
	"../elevio"
	"../order_handler"
)

/* Initialize the elevator and stop at first floor above our starting position */
func InitFSM() {
	currFloor := elevio.GetFloorInit()
	if currFloor == -1 { //If not in floor, go up until you reach a floor
		ch_floor := make(chan int)
		go elevio.PollFloorSensor(ch_floor)
		elevio.SetMotorDirection(elevio.MD_Up)
		currFloor = <-ch_floor
	}
	elevio.SetFloorIndicator(currFloor)
	elevio.SetMotorDirection(elevio.MD_Stop)
}

/* Main fsm function for elevator */
func ElevFSM(ch_masterMatrixRx <-chan [][]int, ch_cabOrderRx <-chan []int, ch_dirTx chan<- int, ch_floorTx chan<- int, ch_stateTx chan<- constant.STATE, ch_cabServed chan<- int) {

	//Sets default values for variables used in ElevFSM
	var lastElevDir elevio.MotorDirection
	var newElevDir elevio.MotorDirection
	var localState constant.STATE
	var masterMatrix [][]int
	var flagTimerActive bool = false

	masterMatrix = initEmptymasterMatrix()
	ch_floorRx := make(chan int, constant.N_FLOORS) //Sends last registred floor for elevator
	cabOrders := make([]int, constant.N_FLOORS)
	currentFloor := elevio.GetFloorInit()
	elevio.SetFloorIndicator(currentFloor)

	lastElevDir = elevio.MD_Up
	//End set default values for variables used in ElevFSM

	go elevio.PollFloorSensor(ch_floorRx)

	for {
		switch localState {
			//If elevator is in IDLE, listen for new Orders and update your masterMatrix and cabOrders when you recieves changes in them
		case constant.IDLE:
			elevio.SetMotorDirection(elevio.MD_Stop)
			ch_dirTx <- int(elevio.MD_Stop)
			ch_stateTx <- localState

		checkIDLE:
			for {
				select {
				case updatemasterMatrix := <-ch_masterMatrixRx:
					order_handler.SetHallLights(updatemasterMatrix)
					masterMatrix = updatemasterMatrix

					//If new order found, set the apropriate motorDirection
					newElevDir = checkQueue(currentFloor, lastElevDir, masterMatrix, cabOrders)
					if newElevDir == elevio.MD_Stop {
						ch_dirTx <- int(newElevDir)
						localState = constant.DOORS_OPEN
						break checkIDLE
					} else if newElevDir != elevio.MD_Idle {
						lastElevDir = newElevDir
						ch_dirTx <- int(newElevDir)
						localState = constant.MOVE
						break checkIDLE
					}
				case updateCabOrders := <-ch_cabOrderRx:
					cabOrders = updateCabOrders

					//If new order found, set the apropriate motorDirection
					newElevDir = checkQueue(currentFloor, lastElevDir, masterMatrix, cabOrders)
					if newElevDir == elevio.MD_Stop {
						ch_dirTx <- int(newElevDir)
						localState = constant.DOORS_OPEN
						break checkIDLE
					} else if newElevDir != elevio.MD_Idle {
						lastElevDir = newElevDir
						ch_dirTx <- int(newElevDir)
						localState = constant.MOVE
						break checkIDLE
					}
				default:
				}
			}

			//While in MOVE, recieve info from Master, listen for new orders and check if you should stop when new floor reached
		case constant.MOVE:
			elevio.SetMotorDirection(newElevDir)
			ch_stateTx <- localState
		checkMOVE:
			for {
				select {
				case updatemasterMatrix := <-ch_masterMatrixRx:
					order_handler.SetHallLights(updatemasterMatrix)
					masterMatrix = updatemasterMatrix
				case updateCabOrders := <-ch_cabOrderRx:
					cabOrders = updateCabOrders
				case floor := <-ch_floorRx:
					currentFloor = floor

					//If new floor detected. Check if you should stop or continue moving
					newElevDir = checkQueue(currentFloor, lastElevDir, masterMatrix, cabOrders)
					elevio.SetFloorIndicator(currentFloor)
					ch_floorTx <- floor

					if newElevDir == elevio.MD_Stop {
						localState = constant.STOP
						ch_dirTx <- int(newElevDir)
						break checkMOVE
					} else if newElevDir != elevio.MD_Idle {
						lastElevDir = newElevDir
						localState = constant.MOVE
						elevio.SetMotorDirection(newElevDir)
						ch_dirTx <- int(newElevDir)
					} else if newElevDir == elevio.MD_Idle {
						localState = constant.IDLE
						newElevDir = elevio.MD_Stop
						break checkMOVE
					}
				}
				if localState != constant.MOVE {
					// fmt.Println("BREAK MOVE")
					break checkMOVE
				}
			}
			break

			//STOP elevator and send your new state to master
		case constant.STOP:
			newElevDir = elevio.MD_Stop
			ch_stateTx <- localState
			ch_dirTx <- int(newElevDir)
			elevio.SetMotorDirection(elevio.MD_Stop)
			localState = constant.DOORS_OPEN
			break

			//Hold Door open for timer duration, while recieving and sending info from/to master
		case constant.DOORS_OPEN:
			ch_timerKill := make(chan bool)
			ch_timerFinished := make(chan bool)
			if flagTimerActive == false {
				go doorTimer(ch_timerKill, ch_timerFinished)
				flagTimerActive = true
			}
			ch_stateTx <- localState
			ch_cabServed <- currentFloor

			elevio.SetDoorOpenLamp(true)
			cabOrders[currentFloor] = 0
			index := IndexFinder(masterMatrix)
		checkDOORSOPEN:
			for {
				select {
				case updatemasterMatrix := <-ch_masterMatrixRx:
					order_handler.SetHallLights(updatemasterMatrix)
					masterMatrix = updatemasterMatrix
					index = IndexFinder(masterMatrix)
				case updateCabOrders := <-ch_cabOrderRx:
					cabOrders = updateCabOrders
				case <-ch_timerFinished:
					elevio.SetDoorOpenLamp(false)
					flagTimerActive = false
					localState = constant.IDLE
					ch_stateTx <- localState
					break checkDOORSOPEN

				default:
					if cabOrders[currentFloor] == 1 || masterMatrix[index][currentFloor] == 1 {
						cabOrders[currentFloor] = 0 // Resets order at current floor
					}
				}
				break
			}
		}
	}
}


/*---------------- Help functions for initFSM() and ElevFSM -------------------*/

/* Initializes an empty masterMatrix with FIRST_FLOOR + N_FLOORS length */
func initEmptymasterMatrix() [][]int {
	masterMatrix := make([][]int, 0)
	for i := 0; i <= 2; i++ {
		masterMatrix = append(masterMatrix, make([]int, constant.FIRST_FLOOR+constant.N_FLOORS))
	}
	return masterMatrix
}

/* Checks if elevator has orders in its current floor, above it or below it, and takes the appropriate action */
func checkQueue(currentFloor int, lastElevDir elevio.MotorDirection, masterMatrix [][]int, cabOrders []int) elevio.MotorDirection {
	var direction elevio.MotorDirection = elevio.MD_Idle
	for i := 0; i < 2; i++ {
	}
	for row := int(constant.FIRST_ELEV); row < len(masterMatrix); row++ {
		if masterMatrix[row][constant.IP] == constant.LocalIP { //Check if order in current floor
			if masterMatrix[row][int(constant.FIRST_FLOOR)+currentFloor] == 1 || cabOrders[currentFloor] == 1 {
				return elevio.MD_Stop
			}
			switch {
			case lastElevDir == elevio.MD_Up: // If last direction was UP, Check above elevator
				direction = checkAbove(row, currentFloor, masterMatrix, cabOrders)
				if direction == elevio.MD_Idle {
					direction = checkBelow(row, currentFloor, masterMatrix, cabOrders)
				}
				return direction

			case lastElevDir == elevio.MD_Down: // If last direction was DOWN, Check below elevator
				direction = checkBelow(row, currentFloor, masterMatrix, cabOrders)
				if direction == elevio.MD_Idle {
					direction = checkAbove(row, currentFloor, masterMatrix, cabOrders)
				}
				return direction
			default:
				// Nothing
			}
		}
	}

	return direction
}

/* Used in checkQueue to check if are any orders above elevator */
func checkAbove(row int, currentFloor int, masterMatrix [][]int, cabOrders []int) elevio.MotorDirection {
	for floor := (int(constant.FIRST_FLOOR) + currentFloor + 1); floor < len(masterMatrix[0]); floor++ {
		if masterMatrix[row][floor] == 1 || cabOrders[floor-int(constant.FIRST_FLOOR)] == 1 {
			return elevio.MD_Up
		}
	}
	return elevio.MD_Idle
}

/* Used in checkQueue to check if are any orders below elevator */
func checkBelow(row int, currentFloor int, masterMatrix [][]int, cabOrders []int) elevio.MotorDirection {
	for floor := (int(constant.FIRST_FLOOR) + currentFloor - 1); floor >= int(constant.FIRST_FLOOR); floor-- {
		if masterMatrix[row][floor] == 1 || cabOrders[floor-int(constant.FIRST_FLOOR)] == 1 {
			return elevio.MD_Down
		}
	}
	return elevio.MD_Idle
}

/* Timer for holding the door open. Sends true to timerFinished when timer has run out */
func doorTimer(timerKill <-chan bool, timerFinished chan<- bool) {
	timer := time.NewTimer(3 * time.Second)
	for {
		select {
		case <-timerKill:
			timer.Stop()
			return
		case <-timer.C:
			timer.Stop()
			timerFinished <- true
			return
		}
	}
}

/* Looks through masterMatrix, and finds at which row the elevators ID is located */
func IndexFinder(masterMatrix [][]int) int {
	rows := len(masterMatrix)
	for index := 0; index < rows; index++ {
		if masterMatrix[index][constant.IP] == constant.LocalIP {
			return index
		}
	}
	return -1
}
