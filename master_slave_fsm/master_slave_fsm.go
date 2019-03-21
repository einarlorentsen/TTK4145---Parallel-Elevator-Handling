package master_slave_fsm

import (
	"fmt"
	"time"
	"os"	// For getPID

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
const PORT_slaveBcast = 16570
const PORT_peers = 15647

var localIP int
var flagDisconnectedPeer bool = false

func InitMasterSlave() {
	fmt.Println("Initializing Master/Slave state machine...")
	var matrixMaster [][]int
	var initialCabOrders []int // Vector for cab orders

	// localIP = getLocalIP() // ENABLE AT LAB, DOESNT WORK ELSEWHERE?
	localIP = os.Getpid()
	fmt.Println("This machines localIP-ID is: ", localIP)

	// CHECK FOR BACKUP FILE, CAB ORDERS
	cabOrders := file_IO.ReadFile(BACKUP_FILENAME) // Matrix
	if len(cabOrders) == 0 {
		fmt.Println("No backups found.")
	} else {
		fmt.Println("Backup found.")
		initialCabOrders = cabOrders[0]
	}

	fmt.Println("Master/Slave state machine initialized.")

	// Start in slave-state
	stateChange(matrixMaster, SLAVE, initialCabOrders)

}

// init UDP
// Slave state

/* PLACEHOLDER TITLE */
func stateChange(matrixMaster [][]int, currentState STATE, cabOrders []int) {
	for {
		switch currentState {
		case MASTER:
			currentState, cabOrders = stateMaster(matrixMaster, cabOrders)
		case SLAVE:
			currentState, cabOrders = stateSlave(cabOrders)
		}
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

func stateMaster(matrixMaster [][]int, cabOrders []int) (STATE, []int) {
	fmt.Println("Masterstate activated.")
	ch_updateInterval := make(chan int)
	ch_peerUpdate := make(chan peers.PeerUpdate)
	ch_peerEnable := make(chan bool)
	ch_transmit := make(chan [][]int)
	ch_recieve := make(chan [][]int)
	ch_slaveTransmit := make(chan [][]int)
	ch_slaveRecieve := make(chan [][]int)
	ch_peerDisconnected := make(chan int)
	ch_repeatedBcast := make(chan [][]int)
	// ch_matrix := make(chan [][]int)

	go peers.Transmitter(PORT_peers, string(localIP), ch_peerEnable)
	go peers.Receiver(PORT_peers, ch_peerUpdate)
	go bcast.Transmitter(PORT_bcast, ch_transmit)
	go bcast.Receiver(PORT_bcast, ch_recieve)

	// Spawn transmitter/reciever for slave-information
	go bcast.Transmitter(PORT_slaveBcast, ch_slaveTransmit)
	go bcast.Receiver(PORT_slaveBcast, ch_slaveRecieve)

	go repeatedBroadcast(ch_repeatedBcast, ch_updateInterval, ch_transmit)

	// Start the update_interval ticker.
	go tickCounter(ch_updateInterval)

	// Check for DCed peers
	go checkDisconnectedPeers(ch_peerUpdate, ch_peerDisconnected)

	// If matrixMaster is empty, generate masterMatrix for 1 elevator
	if matrixMaster == nil {
		matrixMaster = initMatrixMaster()
	}


	// JUST FOR TESTING. DELETE AT LATER STAGE
	// ch_recieve <- matrixMaster

	for {
		fmt.Println("Waiting on 'ch_recieve'")
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

		// Clear orders
		matrixMaster = clearCurrentOrders(matrixMaster)

		// Calculate stop
		matrixMaster = calculateElevatorStops(matrixMaster)

		// Broadcast the whole
		ch_repeatedBcast <- matrixMaster
	}

	return SLAVE, cabOrders
	// stateChange(matrixMaster, SLAVE, cabOrders)
}


func stateSlave(cabOrders []int) (STATE, []int){
	fmt.Println("Initializing slave-state")
	// var matrixSlave [][]int
	return MASTER, cabOrders

}

/* Check if there are other masters in the recieved matrix.
   Lowest IP remains master.
   Return MASTER if remain master, SLAVE if transition to slave */
func checkMaster(matrix [][]int, localIP int) (STATE) {
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
	for i := 0; i <= 2; i++ { // For 1 elevator, master is assumed alone
		matrixMaster = append(matrixMaster, make([]int, 5+N_FLOORS))
	}
	ch_floorSensor := make(chan int)

	fmt.Println(matrixMaster)

	// TODO REMOVE WHEN AT LAB AND HAS HARDWARE / SIMULATOR
	elevio.GetFloorInit(ch_floorSensor)
	// ch_floorSensor <- 2 // Dummy for when elevator is not present
	fmt.Println("UWOTM8")
	matrixMaster[FIRST_ELEV][IP] = localIP
	fmt.Println("Bug here?")
	matrixMaster[FIRST_ELEV][DIR] = elevio.MD_Stop
	fmt.Println("Bug here2?")
	matrixMaster[FIRST_ELEV][FLOOR] = <-ch_floorSensor
	fmt.Println("Bug here3?")
	matrixMaster[FIRST_ELEV][ELEV_STATE] = int(fsm.IDLE)
	fmt.Println("Bug here4?")
	matrixMaster[FIRST_ELEV][SLAVE_MASTER] = int(MASTER)
	fmt.Println("Bug here5?")
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
	if recievedMatrix[UP_BUTTON][ELEV_STATE] == (int(fsm.STOP) || int(fsm.DOORS_OPEN)) {
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

/* Clear the elevators' current orders */
func clearCurrentOrders(matrix [][]int) [][]int {
	for floor := int(FIRST_FLOOR); floor < len(matrix[UP_BUTTON]); floor++ {
		for elev := int(FIRST_ELEV); elev < len(matrix); elev++ {
			matrix[elev][floor] = 0
		}
	}
	return matrix
}

/* Order distribution algorithm */
func calculateElevatorStops(matrix [][]int) [][]int {
	fmt.Println("Calculate stops")
	var flagOrderSet bool
	rowLength := len(matrix[UP_BUTTON])
	colLength := len(matrix)

	for floor := int(FIRST_FLOOR); floor< rowLength; floor++{
		flagOrderSet = false
		if( matrix[UP_BUTTON][floor] == 1 || matrix[DOWN_BUTTON][floor] == 1){

			//Sjekker om jeg har en heis i etasjen
			for elev := int(FIRST_ELEV); elev < colLength; elev++ {
				// If in floor, give order if elevator is idle, stopped or has doors open
				if (matrix[elev][FLOOR] == floor && (matrix[elev][ELEV_STATE] == int(fsm.IDLE) ||
					matrix[elev][ELEV_STATE] == int(fsm.STOP) || matrix[elev][ELEV_STATE] == int(fsm.DOORS_OPEN) )) {
					matrix[elev][floor] = 1	// Stop here
					flagOrderSet = true
					break
				}
			}

			//For både opp og ned bestilling
			if(flagOrderSet == false && matrix[UP_BUTTON][floor] == 1 && matrix[DOWN_BUTTON][floor] == 1){
				index := 1
				for{
					for elev := int(FIRST_ELEV); elev < colLength; elev++{
						//Sjekker under meg, som har retning opp innenfor grense
						if(flagOrderSet == false && (matrix[elev][FLOOR] == (floor-int(FIRST_FLOOR)-index)) && (matrix[elev][DIR] == int(elevio.MD_Up) || matrix[elev][DIR] == int(elevio.MD_Stop)) && (floor-index >= int(FIRST_FLOOR))){
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
						//Sjekk over meg, som har retning ned og innenfor grensa
						if(flagOrderSet == false && (matrix[elev][FLOOR] == (floor-int(FIRST_FLOOR)+index)) && (matrix[elev][DIR] == int(elevio.MD_Down) || matrix[elev][DIR] == int(elevio.MD_Stop)) && (floor+index <= int(FIRST_FLOOR)+N_FLOORS)){
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
					}
					//Gått igjennom alle heisene
					index++
					//Hvis ordre gitt eller utenfor bounds UTEN å ha funnet kandidat
					if(flagOrderSet == true || ((floor-index) < int(FIRST_FLOOR)) && (floor+index > (int(FIRST_FLOOR)+N_FLOORS))){
						break
					}
				}
				// --------------------------------------------------------------------
				// For OPP bestilling
			} else if(flagOrderSet == false && matrix[UP_BUTTON][floor] == 1) {
				index := 1
				for{
					for elev := int(FIRST_ELEV); elev < colLength; elev++{
						//Sjekker under meg, som har retning opp innenfor grense
						if(flagOrderSet == false && (matrix[elev][FLOOR] == (floor-int(FIRST_FLOOR)-index)) && (matrix[elev][DIR] == int(elevio.MD_Up) || matrix[elev][DIR] == int(elevio.MD_Stop)) && (floor-index >= int(FIRST_FLOOR))){
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
						//Sjekk over meg, som har retning ned og innenfor grensa
						if(flagOrderSet == false && (matrix[elev][FLOOR] == (floor-int(FIRST_FLOOR)+index)) && (matrix[elev][DIR] == int(elevio.MD_Stop)) && (floor+index <= int(FIRST_FLOOR)+N_FLOORS)){
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
					}
					//Gått igjennom alle heisene
					index++
					//Hvis ordre gitt eller utenfor bounds UTEN å ha funnet kandidat
					if(flagOrderSet == true || ((floor-index) < int(FIRST_FLOOR)) && (floor+index > (int(FIRST_FLOOR)+N_FLOORS))){
						break
					}
				}
				// --------------------------------------------------------------------
				//For bestilling NED
			} else if(flagOrderSet == false && matrix[DOWN_BUTTON][floor] == 1) {
				index := 1
				for{
					for elev := int(FIRST_ELEV); elev < colLength; elev++{
						//Sjekker under meg, som har retning opp innenfor grense
						if ((flagOrderSet == false && (matrix[elev][FLOOR] == (floor-int(FIRST_FLOOR)-index)) && (matrix[elev][DIR] == int(elevio.MD_Stop)) && (floor-index) >= int(FIRST_FLOOR))){
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
						//Sjekk over meg, som har retning ned og innenfor grensa
						if(flagOrderSet == false && (matrix[elev][FLOOR] == (floor-int(FIRST_FLOOR)+index)) && (matrix[elev][DIR] == int(elevio.MD_Down) || matrix[elev][DIR] == int(elevio.MD_Stop)) && (floor+index <= int(FIRST_FLOOR)+N_FLOORS)){
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
					}
					//Gått igjennom alle heisene
					index++
					//Hvis ordre gitt eller utenfor bounds UTEN å ha funnet kandidat
					if(flagOrderSet == true || ((floor-index) < int(FIRST_FLOOR)) && (floor+index > (int(FIRST_FLOOR)+N_FLOORS))){
						break
					}
				}
			}

			// Give to master if no elevator has gotten the order
			if (flagOrderSet == false){
				for elev := int(FIRST_ELEV); elev < colLength; elev++ {
					if (matrix[elev][SLAVE_MASTER] == int(MASTER)){
						matrix[elev][floor] = 1
					}
				}
			}

		} // End order condition
	}	// End inf loop
	fmt.Println("SHIIIET")
	return matrix
} // End floor loop




/*Broadcasts last item over ch_repeatedBcast */
func repeatedBroadcast(ch_repeatedBcast <-chan [][]int, ch_updateInterval <-chan int, ch_transmit chan [][]int){
	var matrix [][]int
	matrix = <- ch_repeatedBcast
	for {
		select {
		case msg := <- ch_repeatedBcast:
			matrix = msg
		default:
			<- ch_updateInterval
		}
		ch_transmit <- matrix
	}
}
