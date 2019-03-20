package master_slave_fsm

import (
	"fmt"
	"time"

	"../elevator/elevio"
	"../elevator/fsm"
	"../file_IO"
	"../network/bcast"
	"../network/localip"
	"../network/peers"
)

const N_FLOORS = 4

/* Enumeration STATE */
type STATE int

const (
	SLAVE  STATE = 0
	MASTER STATE = 1
)

/* Indices to masterMatrix */
/* | IP | DIR | FLOOR | ELEV_STATE | Slave/Master | Stop1 | .. | Stop N | */
type FIELD int

const (
	IP           FIELD = 0
	DIR          FIELD = 1
	FLOOR        FIELD = 2
	ELEV_STATE   FIELD = 3
	SLAVE_MASTER FIELD = 4

	FIRST_FLOOR FIELD = 5
	FIRST_ELEV  FIELD = 2

	UP_BUTTON   FIELD = 0
	DOWN_BUTTON FIELD = 1
)

const UPDATE_INTERVAL = 250 // Tick time in milliseconds
const BACKUP_FILENAME = "backup.txt"
const PORT_bcast = 16569
const PORT_peers = 15647

var localIP int = getLocalIP()
var flagDisconnectedPeer bool = false

func Init() {
	var matrixMaster [][]int
	var initialCabOrders []int // Vector for cab orders

	// CHECK FOR BACKUP FILE, CAB ORDERS
	cabOrders := file_IO.ReadFile(BACKUP_FILENAME) // Matrix
	if len(cabOrders) == 0 {
		fmt.Println("No backups found.")
	} else {
		fmt.Println("Backup found.")
		initialCabOrders = cabOrders[0]
	}

	// Start in slave-state
	stateChange(matrixMaster, SLAVE, initialCabOrders)

}

// init UDP
// Slave state

/* PLACEHOLDER TITLE */
func stateChange(matrixMaster [][]int, currentState STATE, cabOrders []int) {
	switch currentState {
	case MASTER:
		stateMaster(matrixMaster, cabOrders)
	case SLAVE:
		stateSlave(cabOrders)
	}
}

/* matrixMaster dim: (2+N_ELEVATORS) x (5+N_FLOORS) */
/*           | IP | DIR | FLOOR | ELEV_STATE | Slave/Master | Stop1 | .. | Stop N | */
/* UP lights | x  |  x  |       |      x     |       x      |       | .. |    x   | */
/* DN lights | x  |  x  |       |      x     |       x      |   x   | .. |        | */
/* ELEV 1    |    |     |       |            |              |       | .. |        | */
/* ...       |    |     |       |            |              |       | .. |        | */
/* ELEV N    |    |     |       |            |              |       | .. |        | */
/* Matrix indexing: [ROW][COL] */

func stateMaster(matrixMaster [][]int, cabOrders []int) {
	ch_updateInterval := make(chan int)
	ch_peerUpdate := make(chan peers.PeerUpdate)
	ch_peerEnable := make(chan bool)
	ch_transmit := make(chan [][]int)
	ch_recieve := make(chan [][]int)
	ch_peerDisconnected := make(chan int)
	// ch_matrix := make(chan [][]int)

	go peers.Transmitter(PORT_peers, string(localIP), ch_peerEnable)
	go peers.Receiver(PORT_peers, ch_peerUpdate)
	go bcast.Transmitter(PORT_bcast, ch_transmit)
	go bcast.Receiver(PORT_bcast, ch_recieve)

	// Start the update_interval ticker.
	go tickCounter(ch_updateInterval)

	// Check for DCed peers
	go checkDisconnectedPeers(ch_peerUpdate, ch_peerDisconnected)

	// If matrixMaster is empty, generate masterMatrix for 1 elevator
	if matrixMaster == nil {
		matrixMaster = initMatrixMaster()
	}

	for {
		recievedMatrix := <-ch_recieve
		if checkMaster(recievedMatrix, localIP) == SLAVE {
			break // Change to slave
		}

		// Check for disconnected slaves and delete them
		if flagDisconnectedPeer == true { // Peerus deletus
			disconnectedIP := <-ch_peerDisconnected
			matrixMaster = deleteDisconnectedPeer(matrixMaster, disconnectedIP)
			flagDisconnectedPeer = false
		}

		// Merge info from recievedMatrix, append if new slave
		matrixMaster = mergeRecievedInfo(matrixMaster, recievedMatrix)

		// Remove served order at current floor in recievedMatrix
		matrixMaster = checkOrderServed(matrixMaster, recievedMatrix)

		// Insert unconfirmed orders UP/DOWN into matrixMaster
		matrixMaster = mergeUnconfirmedOrders(matrixMaster, recievedMatrix)

		// Calculate stop
		matrixMaster = calculateElevatorStops(matrixMaster)

		// Broadcast this this
	}

	stateChange(matrixMaster, SLAVE, cabOrders)
}

