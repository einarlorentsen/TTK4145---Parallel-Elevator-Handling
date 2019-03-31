package elevator

import (
	"fmt"
	"time"

	"../constant"
	"../file_IO"
	"./elevio"
	"./fsm"
	"./order_handler"
)

var elevIndex int

// var LocalMatrix [][]int

func backupSendToElevator(ch_cabOrderArray chan<- []int, cabOrders []int) {
	ch_cabOrderArray <- cabOrders
}

func TakeElevatorToNearestFloor() {
	fsm.InitFSM()
}

/* */
// func InitElevator(ch_elevTransmit chan<- [][]int, ch_elevRecieve <-chan [][]int, ch_buttonPressed chan<- bool) {
func InitElevator(ch_elevTransmit chan [][]int, ch_elevRecieve chan [][]int, ch_buttonPressed chan<- bool) {
	ch_matrixMaster := make(chan [][]int, 2*constant.N_FLOORS)
	ch_hallOrder := make(chan elevio.ButtonEvent, 2*constant.N_FLOORS) // Hall orders sent over channel
	ch_cabOrder := make(chan elevio.ButtonEvent, constant.N_FLOORS)    // Cab orders sent over channel
	ch_cabServed := make(chan int)
	ch_cabOrderArray := make(chan []int)
	ch_dir := make(chan int)              // Channel for DIR updates
	ch_floor := make(chan int)            // Channel for FLOOR updates
	ch_state := make(chan constant.STATE) // Channel for STATE updates
	ch_elevQuit := make(chan bool)
	// ch_elevRecieveInternal := make(chan [][]int)
	var cabOrders []int
	var localMatrix [][]int
	// order_handler.InitLocalElevatorMatrix()	// Init in fsm.initFSM

	cabOrdersBackup := file_IO.ReadFile(constant.BACKUP_FILENAME) // Matrix
	if len(cabOrdersBackup) == 0 {
		fmt.Println("No backups found.")
		cabOrders = order_handler.InitCabOrders([]int{})
	} else {
		cabOrders = order_handler.InitCabOrders(cabOrdersBackup[0])
		fmt.Println("Backup found: ", cabOrders)
		go backupSendToElevator(ch_cabOrderArray, cabOrders)
	}

	localMatrix = order_handler.InitLocalElevatorMatrix()
	// Button updates over their respective channels
	go order_handler.UpdateOrderMatrix(ch_hallOrder, ch_cabOrder, ch_buttonPressed)
	// Listen for elevator updates, send the update to master/slave module. Send to elev fsm via ch_cabOrderArray
	go order_handler.UpdateCabOrders(ch_cabOrder, ch_cabServed, cabOrders, ch_cabOrderArray)

	// ch_matrixMasterRx <-chan [][]int, ch_cabOrderRx <-chan []int, ch_dirTx chan<- int, ch_floorTx chan<- int, ch_stateTx chan<- constant.STATE

	go fsm.ElevFSM(ch_matrixMaster, ch_cabOrderArray, ch_dir, ch_floor, ch_state, ch_cabServed)

	go tickCounterInternal(ch_buttonPressed)

	// go elevatorHandler(localMatrix, ch_matrixMasterRx, ch_elevTransmit, ch_elevRecieve, ch_dir, ch_floor, ch_state, ch_hallOrder, ch_buttonPressed)

	go elevatorHandler(localMatrix, ch_matrixMaster, ch_elevTransmit, ch_elevRecieve, ch_dir, ch_floor, ch_state, ch_hallOrder, ch_buttonPressed)

	// go elevatorReciever(ch_elevRecieve, ch_elevRecieveInternal)

	<-ch_elevQuit
}

/* Recieve from master/slave and distribute */
// func elevatorReciever(ch_elevRecieve <-chan [][]int, ch_elevRecieveInternal chan<- [][]int) {
// 	var masterMatrix [][]int
// 	for {
// 		masterMatrix = <-ch_elevRecieve
// 		fmt.Println("elevatorReciever: Recieved: ", masterMatrix)
// 		ch_elevRecieveInternal <- masterMatrix
// 		fmt.Println("elevatorReciever: Sent: ", masterMatrix)
// 	}
// }

