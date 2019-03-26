package elevator

import (
	"fmt"

	"../constant"
	"../file_IO"
	"./elevio"
	"./fsm"
	"./order_handler"
)

var elevIndex int

// var LocalMatrix [][]int

/* */
func InitElevator(ch_elevTransmit chan<- [][]int, ch_elevRecieve <-chan [][]int) {
	ch_matrixMasterRx := make(chan [][]int)
	ch_hallOrder := make(chan elevio.ButtonEvent) // Hall orders sent over channel
	ch_cabOrder := make(chan elevio.ButtonEvent)  // Cab orders sent over channel
	ch_cabServed := make(chan elevio.ButtonEvent)
	ch_cabOrderArray := make(chan []int)
	ch_dir := make(chan int)              // Channel for DIR updates
	ch_floor := make(chan int)            // Channel for FLOOR updates
	ch_state := make(chan constant.STATE) // Channel for STATE updates
	var cabOrders []int
	var localMatrix [][]int
	// order_handler.InitLocalElevatorMatrix()	// Init in fsm.initFSM

	cabOrdersBackup := file_IO.ReadFile(constant.BACKUP_FILENAME) // Matrix
	if len(cabOrdersBackup) == 0 {
		fmt.Println("No backups found.")
		cabOrders = order_handler.InitCabOrders([]int{})
	} else {
		fmt.Println("Backup found.")
		cabOrders = order_handler.InitCabOrders(cabOrdersBackup[0])
	}

	localMatrix = order_handler.InitLocalElevatorMatrix()
	// Button updates over their respective channels
	go order_handler.UpdateOrderMatrix(ch_hallOrder, ch_cabOrder)
	// Listen for elevator updates, send the update to master/slave module.
	go order_handler.UpdateCabOrders(ch_cabOrder, ch_cabServed, cabOrders)

	// ch_matrixMasterRx <-chan [][]int, ch_cabOrderRx <-chan []int, ch_dirTx chan<- int, ch_floorTx chan<- int, ch_stateTx chan<- constant.STATE

	go fsm.ElevFSM(ch_matrixMasterRx, ch_cabOrderArray, ch_dir, ch_floor, ch_state)

	elevatorHandler(localMatrix, ch_matrixMasterRx, ch_elevTransmit, ch_elevRecieve, ch_dir, ch_floor, ch_state, ch_hallOrder)
}

/* Listens on updates from elevator fsm and updates the elevators local matrix */
func elevatorHandler(localMatrix [][]int, ch_matrixMasterTx chan<- [][]int, ch_elevTx chan<- [][]int, ch_elevRx <-chan [][]int, ch_dir <-chan int, ch_floor <-chan int, ch_state <-chan constant.STATE, ch_hallOrder <-chan elevio.ButtonEvent) {
	for {
		fmt.Println("ListenElevator: Waiting on updates.")
		select {
		case matrixMaster := <-ch_elevRx: // Send new matrixMaster to elevator fsm
			localMatrix = confirmOrders(matrixMaster, localMatrix)
			ch_elevTx <- localMatrix
			// Motta master -> Sammenlikne ordre med local (unconfirmed)
			// Fjern de som er delt med master

			ch_matrixMasterTx <- matrixMaster
		case dir := <-ch_dir: // Changed direction
			localMatrix = writeLocalMatrix(ch_elevTx, localMatrix, int(constant.UP_BUTTON), int(constant.DIR), int(dir))
		case floor := <-ch_floor: // Arrived at floor
			// Update to floor
			// Update floor lights
			localMatrix = writeLocalMatrix(ch_elevTx, localMatrix, int(constant.UP_BUTTON), int(constant.FLOOR), int(floor))
		case state := <-ch_state: // Changed state
			localMatrix = writeLocalMatrix(ch_elevTx, localMatrix, int(constant.UP_BUTTON), int(constant.ELEV_STATE), int(state))
		case hallOrder := <-ch_hallOrder: // Recieved hall-order
			if hallOrder.Button == elevio.BT_HallUp {
				localMatrix = writeLocalMatrix(ch_elevTx, localMatrix, int(constant.UP_BUTTON), int(constant.FIRST_FLOOR)+hallOrder.Floor, 1)
			} else if hallOrder.Button == elevio.BT_HallDown {
				localMatrix = writeLocalMatrix(ch_elevTx, localMatrix, int(constant.DOWN_BUTTON), int(constant.FIRST_FLOOR)+hallOrder.Floor, 1)
			}
		}
	}
}

func writeLocalMatrix(ch_elevTx chan<- [][]int, localMatrix [][]int, row int, col int, value int) [][]int {
	localMatrix[row][col] = value
	ch_elevTx <- localMatrix // Send to master/slave module
	return localMatrix
}

func confirmOrders(matrixMaster [][]int, localMatrix [][]int) [][]int {
	for row := constant.UP_BUTTON; row <= constant.DOWN_BUTTON; row++ {
		for col := int(constant.FIRST_FLOOR); col < len(matrixMaster[0]); col++ {
			if matrixMaster[row][col] == 1 {
				localMatrix[row][col] = 0
			}
		}
	}
	return localMatrix
}
