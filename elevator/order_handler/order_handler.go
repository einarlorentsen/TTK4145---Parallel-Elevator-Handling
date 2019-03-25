package order_handler

import (
	"../elevio"
	// "github.com/kentare/exercise-4-pipeline/elevio"
)

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
func updateCabOrders(cabOrders[]int){
	for{
		index := <-ch_cabOrder
		_mtx.Lock()
		cabOrder[index] = 1
		defer _mtx.Unlock()

	}
}

func setLights(matrixMaster[][]int){
	for index = FIRST_FLOOR; index < len(masterMatrix[UP_BUTTON]); row++{
		if masterMatrix[UP_BUTTON][index] == 1{
			SetButtonLamp(BT_HallUp,index-FIRST_FLOOR,true)
		} else if masterMatrix[UP_BUTTON][index] == 0{
			SetButtonLamp(BT_HallUp,index-FIRST_FLOOR,false)
		}

		if masterMatrix[DOWN_BUTTON][index] == 1{
			SetButtonLamp(BT_Hall_Down, FIRST_FLOOR-index, true)
		} else if masterMatrix[DOWN_BUTTON][index] == 0{
				SetButtonLamp(BT_Hall_Down, FIRST_FLOOR-index, false)
		}
	}
}
