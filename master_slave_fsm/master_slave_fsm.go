package master_slave_fsm

import (
	"fmt"
	"os" // For getPID
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
	CAB         FIELD = 2
)

const UPDATE_INTERVAL = 250 // Tick time in milliseconds
const BACKUP_FILENAME = "backup.txt"
const PORT_bcast = 16569
const PORT_slaveBcast = 16570
const PORT_peers = 15647

var localIP int
var flagDisconnectedPeer bool = false
var flagMasterSlave STATE

func InitMasterSlave() {
	fmt.Println("Initializing Master/Slave state machine...")
	var matrixMaster [][]int

	// localIP = getLocalIP() // ENABLE AT LAB, DOESNT WORK ELSEWHERE?
	localIP = os.Getpid()
	fmt.Println("This machines localIP-ID is: ", localIP)

	ch_elevTransmit := make(chan [][]int) // Elevator transmission, FROM elevator
	ch_elevRecieve := make(chan [][]int)  // Elevator reciever,	TO elevator

	ch_updateInterval := make(chan int) // Periodic update-ticks
	ch_peerUpdate := make(chan peers.PeerUpdate)
	ch_peerEnable := make(chan bool)
	ch_transmit := make(chan [][]int)      // Master matrix transmission
	ch_recieve := make(chan [][]int)       // Master matrix reciever
	ch_transmitSlave := make(chan [][]int) // Slave matrix transmission
	ch_recieveSlave := make(chan [][]int)  // Slave matrix reciever
	ch_peerDisconnected := make(chan int)
	ch_repeatedBcast := make(chan [][]int)

	// Communicates with the local elevator
	go localOrderHandler(ch_recieve, ch_transmitSlave, ch_elevRecieve, ch_elevTransmit)

	go peers.Transmitter(PORT_peers, string(localIP), ch_peerEnable)
	go peers.Receiver(PORT_peers, ch_peerUpdate)

	// Spawn transmission/reciever goroutines.
	go bcast.Transmitter(PORT_bcast, ch_transmit)
	go bcast.Receiver(PORT_bcast, ch_recieve)
	go bcast.Transmitter(PORT_slaveBcast, ch_transmitSlave)
	go bcast.Receiver(PORT_slaveBcast, ch_recieveSlave)
	go repeatedBroadcast(ch_repeatedBcast, ch_updateInterval, ch_transmit)
	// Start the update_interval ticker.

	go tickCounter(ch_updateInterval)

	// Check for DCed peers
	go checkDisconnectedPeers(ch_peerUpdate, ch_peerDisconnected)

	fmt.Println("Master/Slave state machine initialized.")

	// Start in slave-state
	stateChange(matrixMaster, SLAVE, ch_recieve, ch_recieveSlave, ch_peerDisconnected, ch_repeatedBcast)

}

// init UDP
// Slave state

