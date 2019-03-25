package order_handler

import (
	"sync"

	"..../constant"
	"../elevio"
	// "github.com/kentare/exercise-4-pipeline/elevio"
)

var _mtx sync.Mutex
var cabOrder []int

/* Polls all buttons and sends recieved orders out on their respective channels */
func UpdateOrderMatrix(ch_hallOrder chan<- elevio.ButtonEvent, ch_cabOrder chan<- elevio.ButtonEvent) {
	ch_pollButtons := make(chan elevio.ButtonEvent)
	var order elevio.ButtonEvent
	go elevio.PollButtons(ch_pollButtons) // Returns slice [floor, button]
	for {
		select {
		case order = <-ch_pollButtons:
			if order.Button == elevio.BT_Cab {
				ch_cabOrder <- order
			} else {
				ch_hallOrder <- order
			}
		}
	}
}

//Recieves the floor that has a set cab order and sets the flag in that floor
func updateCabOrders(ch_cabOrder <-chan []int) {
	for {
		index := <-ch_cabOrder
		_mtx.Lock()
		cabOrder[index] = 1
		_mtx.Unlock()
		setCabLights()

	}
}

func setCabLights() {
	for floor := 0; floor < len(cabOrders); floor++ {
		if cabOrders[floor] == 1 {
			elevio.SetButtonLamp(elevio.BT_Cab, floor, true)
		} else if cabOrders[index] == 0 {
			elevio.SetButtonLamp(elevio.BT_Cab, index, false)
		}
	}
}

func setHallLights(matrixMaster [][]int) {
	for index := constant.FIRST_FLOOR; index < len(masterMatrix[constant.UP_BUTTON]); row++ {
		if masterMatrix[constant.UP_BUTTON][index] == 1 {
			elevio.SetButtonLamp(elevio.BT_HallUp, index-constant.FIRST_FLOOR, true)
		} else if masterMatrix[constant.UP_BUTTON][index] == 0 {
			elevio.SetButtonLamp(elevio.BT_HallUp, index-constant.FIRST_FLOOR, false)
		}

		if masterMatrix[constant.DOWN_BUTTON][index] == 1 {
			elevio.SetButtonLamp(elevio.BT_Hall_Down, constant.FIRST_FLOOR-index, true)
		} else if masterMatrix[constant.DOWN_BUTTON][index] == 0 {
			elevio.SetButtonLamp(elevio.BT_Hall_Down, constant.FIRST_FLOOR-index, false)
		}
	}
}

/* Listens on updates from elevator fsm and updates the elevators local matrix */
func ListenElevator(ch_elevTx, ch_dir <-chan constant.FIELD, ch_floor <-chan constant.FIELD, ch_state <-chan fsm.STATE, ch_hallOrder <-chan elevio.ButtonEvent) {
	for {
		select {
		case dir := <-ch_dir:
			writeLocalMatrix(ch_elevTx, int(constant.UP_BUTTON), int(constant.DIR), int(dir))
		case floor := <-ch_floor:
			writeLocalMatrix(ch_elevTx, int(constant.UP_BUTTON), int(constant.FLOOR), int(floor))
		case state := <-ch_state:
			writeLocalMatrix(ch_elevTx, int(constant.UP_BUTTON), int(constant.STATE), int(state))
		case hallOrder := <-ch_hallOrder:
			if hallOrder.Button == elevio.BT_HallUp {
				writeLocalMatrix(ch_elevTx, int(constant.UP_BUTTON), int(constant.FIRST_FLOOR)+hallOrder.Floor, 1)
			} else if hallOrder.Button == elevio.BT_HallDown {
				writeLocalMatrix(ch_elevTx, int(constant.DOWN_BUTTON), int(constant.FIRST_FLOOR)+hallOrder.Floor, 1)
			}
		}
	}
}
func writeLocalMatrix(ch_elevTx chan<- [][]int, row int, col int, value int) {
	_mtx.Lock()
	defer _mtx.Unlock()
	elevator.LocalMatrix[row][col] = value
	ch_elevTx <- elevator.LocalMatrix
}
