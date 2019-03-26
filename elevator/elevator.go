package elevator

import (
	"fmt"
	"sync"

	"../constant"
	"../file_IO"
	"./elevio"
	"./order_handler"
)

var _mtx sync.Mutex
var elevIndex int

// var LocalMatrix [][]int

/* */
func InitElevator(ch_elevTransmit chan<- [][]int, ch_elevRecieve <-chan [][]int) {
	ch_hallOrder := make(chan elevio.ButtonEvent) // Hall orders sent over channel
	ch_cabOrder := make(chan elevio.ButtonEvent)  // Cab orders sent over channel
	ch_cabServed := make(chan elevio.ButtonEvent)
	ch_dir := make(chan constant.FIELD)   // Channel for DIR updates
	ch_floor := make(chan constant.FIELD) // Channel for FLOOR updates
	ch_state := make(chan constant.STATE) // Channel for STATE updates
	var cabOrders []int
	var localMatrix [][]int
	// order_handler.InitLocalElevatorMatrix()	// Init in fsm.initFSM

	cabOrdersBackup := file_IO.ReadFile(constant.BACKUP_FILENAME) // Matrix
	if len(cabOrdersBackup) == 0 {
		fmt.Println("No backups found.")
		cabOrders = order_handler.InitCabOrders([]int{})
	} else {
		fmt.Println("Backup found.") /*  */
		_mtx.Lock()
		cabOrders = order_handler.InitCabOrders(cabOrdersBackup[0])
	}

	localMatrix = order_handler.InitLocalElevatorMatrix()

	// Button updates over their respective channels
	go order_handler.UpdateOrderMatrix(ch_hallOrder, ch_cabOrder)
	// Listen for elevator updates, send the update to master/slave module.
	go order_handler.UpdateCabOrders(ch_cabOrder, ch_cabServed, cabOrders)
	elevatorHandler(localMatrix, ch_elevTransmit, ch_elevRecieve, ch_dir, ch_floor, ch_state, ch_hallOrder)
}

/* Listens on updates from elevator fsm and updates the elevators local matrix */
func elevatorHandler(localMatrix [][]int, ch_elevTx chan<- [][]int, ch_elevRx <-chan [][]int, ch_dir <-chan constant.FIELD, ch_floor <-chan constant.FIELD, ch_state <-chan constant.STATE, ch_hallOrder <-chan elevio.ButtonEvent) {
	var lastMatrixMaster [][]int
	for {
		fmt.Println("ListenElevator: Waiting on updates.")
		select {
		case matrixMaster := <-ch_elevRx:
			lastMatrixMaster = matrixMaster
		// currentIndex := indexFinder(matrixMaster)
		// Check stops
		// for floors
		// Read orders, distribute
		//
		case dir := <-ch_dir: // Changed direction
			writeLocalMatrix(ch_elevTx, int(constant.UP_BUTTON), int(constant.DIR), int(dir))
		case floor := <-ch_floor: // Arrived at floor
			// Update to floor
			// Update floor lights
			writeLocalMatrix(ch_elevTx, int(constant.UP_BUTTON), int(constant.FLOOR), int(floor))
		case state := <-ch_state: // Changed state
			writeLocalMatrix(ch_elevTx, int(constant.UP_BUTTON), int(constant.ELEV_STATE), int(state))
		case hallOrder := <-ch_hallOrder: // Recieved hall-order
			if hallOrder.Button == elevio.BT_HallUp {
				writeLocalMatrix(ch_elevTx, int(constant.UP_BUTTON), int(constant.FIRST_FLOOR)+hallOrder.Floor, 1)
			} else if hallOrder.Button == elevio.BT_HallDown {
				writeLocalMatrix(ch_elevTx, int(constant.DOWN_BUTTON), int(constant.FIRST_FLOOR)+hallOrder.Floor, 1)
			}
		}
		// Send localmatrix to master/slave through ch_elevTx
	}
}
func writeLocalMatrix(ch_elevTx chan<- [][]int, row int, col int, value int) {
	// _mtx.Lock()
	// defer _mtx.Unlock()
	// localMatrix[row][col] = value
	// ch_elevTx <- localMatrix
}

// func IndexFinder(matrixMaster [][]int) int {
// 	rows := len(matrixMaster)
// 	for index := 0; index < rows; index++ {
// 		if matrixMaster[index][constant.IP] == master_slave_fsm.LocalIP {
// 			return index
// 		}
// 	}
// 	return -1
// }

// func updateMasterMatrix(ch_elevRecieve <-chan [][]int, ch_copyMatrixMaster chan<- [][]int) {
// 	var copyMatrixMaster [][]int = master_slave_fsm.InitMatrixMaster()
// 	var recievedMatrix [][]int
// 	for {
// 		select {
// 		case recievedMatrix = <-ch_elevRecieve:
// 			if checkMatrixUpdate(recievedMatrix, copyMatrixMaster) == true {
// 				copyMatrixMaster = recievedMatrix
// 				_mtx.Lock()
// 				elevIndex = indexFinder(copyMatrixMaster)
// 				_mtx.Unlock()
// 				ch_copyMatrixMaster <- copyMatrixMaster
//
// 			}
// 		default:
// 			//Do absolutly nothing
// 		}
// 	}
// }
//
// func checkMatrixUpdate(currentMatrix [][]int, prevMatrix [][]int) bool {
// 	rowLength := len(currentMatrix)
// 	colLength := len(currentMatrix[0])
// 	for row := 0; row < rowLength; row++ {
// 		for col := 0; col < colLength; col++ {
// 			if currentMatrix[row][col] != prevMatrix[row][col] {
// 				return true
// 			}
// 		}
// 	}
// 	return false
// }