/* Listens on updates from elevator fsm and updates the elevators local matrix */
func elevatorHandler(localMatrix [][]int, ch_matrixMasterTx chan<- [][]int, ch_elevTx chan<- [][]int, ch_elevRecieve <-chan [][]int, ch_dir <-chan int, ch_floor <-chan int, ch_state <-chan constant.STATE, ch_hallOrder <-chan elevio.ButtonEvent, ch_buttonPressed chan<- bool) {
	fmt.Println("elevatorHandler: Started")
	for {
		fmt.Println("elevatorHandler: Waiting on updates.")
		select {
		case matrixMaster := <-ch_elevRecieve: // Send new matrixMaster to elevator fsm
			for n := 0; n < 10; n++ {
				fmt.Println("elevatorHandler: Recieved: ", matrixMaster)
			}
			localMatrix = confirmOrders(matrixMaster, localMatrix)

			fmt.Println("elevatorHandler: Sending localMatrix to elevator...")
			ch_elevTx <- localMatrix
			fmt.Println("elevatorHandler: Sendt localMatrix to elevator!")

			fmt.Println("elevatorHandler: Sending matrixMaster to elevator...")
			ch_matrixMasterTx <- matrixMaster
			fmt.Println("elevatorHandler: Sendt matrixMaster to elevator!")

			// Send matrix to elevator when there is an update
			// if reflect.DeepEqual(matrixMaster, prevMatrixMaster) == false {
			// 	fmt.Println("elevatorHandler: New matrixMaster recieved. Updating.")
			// 	go transmitMatrixMasterToElevator(ch_matrixMasterTx, matrixMaster)
			// 	prevMatrixMaster = matrixMaster
			// }

		case dir := <-ch_dir: // Changed direction
			fmt.Println("elevatorHandler: Recieved ch_dir, ", dir)
			// ch_buttonPressed <- true
			localMatrix = writeLocalMatrix(ch_elevTx, localMatrix, int(constant.UP_BUTTON), int(constant.DIR), int(dir))
			fmt.Println("elevatorHandler: FINISHED ch_dir, ", dir)
		case floor := <-ch_floor: // Arrived at floor
			fmt.Println("elevatorHandler: Recieved ch_floor, ", floor)
			// ch_buttonPressed <- true
			localMatrix = writeLocalMatrix(ch_elevTx, localMatrix, int(constant.UP_BUTTON), int(constant.FLOOR), int(floor))
			fmt.Println("elevatorHandler: FINISHED ch_floor, ", floor)
		case state := <-ch_state: // Changed state
			fmt.Println("elevatorHandler: Recieved ch_state, ", state)
			// ch_buttonPressed <- true
			localMatrix = writeLocalMatrix(ch_elevTx, localMatrix, int(constant.UP_BUTTON), int(constant.ELEV_STATE), int(state))
			fmt.Println("elevatorHandler: FINISHED ch_state, ", state)
		case hallOrder := <-ch_hallOrder: // Recieved hall-order
			fmt.Println("elevatorHandler: Recieved ch_hallOrder")
			// ch_buttonPressed <- true
			if hallOrder.Button == elevio.BT_HallUp {
				localMatrix = writeLocalMatrix(ch_elevTx, localMatrix, int(constant.UP_BUTTON), int(constant.FIRST_FLOOR)+hallOrder.Floor, 1)
			} else if hallOrder.Button == elevio.BT_HallDown {
				localMatrix = writeLocalMatrix(ch_elevTx, localMatrix, int(constant.DOWN_BUTTON), int(constant.FIRST_FLOOR)+hallOrder.Floor, 1)
			}
			fmt.Println("elevatorHandler: FINISHED ch_hallOrder")
		}
	}
}

func transmitMatrixMasterToElevator(ch_matrixMasterTx chan<- [][]int, matrixMaster [][]int) {
	ch_matrixMasterTx <- matrixMaster
}

func writeLocalMatrix(ch_elevTx chan<- [][]int, localMatrix [][]int, row int, col int, value int) [][]int {
	localMatrix[row][col] = value
	fmt.Println("writeLocalMatrix: Writing ", value, "...")
	ch_elevTx <- localMatrix // Send to master/slave module
	fmt.Println("writeLocalMatrix: Sent ", value, "!")
	return localMatrix
}

/* Delete unconfirmed orders in localMatrix */
func confirmOrders(matrixMaster [][]int, localMatrix [][]int) [][]int {
	for row := constant.UP_BUTTON; row <= constant.DOWN_BUTTON; row++ {
		for col := int(constant.FIRST_FLOOR); col < len(matrixMaster[0]); col++ {
			if matrixMaster[row][col] == 1 {
				fmt.Println("DELETED UNCONFIRMED ORDER AT ELEVATOR ", row, ", FLOOR ", col)
				localMatrix[row][col] = 0
			}
		}
	}
	return localMatrix
}

/* Sends a tick to update the internal masterMatrix with the recieved one */
func tickCounterInternal(ch_buttonPressed chan<- bool) {
	ticker2 := time.NewTicker(500 * time.Millisecond)
	for range ticker2.C {
		ch_buttonPressed <- true
	}
}

func Dummy() {

}
