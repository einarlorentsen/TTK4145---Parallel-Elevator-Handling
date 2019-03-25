package fsm

import (
	"sync"

	"../../constant"
	"../elevio"
)

// var ElevState constant.STATE
var ElevDir elevio.MotorDirection
var _mtx_dir sync.Mutex

func InitFSM() {
	// Init matrix to keep track of the various states of the elevator
	// order_handler.InitLocalElevatorMatrix()
	// Up to first FLOOR
	currFloor := elevio.GetFloorInit()
	if currFloor == -1 {
		ch_floor := make(chan int)
		go elevio.PollFloorSensor(ch_floor)
		// elevSetMotorDirection(elevio.MD_Up)
		currFloor = <-ch_floor
	}
	// elevSetMotorDirection(elevio.MD_Stop)
	// elevSetState(constant.IDLE)
}

func ElevFSM(ch_matrixMasterRx <-chan [][]int, ch_cabOrderRx <-chan []int, ch_dirTx chan<- constant.FIELD, ch_floorTx chan<- int, ch_stateTx chan<- constant.STATE) {
	var localState constant.STATE
	var matrixMaster [][]int
	ch_floorRx := make(chan int)
	cabOrders := make([]int, constant.N_FLOORS)

	go elevio.PollFloorSensor(ch_floorRx)

	for {
		switch localState {
		case constant.IDLE:
			// for select: cabOrder, matrixMaster

		case constant.MOVE:
			for {
				select {
				case floor := <-ch_floorRx:
					// Check cabOrder
					// Check other orders (i.e. stop here or completely if no orders)
					// Send floor signal
					ch_floorTx <- floor
				case cabOrders = <-ch_cabOrderRx:
				}
			}

		case constant.STOP:

		case constant.DOORS_OPEN:
			// for select: cabOrders -- Is handled by order_handler
			// for select: matrixMaster
			// check matrixMaster for new orders/stops

		case constant.DOORS_CLOSED:
			// ImmEDIATE TRANSISION
		}
	}
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
