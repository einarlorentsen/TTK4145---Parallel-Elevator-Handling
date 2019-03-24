package user_interface

import (
	"fmt"
	"sync"

	"../file_IO"
	"../master_slave_fsm"
	"./elevio"
)

var _mtx sync.Mutex

/* Set lights */

/* Current floor light */
func CurrentFloorLight(floor int) {
	if floor != -1 {
		elevio.SetFloorIndicator(floor)
	}
}

/*  */

/* */
func InitElevator() {

	cabOrders := file_IO.ReadFile(master_slave_fsm.BACKUP_FILENAME) // Matrix
	if len(cabOrders) == 0 {
		fmt.Println("No backups found.")
	} else {
		fmt.Println("Backup found.")
		//initialCabOrders := cabOrders[0]
	}
}

func updateMasterMatrix(ch_elevRecieve <-chan [][]int) {
	var copyMatrixMaster [][]int = master_slave_fsm.InitMatrixMaster()
	var recievedMatrix [][]int
	for {
		select {
		case recievedMatrix = <-ch_elevRecieve:
			if checkMatrixUpdate(recievedMatrix, copyMatrixMaster) == true {
				copyMatrixMaster = recievedMatrix
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
