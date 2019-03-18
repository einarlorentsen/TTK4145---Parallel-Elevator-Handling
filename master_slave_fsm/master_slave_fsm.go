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
	FIRST_ELEV  FIELD = 3

	UP_BUTTON   FIELD = 0
	DOWN_BUTTON FIELD = 1
)

const UPDATE_INTERVAL = 250 // Tick time in milliseconds
const BACKUP_FILENAME = "backup.txt"
const PORT_bcast = 16569
const PORT_peers = 15647

var localIP int = getLocalIP()

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
	// ch_matrix := make(chan [][]int)

	go peers.Transmitter(PORT_peers, string(IP), ch_peerEnable)
	go peers.Receiver(PORT_peers, ch_peerUpdate)
	go bcast.Transmitter(PORT_bcast, ch_transmit)
	go bcast.Receiver(PORT_bcast, ch_recieve)

	// Start the update_interval ticker.
	go tickCounter(ch_updateInterval)

	// if matrixMaster == empty
	// Generate for 1 elevator
	// then listen for slaves, goroutine
	if matrixMaster == nil {
		matrixMaster = initMatrixMaster()
	}

	for {
		recievedMatrix := <-ch_recieve
		if checkMaster(recievedMatrix, localIP) == SLAVE {
			break // Change to slave
		}

		// Remove served order at current floor in recievedMatrix
		matrixMaster = checkOrderServed(matrixMaster, recievedMatrix)

		// Insert unconfirmed orders UP/DOWN into matrixMaster
		matrixMaster = mergeUnconfirmedOrders(matrixMaster, recievedMatrix)

		// Check if elevator is stopped on a floor

		// Slaves: 50ms pr message
		// Buffer at master, check newest messages
		// Use newest messages.
		// If no message from slave within tick period -> It has lost connection

	}

	// Listen to channel ch_recieve
	// Recieves a message
	// Update elevator fields (not stop)
	// OR the up/down order fields
	// Calculate new order distribution.
	// Update masterMatrix
	// Logic to break master-state if master is alone and recieves from larger master

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
