package order_handler

import (
	"fmt"

	"../../constant"
	"../../file_IO"
	"../../master_slave_fsm"
	"../elevio"
	// "github.com/kentare/exercise-4-pipeline/elevio"
)

//
// var localMatrix [][]int

func InitCabOrders(fromBackup []int) []int {
	var cabOrders []int
	for i := 0; i < constant.N_FLOORS; i++ {
		if len(fromBackup) > i {
			cabOrders = append(cabOrders, fromBackup[i])
		} else {
			cabOrders = append(cabOrders, 0)
		}
	}
	// for i := 0; i < constant.N_FLOORS; i++ {
	// 	cabOrders = append(cabOrders, 0)
	// }
	return cabOrders
}

func InitLocalElevatorMatrix() [][]int {
	localMatrix := master_slave_fsm.InitLocalMatrix()
	localMatrix[constant.UP_BUTTON][constant.IP] = master_slave_fsm.LocalIP
	localMatrix[constant.UP_BUTTON][constant.DIR] = int(elevio.MD_Stop)
	localMatrix[constant.UP_BUTTON][constant.FLOOR] = elevio.GetFloorInit()
	localMatrix[constant.UP_BUTTON][constant.ELEV_STATE] = int(constant.IDLE)
	localMatrix[constant.UP_BUTTON][constant.SLAVE_MASTER] = int(constant.MASTER)
	return localMatrix
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
func UpdateCabOrders(ch_cabOrder <-chan elevio.ButtonEvent, ch_cabServed <-chan elevio.ButtonEvent, cabOrders []int) {
	var tmpBackup [][]int
	tmpBackup = append(tmpBackup, cabOrders)
	for {
		select {
		case buttonEvent := <-ch_cabOrder:
			cabOrders[buttonEvent.Floor] = 1
			fmt.Println("updateCabOrders: Added cabOrder floor: ", buttonEvent.Floor)
		case buttonEvent := <-ch_cabServed:
			cabOrders[buttonEvent.Floor] = 0
			fmt.Println("updateCabOrders: Deleted cabOrder floor: ", buttonEvent.Floor)
		}
		setCabLights(cabOrders)
		tmpBackup[0] = cabOrders
		file_IO.WriteFile(constant.BACKUP_FILENAME, tmpBackup)
	}
}

func setCabLights(cabOrders []int) {
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
