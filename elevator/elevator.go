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

/* Index of the elevator in masterMatrix */
var elevIndex int

func backupSendToElevator(ch_cabOrderArray chan<- []int, cabOrders []int) {
	ch_cabOrderArray <- cabOrders
}

func TakeElevatorToNearestFloor() {
	fsm.InitFSM()
}

/* InitElevator: Spawns channels and goroutines for order handling and elevator state-machine. */
func InitElevator(ch_elevTransmit chan [][]int, ch_elevRecieve chan [][]int, ch_updateOrders chan<- bool) {
	ch_masterMatrix := make(chan [][]int, 2*constant.N_FLOORS)
	ch_hallOrder := make(chan elevio.ButtonEvent, 2*constant.N_FLOORS) // Hall orders sent over channel
	ch_cabOrder := make(chan elevio.ButtonEvent, constant.N_FLOORS)    // Cab orders sent over channel
	ch_cabServed := make(chan int, constant.N_FLOORS)
	ch_cabOrderArray := make(chan []int, constant.N_FLOORS)
	ch_dir := make(chan int, constant.N_FLOORS)              // Channel for DIR updates
	ch_floor := make(chan int, constant.N_FLOORS)            // Channel for FLOOR updates
	ch_state := make(chan constant.STATE, constant.N_FLOORS) // Channel for STATE updates
	ch_elevQuit := make(chan bool, constant.N_FLOORS)

	var cabOrders []int
	var localMatrix [][]int
	localMatrix = order_handler.InitLocalElevatorMatrix()

	cabOrdersBackup := file_IO.ReadFile(constant.BACKUP_FILENAME)
	if len(cabOrdersBackup) == 0 {
		fmt.Println("No backups found.")
		cabOrders = order_handler.InitCabOrders([]int{})
	} else {
		cabOrders = order_handler.InitCabOrders(cabOrdersBackup[0])
		fmt.Println("Backup found: ", cabOrders)
		go backupSendToElevator(ch_cabOrderArray, cabOrders)
	}

	go order_handler.UpdateOrderMatrix(ch_hallOrder, ch_cabOrder, ch_updateOrders)
	go order_handler.UpdateCabOrders(ch_cabOrder, ch_cabServed, cabOrders, ch_cabOrderArray)

	go fsm.ElevFSM(ch_masterMatrix, ch_cabOrderArray, ch_dir, ch_floor, ch_state, ch_cabServed)

	go tickCounterInternal(ch_updateOrders)
	go elevatorHandler(localMatrix, ch_masterMatrix, ch_elevTransmit, ch_elevRecieve, ch_dir, ch_floor, ch_state, ch_hallOrder, ch_updateOrders)

	<-ch_elevQuit
}

/* Listens on updates from elevator fsm and updates the elevators localMatrix */
func elevatorHandler(localMatrix [][]int, ch_masterMatrixTx chan<- [][]int, ch_elevTx chan<- [][]int, ch_elevRecieve <-chan [][]int, ch_dir <-chan int, ch_floor <-chan int, ch_state <-chan constant.STATE, ch_hallOrder <-chan elevio.ButtonEvent, ch_updateOrders chan<- bool) {
	for {
		select {
		case masterMatrix := <-ch_elevRecieve: // Send new masterMatrix to elevator fsm
			localMatrix = confirmOrders(masterMatrix, localMatrix)
			ch_elevTx <- localMatrix
			ch_masterMatrixTx <- masterMatrix

		case dir := <-ch_dir: // Changed direction
			localMatrix = writeLocalMatrix(ch_elevTx, localMatrix, int(constant.UP_BUTTON), int(constant.DIR), int(dir))

		case floor := <-ch_floor: // Arrived at floor
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

func transmitMasterMatrixToElevator(ch_masterMatrixTx chan<- [][]int, masterMatrix [][]int) {
	ch_masterMatrixTx <- masterMatrix
}

/* Update localMatrix and send it to master_slave_fsm module */
func writeLocalMatrix(ch_elevTx chan<- [][]int, localMatrix [][]int, row int, col int, value int) [][]int {
	localMatrix[row][col] = value
	ch_elevTx <- localMatrix
	return localMatrix
}

/* Delete unconfirmed hall orders in localMatrix */
func confirmOrders(masterMatrix [][]int, localMatrix [][]int) [][]int {
	for row := constant.UP_BUTTON; row <= constant.DOWN_BUTTON; row++ {
		for col := int(constant.FIRST_FLOOR); col < len(masterMatrix[0]); col++ {
			if masterMatrix[row][col] == 1 {
				localMatrix[row][col] = 0
			}
		}
	}
	return localMatrix
}

/* Sends a tick to update the internal masterMatrix with the recieved one */
func tickCounterInternal(ch_updateOrders chan<- bool) {
	ticker2 := time.NewTicker(constant.UPDATE_INTERNAL * time.Millisecond)
	for range ticker2.C {
		ch_updateOrders <- true
	}
}
