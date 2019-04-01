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
	if currFloor == -1 { // If not in floor, go up until elevator reaches floor
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
	for i := 0; i <= 2; i++ {
		matrixMaster = append(matrixMaster, make([]int, constant.FIRST_FLOOR+constant.N_FLOORS))
	}
	return matrixMaster
}

/* Main fsm function for elevator */
func ElevFSM(ch_matrixMasterRx <-chan [][]int, ch_cabOrderRx <-chan []int, ch_dirTx chan<- int, ch_floorTx chan<- int, ch_stateTx chan<- constant.STATE, ch_cabServed chan<- int) {

	//Sets default values for variables used in ElevFSM
	var lastElevDir elevio.MotorDirection
	var newElevDir elevio.MotorDirection
	var localState constant.STATE
	var matrixMaster [][]int
	var flagTimerActive bool = false

	matrixMaster = initEmptyMatrixMaster()
	ch_floorRx := make(chan int, constant.N_FLOORS) //Sends last registred floor for elevator
	cabOrders := make([]int, constant.N_FLOORS)
	currentFloor := elevio.GetFloorInit()
	elevio.SetFloorIndicator(currentFloor)

	lastElevDir = elevio.MD_Up
	localState = constant.IDLE
	//End set default values for variables used in ElevFSM

	go elevio.PollFloorSensor(ch_floorRx)

	for {
		switch localState {
		//If elevator is in IDLE, listen for newOrders and update your masterMatrix and cabOrders when you recieves changes in them
		case constant.IDLE:
			elevio.SetMotorDirection(elevio.MD_Stop)
			ch_dirTx <- int(elevio.MD_Stop)
			ch_stateTx <- localState

		checkIDLE:
			for {
				select {
				case updateMatrixMaster := <-ch_matrixMasterRx:
					order_handler.SetHallLights(updateMatrixMaster)
					matrixMaster = updateMatrixMaster
					newElevDir = checkQueue(currentFloor, lastElevDir, matrixMaster, cabOrders)

					//If new order found, set the apropriate motorDirection
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
					newElevDir = checkQueue(currentFloor, lastElevDir, matrixMaster, cabOrders)
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

			//While in MOVE, recieve info from master, listen for new orders and check if elevator should stop when new floor reached
		case constant.MOVE:
			elevio.SetMotorDirection(newElevDir)
			ch_stateTx <- localState
		checkMOVE:
			for {
				select {
				case updateMatrixMaster := <-ch_matrixMasterRx:
					order_handler.SetHallLights(updateMatrixMaster)
					matrixMaster = updateMatrixMaster
				case updateCabOrders := <-ch_cabOrderRx:
					cabOrders = updateCabOrders
				case floor := <-ch_floorRx:
					currentFloor = floor

					//If new floor detected: Check if elevator should stop or continue moving
					newElevDir = checkQueue(currentFloor, lastElevDir, matrixMaster, cabOrders)
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

			//Hold door open for timer duration, while recieving and sending info from/to master
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
			index := IndexFinder(matrixMaster)
		checkDOORSOPEN:
			for {
				select {
				case updateMatrixMaster := <-ch_matrixMasterRx:
					order_handler.SetHallLights(updateMatrixMaster)
					matrixMaster = updateMatrixMaster
					index = IndexFinder(matrixMaster)
				case updateCabOrders := <-ch_cabOrderRx:
					cabOrders = updateCabOrders
				case <-ch_timerFinished:
					elevio.SetDoorOpenLamp(false)
					flagTimerActive = false
					localState = constant.IDLE
					ch_stateTx <- localState
					break checkDOORSOPEN

				default:
					if cabOrders[currentFloor] == 1 || matrixMaster[index][currentFloor] == 1 {
						cabOrders[currentFloor] = 0
					}
				}
			}
			break
		}
	}
}

/*------------ Help functions for initFSM and ElevFSM --------------------*/

/* Initializes an empty masterMatrix with FIRST_FLOOR + N_FLOORS length */
func checkCurrentFloor(row int, currentFloor int, matrixMaster [][]int, cabOrders []int) elevio.MotorDirection {
	if matrixMaster[row][constant.IP] == constant.LocalIP { //Check if order in current floor
		if matrixMaster[row][int(constant.FIRST_FLOOR)+currentFloor] == 1 || cabOrders[currentFloor] == 1 {
			return elevio.MD_Stop
		}
	}
	return elevio.MD_Idle
}

/* Check if elevator has orders in its current floor, above itself or below itself and takes the appropriate action */
func checkQueue(currentFloor int, lastElevDir elevio.MotorDirection, matrixMaster [][]int, cabOrders []int) elevio.MotorDirection {
	var direction elevio.MotorDirection = elevio.MD_Idle
	for row := int(constant.FIRST_ELEV); row < len(matrixMaster); row++ {
		if matrixMaster[row][constant.IP] == constant.LocalIP { //Check if order in current floor
			if matrixMaster[row][int(constant.FIRST_FLOOR)+currentFloor] == 1 || cabOrders[currentFloor] == 1 {
				return elevio.MD_Stop
			}
			switch {
			case lastElevDir == elevio.MD_Up: // If last direction was UP, Check above elevator
				direction = checkAbove(row, currentFloor, matrixMaster, cabOrders)
				if direction == elevio.MD_Idle {
					direction = checkBelow(row, currentFloor, matrixMaster, cabOrders)
				}
				return direction

			case lastElevDir == elevio.MD_Down: // If last direction was DOWN, Check below elevator
				direction = checkBelow(row, currentFloor, matrixMaster, cabOrders)
				if direction == elevio.MD_Idle {
					direction = checkAbove(row, currentFloor, matrixMaster, cabOrders)
				}
				return direction
			default:
			}
		}
	}
	return direction
}

/* Used in checkQueue to check if there are any orders above elevator */
func checkAbove(row int, currentFloor int, matrixMaster [][]int, cabOrders []int) elevio.MotorDirection {
	for floor := (int(constant.FIRST_FLOOR) + currentFloor + 1); floor < len(matrixMaster[0]); floor++ {
		if matrixMaster[row][floor] == 1 || cabOrders[floor-int(constant.FIRST_FLOOR)] == 1 {
			return elevio.MD_Up
		}
	}
	return elevio.MD_Idle
}

/* Used in checkQueue to check if there are any orders below elevator */
func checkBelow(row int, currentFloor int, matrixMaster [][]int, cabOrders []int) elevio.MotorDirection {
	for floor := (int(constant.FIRST_FLOOR) + currentFloor - 1); floor >= int(constant.FIRST_FLOOR); floor-- {
		if matrixMaster[row][floor] == 1 || cabOrders[floor-int(constant.FIRST_FLOOR)] == 1 {
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

/* Looks through matrixMaster and finds at which row the elevators ID is located */
func IndexFinder(matrixMaster [][]int) int {
	rows := len(matrixMaster)
	for index := 0; index < rows; index++ {
		if matrixMaster[index][constant.IP] == constant.LocalIP {
			return index
		}
	}
	return -1
}
