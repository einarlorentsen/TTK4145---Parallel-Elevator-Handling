package order_handler

import (
	"fmt"
	"sync"

	"../../constant"
	"../../file_IO"
	"../../master_slave_fsm"
	"../elevio"
	// "github.com/kentare/exercise-4-pipeline/elevio"
)

var _mtx sync.Mutex
var cabOrders []int
var LocalMatrix [][]int

func InitCabOrders(fromBackup []int) {
	// for i := 0; i < constant.N_FLOORS; i++ {
	// 	if len(fromBackup) > i {
	// 		cabOrders = append(cabOrders, fromBackup[i])
	// 	} else {
	// 		cabOrders = append(cabOrders, 0)
	// 	}
	// }
	for i := 0; i < constant.N_FLOORS; i++ {
		cabOrders = append(cabOrders, 0)
	}
}

func InitLocalElevatorMatrix() {
	_mtx.Lock()
	defer _mtx.Unlock()
	LocalMatrix = master_slave_fsm.InitLocalMatrix()
	LocalMatrix[constant.UP_BUTTON][constant.IP] = master_slave_fsm.LocalIP
	LocalMatrix[constant.UP_BUTTON][constant.DIR] = int(elevio.MD_Stop)
	fmt.Println("initElevatorMatrix: NOT POLLING FLOOR SENSOR")
	LocalMatrix[constant.UP_BUTTON][constant.FLOOR] = 2 //<-ch_floorSensor
	LocalMatrix[constant.UP_BUTTON][constant.ELEV_STATE] = int(constant.IDLE)
	LocalMatrix[constant.UP_BUTTON][constant.SLAVE_MASTER] = int(constant.MASTER)
}

/* Polls all buttons and sends recieved orders out on their respective channels */
func UpdateOrderMatrix(ch_hallOrder chan<- elevio.ButtonEvent, ch_cabOrder chan<- elevio.ButtonEvent) {
	fmt.Println("UpdateOrderMatrix: Init")
	ch_pollButtons := make(chan elevio.ButtonEvent)
	var order elevio.ButtonEvent
	go elevio.PollButtons(ch_pollButtons) // Returns slice [floor, button]
	for {
		select {
		case order = <-ch_pollButtons:
			fmt.Println("UpdateOrderMatrix: Recieved ch_pollButtons")
			if order.Button == elevio.BT_Cab {
				ch_cabOrder <- order
			} else {
				ch_hallOrder <- order
			}
		}
	}
}

//Recieves the floor that has a set cab order and sets the flag in that floor
func updateCabOrders(ch_cabOrder <-chan int) {
	var tmpBackup [][]int
	for {
		index := <-ch_cabOrder
		_mtx.Lock()
		cabOrders[index] = 1
		tmpBackup = append(tmpBackup, cabOrders)
		_mtx.Unlock()
		file_IO.WriteFile(constant.BACKUP_FILENAME, tmpBackup)
		setCabLights()

	}
}

func setCabLights() {
	for floor := 0; floor < len(cabOrders); floor++ {
		if cabOrders[floor] == 1 {
			elevio.SetButtonLamp(elevio.BT_Cab, floor, true)
		} else if cabOrders[floor] == 0 {
			elevio.SetButtonLamp(elevio.BT_Cab, floor, false)
		}
	}
}

func setHallLights(matrixMaster [][]int) {
	for index := int(constant.FIRST_FLOOR); index < len(matrixMaster[constant.UP_BUTTON]); index++ {
		if matrixMaster[constant.UP_BUTTON][index] == 1 {
			elevio.SetButtonLamp(elevio.BT_HallUp, index-int(constant.FIRST_FLOOR), true)
		} else if matrixMaster[constant.UP_BUTTON][index] == 0 {
			elevio.SetButtonLamp(elevio.BT_HallUp, index-int(constant.FIRST_FLOOR), false)
		}

		if matrixMaster[constant.DOWN_BUTTON][index] == 1 {
			elevio.SetButtonLamp(elevio.BT_HallDown, int(constant.FIRST_FLOOR)-index, true)
		} else if matrixMaster[constant.DOWN_BUTTON][index] == 0 {
			elevio.SetButtonLamp(elevio.BT_HallDown, int(constant.FIRST_FLOOR)-index, false)
		}
	}
}

/* Listens on updates from elevator fsm and updates the elevators local matrix */
func ListenElevator(ch_elevTx chan<- [][]int, ch_dir <-chan constant.FIELD, ch_floor <-chan constant.FIELD, ch_state <-chan constant.STATE, ch_hallOrder <-chan elevio.ButtonEvent) {
	for {
		fmt.Println("ListenElevator: Waiting on updates.")
		select {
		case dir := <-ch_dir:
			writeLocalMatrix(ch_elevTx, int(constant.UP_BUTTON), int(constant.DIR), int(dir))
		case floor := <-ch_floor:
			writeLocalMatrix(ch_elevTx, int(constant.UP_BUTTON), int(constant.FLOOR), int(floor))
		case state := <-ch_state:
			writeLocalMatrix(ch_elevTx, int(constant.UP_BUTTON), int(constant.ELEV_STATE), int(state))
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
	LocalMatrix[row][col] = value
	ch_elevTx <- LocalMatrix
}