func stateSlave(cabOrders []int) {
	// var matrixSlave [][]int
}

/* Check if there are other masters in the recieved matrix.
   Lowest IP remains master.
   Return true if remain master, false if transition to slave */
func checkMaster(matrix [][]int, localIP int) STATE {
	rows := len(matrix)
	for row := 0; row < rows; row++ {
		if matrix[row][SLAVE_MASTER] == int(MASTER) {
			if matrix[row][IP] < localIP {
				return SLAVE // Transition to slave
			}
		}
	}
	return MASTER // Remain master
}

func initMatrixMaster() [][]int {
	matrixMaster := make([][]int, 0)
	for i := 0; i <= 2; i++ { // For 1 elevator
		matrixMaster = append(matrixMaster, make([]int, 4+N_FLOORS))
	}
	ch_floorSensor := make(chan int)
	elevio.GetFloorInit(ch_floorSensor)

	matrixMaster[FIRST_ELEV][IP] = localIP
	matrixMaster[FIRST_ELEV][DIR] = elevio.MD_Stop
	matrixMaster[FIRST_ELEV][FLOOR] = <-ch_floorSensor
	matrixMaster[FIRST_ELEV][ELEV_STATE] = int(fsm.READY)
	matrixMaster[FIRST_ELEV][SLAVE_MASTER] = int(MASTER)

	return matrixMaster
}

/*  Converts the IP to an int. Example:
    "10.100.23.253" -> 253 */
func getLocalIP() int {
	returnedIP, err := localip.LocalIP()
	if err != nil {
		fmt.Println(err)
		returnedIP = "DISCONNECTED"
	}

	IP_length := len(returnedIP)
	for i := IP_length - 1; i > 0; i-- {
		if returnedIP[i] == '.' {
			returnedIP = returnedIP[i+1 : IP_length]
			break
		}
	}
	return file_IO.StringToNumbers(returnedIP)[0] // Vector of 1 element
}

/* Ticks every UPDATE_INTERVAL milliseconds */
func tickCounter(ch_updateInterval chan<- int) {
	ticker := time.NewTicker(UPDATE_INTERVAL * time.Millisecond)
	for range ticker.C {
		ch_updateInterval <- 1
	}
}

/* *********************************************** */
/*               HELPER FUNCTIONS                  */

/* Check for disconnected peers, pass IP as int over channel */
func checkDisconnectedPeers(ch_peerUpdate <-chan peers.PeerUpdate, ch_peerDisconnected chan<- int) {
	for {
		if flagDisconnectedPeer == false {
			peerUpdate := <-ch_peerUpdate
			if peerUpdate.Lost != nil {
				flagDisconnectedPeer = true
				peerIP := file_IO.StringToNumbers(peerUpdate.Lost[0])[0]
				ch_peerDisconnected <- peerIP
			}
		}
	}
}

/* Delete peer with the corresponding IP */
func deleteDisconnectedPeer(matrixMaster [][]int, disconnectedIP int) [][]int {
	for row := int(FIRST_ELEV); row < len(matrixMaster); row++ {
		if matrixMaster[row][IP] == disconnectedIP {
			matrixMaster = append(matrixMaster[:row], matrixMaster[row+1:]...) // Delete row
		}
	}
	return matrixMaster
}

/* Merge info from recievedMatrix, append if new slave */
func mergeRecievedInfo(matrixMaster [][]int, recievedMatrix [][]int) [][]int {
	slaveIP := recievedMatrix[UP_BUTTON][IP]
	flagSlaveExist := false
	for row := int(FIRST_ELEV); row < len(matrixMaster); row++ {
		if matrixMaster[row][IP] == slaveIP {
			matrixMaster[row][DIR] = recievedMatrix[UP_BUTTON][DIR]
			matrixMaster[row][FLOOR] = recievedMatrix[UP_BUTTON][FLOOR]
			matrixMaster[row][ELEV_STATE] = recievedMatrix[UP_BUTTON][ELEV_STATE]
			flagSlaveExist = true
		}
	}
	if flagSlaveExist == false {
		newSlave := make([]int, FIRST_FLOOR+N_FLOORS)
		copy(newSlave[0:SLAVE_MASTER+1], recievedMatrix[UP_BUTTON][0:SLAVE_MASTER+1]) // Copy not inclusive for last index
		matrixMaster = append(matrixMaster, newSlave)
	}
	return matrixMaster
}

