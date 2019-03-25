package fsm

import (
	"sync"

	"../../constant"
	"../elevio"
	"../order_handler"
)

var ElevState constant.STATE
var ElevDir elevio.MotorDirection
var _mtx sync.Mutex

func InitFSM() {
	// Init matrix to keep track of the various states of the elevator
	order_handler.InitLocalElevatorMatrix()
	// Up to first FLOOR
	currFloor := elevio.GetFloorInit()
	if currFloor == -1 {
		ch_floor := make(chan int)
		go elevio.PollFloorSensor(ch_floor)
		elevSetMotorDirection(elevio.MD_Up)
		currFloor = <-ch_floor
	}
	elevSetMotorDirection(elevio.MD_Stop)
	elevSetState(constant.IDLE)
}

func elevSetMotorDirection(direction elevio.MotorDirection) {
	_mtx.Lock()
	order_handler.LocalMatrix[constant.UP_BUTTON][constant.DIR] = int(direction)
	_mtx.Unlock()
	elevio.SetMotorDirection(direction)
}

func elevSetFloor(floor int) {
	_mtx.Lock()
	order_handler.LocalMatrix[constant.UP_BUTTON][constant.FLOOR] = floor
	_mtx.Unlock()
}

func elevSetState(state constant.STATE) {
	_mtx.Lock()
	order_handler.LocalMatrix[constant.UP_BUTTON][constant.ELEV_STATE] = int(state)
	ElevState = state
	_mtx.Unlock()
}
