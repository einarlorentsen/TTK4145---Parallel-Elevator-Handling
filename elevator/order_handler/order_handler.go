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
