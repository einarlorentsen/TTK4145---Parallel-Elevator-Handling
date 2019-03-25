package elevator

import (
	"fmt"
	"sync"

	"../constant"
	"../file_IO"
	"../master_slave_fsm"
	"./elevio"
	"./fsm"
	"./order_handler"
)

var _mtx sync.Mutex
var elevIndex int

// var LocalMatrix [][]int

/* Set lights */

/* Current floor light */
func CurrentFloorLight(floor int) {
	if floor != -1 {
		elevio.SetFloorIndicator(floor)
	}
}

/* */
func InitElevator(ch_elevTransmit chan<- [][]int, ch_elevRecieve <-chan [][]int) {
	ch_hallOrder := make(chan elevio.ButtonEvent) // Hall orders sent over channel
	ch_cabOrder := make(chan elevio.ButtonEvent)  // Cab orders sent over channel
	ch_dir := make(chan constant.FIELD)           // Channel for DIR updates
	ch_floor := make(chan constant.FIELD)         // Channel for FLOOR updates
	ch_state := make(chan fsm.STATE)              // Channel for STATE updates

	order_handler.InitLocalElevatorMatrix()

	cabOrdersBackup := file_IO.ReadFile(constant.BACKUP_FILENAME) // Matrix
	if len(cabOrdersBackup) == 0 {
		fmt.Println("No backups found.")
		order_handler.InitCabOrders([]int{})
	} else {
		fmt.Println("Backup found.") /*  */
		_mtx.Lock()
		order_handler.InitCabOrders(cabOrdersBackup[0])
	}

	// Button updates over their respective channels
	go order_handler.UpdateOrderMatrix(ch_hallOrder, ch_cabOrder)
	// Listen for elevator updates, send the update to master/slave module.
	go order_handler.ListenElevator(ch_elevTransmit, ch_dir, ch_floor, ch_state, ch_hallOrder)
	/* .. STUFF */

	// elevatorHandler(ch_elevTransmit, ch_elevRecieve)
}

/*  */
// func elevatorHandler(ch_elevTransmit chan<- [][]int, ch_elevRecieve <-chan [][]int) {
//
// 	// var cabOrders = make([]int, int(constant.N_FLOORS))
//
// 	/* Stuff */
//
// 	for {
// 		// select {
// 		// case matrixMaster := <-ch_elevRecieve:
// 		// 	// Extract light-matrix
// 		// 	// Extract stops
// 		//
// 		// }
// 	}
//
// }

func updateMasterMatrix(ch_elevRecieve <-chan [][]int, ch_copyMatrixMaster chan<- [][]int) {
	var copyMatrixMaster [][]int = master_slave_fsm.InitMatrixMaster()
	var recievedMatrix [][]int
	for {
		select {
		case recievedMatrix = <-ch_elevRecieve:
			if checkMatrixUpdate(recievedMatrix, copyMatrixMaster) == true {
				copyMatrixMaster = recievedMatrix
				_mtx.Lock()
				elevIndex = indexFinder(copyMatrixMaster)
				_mtx.Unlock()
				ch_copyMatrixMaster <- copyMatrixMaster

			}
		default:
			//Do absolutly nothing
		}
	}
}

func checkMatrixUpdate(currentMatrix [][]int, prevMatrix [][]int) bool {
	rowLength := len(currentMatrix)
	colLength := len(currentMatrix[0])
	for row := 0; row < rowLength; row++ {
		for col := 0; col < colLength; col++ {
			if currentMatrix[row][col] != prevMatrix[row][col] {
				return true
			}
		}
	}
	return false
}

func indexFinder(matrixMaster [][]int) int {
	rows := len(matrixMaster)
	for index := 0; index < rows; index++ {
		if matrixMaster[index][constant.IP] == master_slave_fsm.LocalIP {
			return index
		}
	}
	return -1
}
