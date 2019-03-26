package fsm

import (
	"sync"

	"../../constant"
	"../../master_slave_fsm"
	"../elevio"
)

// var ElevState constant.STATE
var _mtx_dir sync.Mutex

/* Initialize the elevator and stop at first floor above our starting position */
func InitFSM() {
	currFloor := elevio.GetFloorInit()
	if currFloor == -1 {
		ch_floor := make(chan int)
		go elevio.PollFloorSensor(ch_floor)
		elevio.SetMotorDirection(elevio.MD_Up)
		currFloor = <-ch_floor
	}
	elevio.SetMotorDirection(elevio.MD_Stop)
}

func ElevFSM(ch_matrixMasterRx <-chan [][]int, ch_cabOrderRx <-chan []int, ch_dirTx chan<- constant.FIELD, ch_floorTx chan<- int, ch_stateTx chan<- constant.STATE) {
	var lastElevDir elevio.MotorDirection
	var newElevDir elevio.MotorDirection
	var localState constant.STATE
	var matrixMaster [][]int
	ch_floorRx := make(chan int)
	cabOrders := make([]int, constant.N_FLOORS)
	currentFloor := elevio.GetFloorInit()

	lastElevDir = elevio.MD_Up // After initialization

	go elevio.PollFloorSensor(ch_floorRx)

	for {
		switch localState {
		case constant.IDLE:
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
						localState = constant.DOORS_OPEN
					} else if newElevDir != elevio.MD_Idle {
						lastElevDir = newElevDir
						localState = constant.MOVE
					}
				}
			}

		case constant.MOVE:
			for {
				select {
				case floor := <-ch_floorRx:
					// Når jeg kommer til en etasje, sjekk om jeg har en bestilling her i CAB eller matrixMaster.
					// Hvis ja - hopp til STOPP state. Hvis nei, sjekk om jeg har en bestilling videre i retningen jeg
					// kjører. Hvis ja, fortsett i MOVE med samme retning. Hvi jeg kun har en bestilling
					// i feil retning, skift retning, hvis jeg ikke har noen bestillinger, sett motorRetning
					// til stopp og hopp til IDLE state.
					ch_floorTx <- floor
				case cabOrders = <-ch_cabOrderRx:
				}
			}

		case constant.STOP:

		case constant.DOORS_OPEN:
			// Åpne dørene og hold de åpne så lenge som nødvendig.
			// Slukk CAB lys for denne etasjen
			// Når timeren som holder døren åpen går ut, hopp til DOORS_CLOSED

		case constant.DOORS_CLOSED:
			// Lukk døren
			// Hopp til IDLE state
		}
	}
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

// func elevSetMotorDirection(direction elevio.MotorDirection) {
// 	_mtx.Lock()
// 	order_handler.LocalMatrix[constant.UP_BUTTON][constant.DIR] = int(direction)
// 	_mtx.Unlock()
// 	elevio.SetMotorDirection(direction)
// }
//
// func elevSetFloor(floor int) {
// 	_mtx.Lock()
// 	order_handler.LocalMatrix[constant.UP_BUTTON][constant.FLOOR] = floor
// 	_mtx.Unlock()
// }
//
// func elevSetState(state constant.STATE) {
// 	_mtx.Lock()
// 	order_handler.LocalMatrix[constant.UP_BUTTON][constant.ELEV_STATE] = int(state)
// 	ElevState = state
// 	_mtx.Unlock()
// }