/* PLACEHOLDER TITLE */
func stateChange(matrixMaster [][]int, currentState STATE, ch_recieve <-chan [][]int, ch_recieveSlave <-chan [][]int, ch_peerDisconnected <-chan int, ch_repeatedBcast chan<- [][]int) {
	for {
		switch currentState {
		case MASTER:
			currentState = stateMaster(matrixMaster, ch_recieve, ch_recieveSlave, ch_peerDisconnected, ch_repeatedBcast)
		case SLAVE:
			currentState = stateSlave(ch_recieve)
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

func stateMaster(matrixMaster [][]int, ch_recieve <-chan [][]int, ch_recieveSlave <-chan [][]int, ch_peerDisconnected <-chan int, ch_repeatedBcast chan<- [][]int) STATE {
	flagMasterSlave = MASTER
	fmt.Println("Masterstate activated.")

	// If matrixMaster is empty, generate masterMatrix for 1 elevator
	if matrixMaster == nil {
		matrixMaster = initMatrixMaster()
	}
	// JUST FOR TESTING. DELETE AT LATER STAGE
	// ch_recieve <- matrixMaster
	for {
		select {
		case newMatrixMaster := <-ch_recieve:
			if checkMaster(newMatrixMaster) == SLAVE {
				return SLAVE // Change to slave
			}
		default:
			// Remain master, continue
		}

		fmt.Println("Waiting on 'ch_recieveSlave'")
		recievedMatrix := <-ch_recieveSlave

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
	// return SLAVE	// Unreachable code.
	// stateChange(matrixMaster, SLAVE, cabOrders)
}

/* Slave state, checks for alive masters. Transitions if no masters on UDP. */
func stateSlave(ch_recieve <-chan [][]int) STATE {
	// var masterMatrix [][]int		// masterMatrix not used, only checks for signal on channel
	ch_slaveAlone := make(chan bool)
	ch_killTimer := make(chan bool)
	flagSlaveAlone := true // Assumes slave to be alone
	fmt.Println("Initializing slave-state")

	for {
		if flagSlaveAlone == true {
			go slaveTimer(ch_slaveAlone, ch_killTimer)
			flagSlaveAlone = false
		}
		select {
		case <-ch_recieve: // Recieves masterMatrix on channel from master over UDP. //masterMatrix = <-ch_recieve:
			flagSlaveAlone = true // Reset timer-flag
			ch_killTimer <- true  // Kill timer
		case <-ch_slaveAlone:
			return MASTER
		default:
			// Do nothing
		}
	}
	// return MASTER
}

func slaveTimer(ch_slaveAlone chan<- bool, ch_killTimer <-chan bool) {
	// Timer of 5 times the UPDATE_INTERVAL
	timer := time.NewTimer(5 * UPDATE_INTERVAL * time.Millisecond)
	for {
		select {
		case <-timer.C:
			ch_slaveAlone <- true
			break
		case <-ch_killTimer:
			break
		}
	}
}

/* Communicates the master matrix to the elevator, and recieves data of the
elevators current state which is broadcast to master over UDP. */
func localOrderHandler(ch_recieve <-chan [][]int, ch_transmitSlave chan<- [][]int, ch_elevRecieve chan<- [][]int, ch_elevTransmit <-chan [][]int) {
	localMatrix := initLocalMatrix()
	for {
		select {
		case masterMatrix := <-ch_recieve:
			ch_elevRecieve <- masterMatrix // masterMatrix TO elevator
		case localMatrix = <-ch_elevTransmit: // localMatrix FROM elevator
			localMatrix[UP_BUTTON][SLAVE_MASTER] = int(flagMasterSlave) // Ensure correct state
			localMatrix[UP_BUTTON][IP] = localIP                        // Ensure correct IP
			ch_transmitSlave <- localMatrix
		default:
			// Do nothing.
		}
	}
}

/* Initialize local matrix, 3x(5+N_FLOORS)
   contains information about local elevator and UP/DOWN hall lights. */
func initLocalMatrix() [][]int {
	localMatrix := make([][]int, 0)
	for i := 0; i <= 1; i++ {
		localMatrix = append(localMatrix, make([]int, 5+N_FLOORS))
	}
	return localMatrix
}

/* Check if there are other masters in the recieved matrix.
   Lowest IP remains master.
   Return MASTER if remain master, SLAVE if transition to slave */
func checkMaster(matrix [][]int) STATE {
	rows := len(matrix)
	for row := int(FIRST_ELEV); row < rows; row++ {
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
	if checkStoppedOrDoorsOpen(recievedMatrix) == true {
		matrixMaster[UP_BUTTON][int(FIRST_FLOOR)+currentFloor-1] = 0
		matrixMaster[DOWN_BUTTON][int(FIRST_FLOOR)+currentFloor-1] = 0
	}
	return matrixMaster
}
func checkStoppedOrDoorsOpen(recievedMatrix [][]int) bool {
	if recievedMatrix[UP_BUTTON][ELEV_STATE] == int(fsm.STOP) {
		return true
	}
	if recievedMatrix[UP_BUTTON][ELEV_STATE] == int(fsm.DOORS_OPEN) {
		return true
	}
	return false
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

	for floor := int(FIRST_FLOOR); floor < rowLength; floor++ {
		flagOrderSet = false
		if matrix[UP_BUTTON][floor] == 1 || matrix[DOWN_BUTTON][floor] == 1 {

			//Sjekker om jeg har en heis i etasjen
			for elev := int(FIRST_ELEV); elev < colLength; elev++ {
				// If in floor, give order if elevator is idle, stopped or has doors open
				if matrix[elev][FLOOR] == floor && (matrix[elev][ELEV_STATE] == int(fsm.IDLE) ||
					matrix[elev][ELEV_STATE] == int(fsm.STOP) || matrix[elev][ELEV_STATE] == int(fsm.DOORS_OPEN)) {
					matrix[elev][floor] = 1 // Stop here
					flagOrderSet = true
					break
				}
			}

			//For både opp og ned bestilling
			if flagOrderSet == false && matrix[UP_BUTTON][floor] == 1 && matrix[DOWN_BUTTON][floor] == 1 {
				index := 1
				for {
					for elev := int(FIRST_ELEV); elev < colLength; elev++ {
						//Sjekker under meg, som har retning opp innenfor grense
						if flagOrderSet == false && (matrix[elev][FLOOR] == (floor - int(FIRST_FLOOR) - index)) && (matrix[elev][DIR] == int(elevio.MD_Up) || matrix[elev][DIR] == int(elevio.MD_Stop)) && (floor-index >= int(FIRST_FLOOR)) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
						//Sjekk over meg, som har retning ned og innenfor grensa
						if flagOrderSet == false && (matrix[elev][FLOOR] == (floor - int(FIRST_FLOOR) + index)) && (matrix[elev][DIR] == int(elevio.MD_Down) || matrix[elev][DIR] == int(elevio.MD_Stop)) && (floor+index <= int(FIRST_FLOOR)+N_FLOORS) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
					}
					//Gått igjennom alle heisene
					index++
					//Hvis ordre gitt eller utenfor bounds UTEN å ha funnet kandidat
					if flagOrderSet == true || ((floor-index) < int(FIRST_FLOOR)) && (floor+index > (int(FIRST_FLOOR)+N_FLOORS)) {
						break
					}
				}
				// --------------------------------------------------------------------
				// For OPP bestilling
			} else if flagOrderSet == false && matrix[UP_BUTTON][floor] == 1 {
				index := 1
				for {
					for elev := int(FIRST_ELEV); elev < colLength; elev++ {
						//Sjekker under meg, som har retning opp innenfor grense
						if flagOrderSet == false && (matrix[elev][FLOOR] == (floor - int(FIRST_FLOOR) - index)) && (matrix[elev][DIR] == int(elevio.MD_Up) || matrix[elev][DIR] == int(elevio.MD_Stop)) && (floor-index >= int(FIRST_FLOOR)) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
						//Sjekk over meg, som har retning ned og innenfor grensa
						if flagOrderSet == false && (matrix[elev][FLOOR] == (floor - int(FIRST_FLOOR) + index)) && (matrix[elev][DIR] == int(elevio.MD_Stop)) && (floor+index <= int(FIRST_FLOOR)+N_FLOORS) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
					}
					//Gått igjennom alle heisene
					index++
					//Hvis ordre gitt eller utenfor bounds UTEN å ha funnet kandidat
					if flagOrderSet == true || ((floor-index) < int(FIRST_FLOOR)) && (floor+index > (int(FIRST_FLOOR)+N_FLOORS)) {
						break
					}
				}
				// --------------------------------------------------------------------
				//For bestilling NED
			} else if flagOrderSet == false && matrix[DOWN_BUTTON][floor] == 1 {
				index := 1
				for {
					for elev := int(FIRST_ELEV); elev < colLength; elev++ {
						//Sjekker under meg, som har retning opp innenfor grense
						if flagOrderSet == false && (matrix[elev][FLOOR] == (floor - int(FIRST_FLOOR) - index)) && (matrix[elev][DIR] == int(elevio.MD_Stop)) && (floor-index) >= int(FIRST_FLOOR) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
						//Sjekk over meg, som har retning ned og innenfor grensa
						if flagOrderSet == false && (matrix[elev][FLOOR] == (floor - int(FIRST_FLOOR) + index)) && (matrix[elev][DIR] == int(elevio.MD_Down) || matrix[elev][DIR] == int(elevio.MD_Stop)) && (floor+index <= int(FIRST_FLOOR)+N_FLOORS) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
					}
					//Gått igjennom alle heisene
					index++
					//Hvis ordre gitt eller utenfor bounds UTEN å ha funnet kandidat
					if flagOrderSet == true || ((floor-index) < int(FIRST_FLOOR)) && (floor+index > (int(FIRST_FLOOR)+N_FLOORS)) {
						break
					}
				}
			}

			// Give to master if no elevator has gotten the order
			if flagOrderSet == false {
				for elev := int(FIRST_ELEV); elev < colLength; elev++ {
					if matrix[elev][SLAVE_MASTER] == int(MASTER) {
						matrix[elev][floor] = 1
					}
				}
			}

		} // End order condition
	} // End inf loop
	fmt.Println("SHIIIET")
	return matrix
} // End floor loop

/*Broadcasts last item over ch_repeatedBcast */
func repeatedBroadcast(ch_repeatedBcast <-chan [][]int, ch_updateInterval <-chan int, ch_transmit chan [][]int) {
	var matrix [][]int
	matrix = <-ch_repeatedBcast
	for {
		select {
		case msg := <-ch_repeatedBcast:
			matrix = msg
		default:
			// Empty
		}
		<-ch_updateInterval
		ch_transmit <- matrix
	}
}