/* Removes served orders in the current floor of recievedMatrix */
func checkOrderServed(matrixMaster [][]int, recievedMatrix [][]int) [][]int {
	currentFloor := recievedMatrix[UP_BUTTON][FLOOR]
	if recievedMatrix[UP_BUTTON][ELEV_STATE] == int(fsm.STOP) {
		matrixMaster[UP_BUTTON][int(FIRST_FLOOR)+currentFloor-1] = 0
		matrixMaster[DOWN_BUTTON][int(FIRST_FLOOR)+currentFloor-1] = 0
	}
	return matrixMaster
}

/* Insert unconfirmed orders UP/DOWN into matrixMaster */
func mergeUnconfirmedOrders(matrixMaster [][]int, recievedMatrix [][]int) [][]int {
	for row := UP_BUTTON; row <= DOWN_BUTTON; row++ {
		for col := FIRST_FLOOR; col < (N_FLOORS + FIRST_FLOOR); col++ {
			if recievedMatrix[row][col] == 1 {
				matrixMaster[row][col] = 1
			}
		}
	}
	return matrixMaster
}

/* matrixMaster dim: (2+N_ELEVATORS) x (5+N_FLOORS) */
/*           | IP | DIR | FLOOR | ELEV_STATE | Slave/Master | Stop1 | .. | Stop N | */
/* UP lights | x  |  x  |       |      x     |       x      |       | .. |    x   | */
/* DN lights | x  |  x  |       |      x     |       x      |   x   | .. |        | */
/* ELEV 1    |    |     |       |            |              |       | .. |        | */
/* ...       |    |     |       |            |              |       | .. |        | */
/* ELEV N    |    |     |       |            |              |       | .. |        | */
/* Matrix indexing: [ROW][COL] */


/* Order distribution algorithm */
func calculateElevatorStops(matrix [][]int) [][]int {
	var flagOrderSet bool
	rowLength := len(matrix[UP_BUTTON])
	colLength := len(matrix)

	for floor := int(FIRST_FLOOR); floor < rowLength; floor++ {
		flagOrderSet = false
		// Assumes elevator stops if any order at Floor
		for elev := int(FIRST_ELEV); elev < colLength; elev++ {
			// If in floor, give order if elevator is idle, stopped or has doors open
			if (matrix[elev][FLOOR] == floor && (matrix[elev][STATE] == fsm.IDLE ||
				matrix[elev][STATE] == fsm.STOP || matrix[elev][STATE] == fsm.OPEN_DOORS )) {
					matrix[elev][floor] = 1	// Stop here
					flagOrderSet = true
					break
			}
		}

		if flagOrderSet == false && matrix[elev][UP_BUTTON] == 1 && matrix[elev][DOWN_BUTTON]{
			for index := 1 ; index < N_FLOORS ; index ++{
				for elev := int(FIRST_ELEV); elev < colLength; elev++ {
					// Both direction buttons set
					aboveFloor := floor + index
					belowFloor := floor - index

					// UP button set

					// Down button set

				}
			}
		}







		// if (matrixMaster[UP_BUTTON][floor] || matrixMaster[UP_BUTTON][floor]) {
		// 	for elev := int(FIRST_ELEV); elev < colLength; elev++ {
		// 		// If in floor, give order if elevator is idle, stopped or has doors open
		// 		if (matrixMaster[elev][FLOOR] == floor && (matrixMaster[elev][STATE] == fsm.IDLE ||
		// 			matrixMaster[elev][STATE] == fsm.STOP || matrixMaster[elev][STATE] == fsm.OPEN_DOORS )) {
		// 				matrixMaster[elev][floor] = 1	// Stop here
		// 		}
		// 	}
		// 	for index := 1 ; index < N_FLOORS ; index++{
		// 		for elev := int(FIRST_ELEV); elev < colLength; elev++ {
		// 			aboveFloor := floor + index
		// 			belowFloor := floor - index
		// 			if (matrixMaster[elev][FLOOR] == aboveFloor && matrixMaster[elev][FLOOR] == elevio.MD_DOWN){
		// 				//Give order
		// 			}
		// 			else (matrixMaster[elev][FLOOR] == belowFloor && matrixMaster[elev][FLOOR] == elevio.MD_UP)
		// 		}
		// 	}


				// else iterate: floor+1
				// check for elevators down to floor or idle
				// else iterate: floor-1
				// check for elevators up to floor or idle
		}
	}
}
