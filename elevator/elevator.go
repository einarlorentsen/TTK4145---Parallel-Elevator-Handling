package elevator

import (
	"fmt"
	"sync"

	"../file_IO"
	"../master_slave_fsm"
	"./elevio"
	"./fsm"
	"./order_handler"
)

var _mtx sync.Mutex
var elevIndex int

/* Set lights */

/* Current floor light */
func CurrentFloorLight(floor int) {
	if floor != -1 {
		elevio.SetFloorIndicator(floor)
	}
}

/* */
func InitElevator(ch_elevTransmit chan<- [][]int, ch_elevRecieve <-chan [][]int) {
	ch_hallOrder := make(chan elevio.ButtonEvent)                   // Hall orders sent over channel
	ch_cabOrder := make(chan elevio.ButtonEvent)                    // Cab orders sent over channel
	cabOrders := file_IO.ReadFile(master_slave_fsm.BACKUP_FILENAME) // Matrix
	if len(cabOrders) == 0 {
		fmt.Println("No backups found.")
	} else {
		fmt.Println("Backup found.") /*  */
		//initialCabOrders := cabOrders[0]
	}

	go order_handler.UpdateOrderMatrix(ch_hallOrder, ch_cabOrder) // Button updates over these channels

	/* .. STUFF */
	elevatorHandler(ch_elevTransmit, ch_elevRecieve)
}

/*  */
func elevatorHandler(ch_elevTransmit chan<- [][]int, ch_elevRecieve <-chan [][]int) {
	localMatrix := initLocalElevatorMatrix()
	var cabOrders = make([]int, int(master_slave_fsm.N_FLOORS))

	/* Stuff */

	for {
		select {
		case matrixMaster := <-ch_elevRecieve:
			// Extract light-matrix
			// Extract stops

		}
	}

}

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
		if matrixMaster[index][master_slave_fsm.IP] == master_slave_fsm.LocalIP {
			return index
		}
	}
	return -1
}

func initLocalElevatorMatrix() [][]int {
	localMatrix := master_slave_fsm.InitLocalMatrix()
	localMatrix[master_slave_fsm.UP_BUTTON][master_slave_fsm.IP] = master_slave_fsm.LocalIP
	localMatrix[master_slave_fsm.UP_BUTTON][master_slave_fsm.DIR] = int(elevio.MD_Stop)
	fmt.Println("initElevatorMatrix: NOT POLLING FLOOR SENSOR")
	localMatrix[master_slave_fsm.UP_BUTTON][master_slave_fsm.FLOOR] = 2 //<-ch_floorSensor
	localMatrix[master_slave_fsm.UP_BUTTON][master_slave_fsm.ELEV_STATE] = int(fsm.IDLE)
	localMatrix[master_slave_fsm.UP_BUTTON][master_slave_fsm.SLAVE_MASTER] = int(master_slave_fsm.MASTER)
	return localMatrix
}
