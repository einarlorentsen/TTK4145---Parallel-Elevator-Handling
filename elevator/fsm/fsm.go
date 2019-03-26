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
			// Let igjennom cabOrders og masterMatrix etter bestillinger. Vi foretrekker bestillinger
			// i forrige registrerte retning.
			// Hvis vi finner en bestilling sett retning til opp eller ned utifra hvor bestillingen er
			// og hopp til MOVE state. Hvis du finner en bestiling i etasjen du allerede er i - hopp til DOORS OPEN

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
