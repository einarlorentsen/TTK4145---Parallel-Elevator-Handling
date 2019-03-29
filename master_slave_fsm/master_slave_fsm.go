package master_slave_fsm

import (
	"fmt"
	"os"
	"time"

	"../constant"

	"../elevator/elevio"
	"../network/bcast"
	"../network/peers"
)

var LocalIP int
var flagDisconnectedPeer bool = false
var flagMasterSlave constant.STATE

func SetLocalIP() {
	// LocalIP = getLocalIP() // ENABLE AT LAB, DOESNT WORK ELSEWHERE?
	LocalIP = os.Getpid()
}

func InitMasterSlave(ch_elevTransmit <-chan [][]int, ch_elevRecieve chan<- [][]int, ch_buttonPressed <-chan bool) {
	fmt.Println("Initializing Master/Slave state machine...")
	var matrixMaster [][]int

	// fullLocalIP, _ := LocalIP.LocalIP() // CURRENTLY PASSED TO PEERS TRANSMITTER. UNSURE
	fmt.Println("This machines LocalIP-ID is: ", LocalIP)

	ch_updateInterval := make(chan int) // Periodic update-ticks
	ch_peerUpdate := make(chan peers.PeerUpdate)
	ch_peerEnable := make(chan bool)
	ch_transmit := make(chan [][]int)          // Master matrix transmission
	ch_recieve := make(chan [][]int)           // Master matrix reciever
	ch_transmitSlave := make(chan [][]int)     // Slave matrix transmission
	ch_recieveSlave := make(chan [][]int)      // Slave matrix reciever
	ch_recieveLocal := make(chan [][]int)      // Local master matrix transfer
	ch_recieveSlaveLocal := make(chan [][]int) // Local slave matrix transfer
	ch_peerDisconnected := make(chan string)
	ch_repeatedBcast := make(chan [][]int)

	// Communicates with the local elevator
	go localOrderHandler(ch_recieveLocal, ch_transmitSlave, ch_elevRecieve, ch_elevTransmit, ch_recieveSlaveLocal)

	go peers.Transmitter(constant.PORT_peers, string(LocalIP), ch_peerEnable)
	// go peers.Transmitter(PORT_peers, fullLocalIP, ch_peerEnable)
	go peers.Receiver(constant.PORT_peers, ch_peerUpdate)

	// Spawn transmission/reciever goroutines.
	go bcast.Transmitter(constant.PORT_bcast, ch_transmit)
	go bcast.Receiver(constant.PORT_bcast, ch_recieve)
	go bcast.Transmitter(constant.PORT_slaveBcast, ch_transmitSlave)
	go bcast.Receiver(constant.PORT_slaveBcast, ch_recieveSlave)
	go repeatedBroadcast(ch_repeatedBcast, ch_updateInterval, ch_transmit, ch_transmitSlave)
	// Start the update_interval ticker.
	go tickCounter(ch_updateInterval)

	// Check for DCed peers
	go checkDisconnectedPeers(ch_peerUpdate, ch_peerDisconnected)

	fmt.Println("Master/Slave state machine initialized.")
	stateChange(matrixMaster, constant.SLAVE, ch_recieve, ch_recieveSlave, ch_peerDisconnected, ch_repeatedBcast, ch_recieveLocal, ch_recieveSlaveLocal, ch_buttonPressed)
}

/* Continously swapping states */
func stateChange(matrixMaster [][]int, currentState constant.STATE, ch_recieve <-chan [][]int, ch_recieveSlave <-chan [][]int, ch_peerDisconnected <-chan string, ch_repeatedBcast chan<- [][]int, ch_recieveLocal chan<- [][]int, ch_recieveSlaveLocal <-chan [][]int, ch_buttonPressed <-chan bool) {
	for {
		switch currentState {
		case constant.MASTER:
			currentState = stateMaster(matrixMaster, ch_recieve, ch_recieveSlave, ch_peerDisconnected, ch_repeatedBcast, ch_recieveLocal, ch_buttonPressed)
		case constant.SLAVE:
			currentState = stateSlave(ch_recieve, ch_repeatedBcast, ch_recieveLocal, ch_recieveSlaveLocal, ch_buttonPressed)
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

func stateMaster(matrixMaster [][]int, ch_recieve <-chan [][]int, ch_recieveSlave <-chan [][]int, ch_peerDisconnected <-chan string, ch_repeatedBcast chan<- [][]int, ch_recieveLocal chan<- [][]int, ch_buttonPressed <-chan bool) constant.STATE {
	flagMasterSlave = constant.MASTER
	flagDisconnectedMatrixHandling := false
	fmt.Println("Masterstate activated.")

	// If matrixMaster is empty, generate masterMatrix for 1 elevator
	if matrixMaster == nil {
		matrixMaster = InitMatrixMaster()
	}
	ch_repeatedBcast <- matrixMaster // Start the correct masterMatrix UDP broadcast

	for {
		select {
		case newMatrixMaster := <-ch_recieve:
			// fmt.Println("stateMaster: Recieved masterMatrix")
			if checkMaster(newMatrixMaster) == constant.SLAVE {
				fmt.Println("stateMaster: checkMaster returned SLAVE")
				return constant.SLAVE // Change to slave
			}
		case disconnectedIP := <-ch_peerDisconnected:
			fmt.Println("Recieved over ch_peerDisconnect")
			matrixMaster = deleteDisconnectedPeer(matrixMaster, disconnectedIP)
			flagDisconnectedPeer = false
			fmt.Println("flagDisconnectedPeer = false")
			fmt.Println("case disconnectedIP: FINISHED")

			sendMatrixMasterToElevator(ch_buttonPressed, ch_recieveLocal, matrixMaster)
			fmt.Println("DC matrixMaster: ", matrixMaster)
			ch_repeatedBcast <- matrixMaster

			flagDisconnectedMatrixHandling = true

		case recievedMatrix := <-ch_recieveSlave:

			// Matrix not properly updated elsewhere. Fix that and then confirm if
			// this check really is neccessary.
			switch flagDisconnectedMatrixHandling {
			case true:
				if matrixDimensionsEqual(matrixMaster, recievedMatrix) == true {
					flagDisconnectedMatrixHandling = false
				}
			case false:
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
				sendMatrixMasterToElevator(ch_buttonPressed, ch_recieveLocal, matrixMaster)
				// fmt.Println("MASTER: Sent on ch_recieveLocal")

				fmt.Println("Update matrixMaster: ", matrixMaster)

				ch_repeatedBcast <- matrixMaster
				// fmt.Println("MASTER: Sent on ch_repeatBcast")
			default:
			}
		}
	}
}

/* */
func matrixDimensionsEqual(matr1 [][]int, matr2 [][]int) bool {
	col1 := len(matr1)
	col2 := len(matr2)
	row1 := len(matr1[0])
	row2 := len(matr2[0])
	if (col1 == col2) && (row1 == row2) {
		return true
	}
	return false
}

func sendMatrixMasterToElevator(ch_buttonPressed <-chan bool, ch_recieveLocal chan<- [][]int, matrixMaster [][]int) {
	<-ch_buttonPressed
	ch_recieveLocal <- matrixMaster // Send to local elevator (localOrderHandler)
}

/* Slave state, checks for alive masters. Transitions if no masters on UDP. */
func stateSlave(ch_recieve <-chan [][]int, ch_repeatedBcast chan<- [][]int, ch_recieveLocal chan<- [][]int, ch_recieveSlaveLocal <-chan [][]int, ch_buttonPressed <-chan bool) constant.STATE {
	// var masterMatrix [][]int		// masterMatrix not used, only checks for signal on channel
	flagMasterSlave = constant.SLAVE
	ch_slaveAlone := make(chan bool)
	ch_killTimer := make(chan bool)
	flagSlaveAlone := true // Assumes slave to be alone
	fmt.Println("Slave-state initialized")

	for {
		select {

		// Limit the number of messages over ch_repeatedBcast
		// NOTE PG: Do this later
		case localMatrix := <-ch_recieveSlaveLocal: // Update repeated Bcasts with last local state
			fmt.Println("stateSlave: localMatrix recieved")
			ch_repeatedBcast <- localMatrix
			fmt.Println("stateSlave: localMatrix sent to ch_repeatedBcast")
		case masterMatrix := <-ch_recieve: // Recieves masterMatrix on channel from master over UDP. //masterMatrix = <-ch_recieve:
			fmt.Println("stateSlave: masterMatrix recieved")
			fmt.Println(masterMatrix)
			if flagSlaveAlone == false {
				ch_killTimer <- true  // Kill time
				flagSlaveAlone = true // Reset timer-flag
			}
			ch_recieveLocal <- masterMatrix
			fmt.Println("stateSlave: masterMatrix sent on ch_recieveLocal")
		case <-ch_slaveAlone:
			fmt.Println("SLAVE ID ", LocalIP, "is transitioning to MASTER")
			return constant.MASTER
		default:
			if flagSlaveAlone == true {
				go slaveTimer(ch_slaveAlone, ch_killTimer)
				flagSlaveAlone = false
			}
		}
	}
}

func slaveTimer(ch_slaveAlone chan<- bool, ch_killTimer <-chan bool) {
	// Timer of 5 times the UPDATE_INTERVAL
	timer := time.NewTimer(5 * constant.UPDATE_INTERVAL * time.Millisecond)
	for {
		select {
		case <-timer.C:
			ch_slaveAlone <- true
			break
		case <-ch_killTimer:
			return
		}
	}
}

/* Communicates the master matrix to the elevator, and recieves data of the
elevators current state which is broadcast to master over UDP. */
func localOrderHandler(ch_recieveLocal <-chan [][]int, ch_transmitSlave chan<- [][]int, ch_elevRecieve chan<- [][]int, ch_elevTransmit <-chan [][]int, ch_recieveSlaveLocal chan<- [][]int) {
	localMatrix := InitLocalMatrix()
	ch_recieveSlaveLocal <- localMatrix
	for {
		select {
		case masterMatrix := <-ch_recieveLocal:
			// fmt.Println("localOrderHandler: Sending masterMatrix on ch_recieveLocal")
			fmt.Println("localOrderHandler: masterMatrix from ch_recieveLocal")
			fmt.Println(masterMatrix)
			ch_elevRecieve <- masterMatrix // masterMatrix TO elevator
			fmt.Println("localOrderHandler: ch_elevRecieve sent.")
			// fmt.Println("localOrderHandler: masterMatrix sent to ch_elevRecieve")
		case localMatrix = <-ch_elevTransmit: // localMatrix FROM elevator
			localMatrix[constant.UP_BUTTON][constant.SLAVE_MASTER] = int(flagMasterSlave) // Ensure correct state
			localMatrix[constant.UP_BUTTON][constant.IP] = LocalIP
			fmt.Println("case mottar fra ch_elevTransmit") // Ensure correct IP
			ch_transmitSlave <- localMatrix
			fmt.Println("mottok localMatrix")
			if flagMasterSlave == constant.SLAVE {
				ch_recieveSlaveLocal <- localMatrix
				fmt.Println("localOrderHandler: Sent localMatrix")
			}
		default:

		}
	}
}

/* Initialize local matrix, 2x(5+N_FLOORS)
   contains information about local elevator and UP/DOWN hall lights. */
func InitLocalMatrix() [][]int {
	localMatrix := make([][]int, 0)
	for i := 0; i <= 1; i++ {
		localMatrix = append(localMatrix, make([]int, 5+constant.N_FLOORS))
	}
	localMatrix[constant.UP_BUTTON][constant.IP] = LocalIP
	localMatrix[constant.UP_BUTTON][constant.DIR] = int(elevio.MD_Stop)
	localMatrix[constant.UP_BUTTON][constant.FLOOR] = elevio.GetFloorInit()
	localMatrix[constant.UP_BUTTON][constant.ELEV_STATE] = int(constant.IDLE)
	localMatrix[constant.UP_BUTTON][constant.SLAVE_MASTER] = int(flagMasterSlave)
	return localMatrix
}

/* Check if there are other masters in the recieved matrix.
   Lowest ID remains master. */
func checkMaster(matrix [][]int) constant.STATE {
	rows := len(matrix)
	for row := int(constant.FIRST_ELEV); row < rows; row++ {
		if matrix[row][constant.SLAVE_MASTER] == int(constant.MASTER) {
			if matrix[row][constant.IP] < LocalIP {
				return constant.SLAVE
			}
		}
	}
	return constant.MASTER
}

func InitMatrixMaster() [][]int {
	matrixMaster := make([][]int, 0)
	for i := 0; i <= 2; i++ { // For 1 elevator, master is assumed alone
		matrixMaster = append(matrixMaster, make([]int, 5+constant.N_FLOORS))
	}
	matrixMaster[constant.FIRST_ELEV][constant.IP] = LocalIP
	matrixMaster[constant.FIRST_ELEV][constant.DIR] = int(elevio.MD_Stop)
	matrixMaster[constant.FIRST_ELEV][constant.FLOOR] = elevio.GetFloorInit()
	matrixMaster[constant.FIRST_ELEV][constant.ELEV_STATE] = int(constant.IDLE)
	matrixMaster[constant.FIRST_ELEV][constant.SLAVE_MASTER] = int(constant.MASTER)
	return matrixMaster
}

/* Ticks every UPDATE_INTERVAL milliseconds */
func tickCounter(ch_updateInterval chan<- int) {
	ticker := time.NewTicker(constant.UPDATE_INTERVAL * time.Millisecond)
	for range ticker.C {
		ch_updateInterval <- 1
	}
}

/* *********************************************** */
/*               HELPER FUNCTIONS                  */

/* Check for disconnected peers, pass ID as string over channel */
func checkDisconnectedPeers(ch_peerUpdate <-chan peers.PeerUpdate, ch_peerDisconnected chan<- string) {
	for {
		if flagDisconnectedPeer == false {
			peerUpdate := <-ch_peerUpdate
			peerUpdateLost := peerUpdate.Lost
			fmt.Println("checkDisconnectedPeers: Lost peer-array: ")
			fmt.Println(peerUpdateLost)
			if len(peerUpdate.Lost) >= 1 {
				flagDisconnectedPeer = true
				fmt.Println("checkDisconnectedPeers: flagDisconnectedPeer = true")
				fmt.Println("checkDisconnectedPeers: Lost peers: ", peerUpdate.Lost[0])
				fmt.Println("checkDisconnectedPeers: peerReturnedIP: ", peerUpdateLost[0])
				ch_peerDisconnected <- peerUpdateLost[0]
				fmt.Println("checkDisconnectedPeers: Sent over ch_peerDisconnected")
			}
		}
	}
}

/* Delete peer with the corresponding IP */
func deleteDisconnectedPeer(matrixMaster [][]int, disconnectedIP string) [][]int {
	var peer int
	for _, c := range disconnectedIP {
		peer = int(c) // Cast the rune from string into a process ID integer
		for i := 1; i < 100; i++ {
			fmt.Println(peer)
		}
	}
	for row := int(constant.FIRST_ELEV); row < len(matrixMaster); row++ {
		if matrixMaster[row][constant.IP] == peer {
			fmt.Println("Old matrixMaster: ", matrixMaster)
			matrixMaster = append(matrixMaster[:row], matrixMaster[row+1:]...) // Delete row
			fmt.Println("New matrixMaster: ", matrixMaster)
		}
	}
	return matrixMaster
}

/* Merge info from recievedMatrix, append if new slave */
func mergeRecievedInfo(matrixMaster [][]int, recievedMatrix [][]int) [][]int {
	slaveIP := recievedMatrix[constant.UP_BUTTON][constant.IP]
	flagSlaveExist := false
	for row := int(constant.FIRST_ELEV); row < len(matrixMaster); row++ {
		if matrixMaster[row][constant.IP] == slaveIP {
			matrixMaster[row][constant.DIR] = recievedMatrix[constant.UP_BUTTON][constant.DIR]
			matrixMaster[row][constant.FLOOR] = recievedMatrix[constant.UP_BUTTON][constant.FLOOR]
			matrixMaster[row][constant.ELEV_STATE] = recievedMatrix[constant.UP_BUTTON][constant.ELEV_STATE]
			flagSlaveExist = true
		}
	}
	if flagSlaveExist == false {
		newSlave := make([]int, constant.FIRST_FLOOR+constant.N_FLOORS)
		copy(newSlave[0:constant.SLAVE_MASTER+1], recievedMatrix[constant.UP_BUTTON][0:constant.SLAVE_MASTER+1]) // Copy not inclusive for last index
		matrixMaster = append(matrixMaster, newSlave)
	}
	return matrixMaster
}

/* Removes served orders in the current floor of recievedMatrix */
func checkOrderServed(matrixMaster [][]int, recievedMatrix [][]int) [][]int {
	currentFloor := recievedMatrix[constant.UP_BUTTON][constant.FLOOR]
	if checkStoppedOrDoorsOpen(recievedMatrix) == true {
		matrixMaster[constant.UP_BUTTON][int(constant.FIRST_FLOOR)+currentFloor] = 0
		matrixMaster[constant.DOWN_BUTTON][int(constant.FIRST_FLOOR)+currentFloor] = 0
	}
	return matrixMaster
}
func checkStoppedOrDoorsOpen(recievedMatrix [][]int) bool {
	if recievedMatrix[constant.UP_BUTTON][constant.ELEV_STATE] == int(constant.STOP) {
		return true
	}
	if recievedMatrix[constant.UP_BUTTON][constant.ELEV_STATE] == int(constant.DOORS_OPEN) {
		return true
	}
	return false
}

/* Insert unconfirmed orders UP/DOWN into matrixMaster */
func mergeUnconfirmedOrders(matrixMaster [][]int, recievedMatrix [][]int) [][]int {
	for row := constant.UP_BUTTON; row <= constant.DOWN_BUTTON; row++ {
		for col := constant.FIRST_FLOOR; col < (constant.N_FLOORS + constant.FIRST_FLOOR); col++ {
			if recievedMatrix[row][col] == 1 {
				matrixMaster[row][col] = 1
			}
		}
	}
	return matrixMaster
}

/* Clear the elevators' current orders */
func clearCurrentOrders(matrix [][]int) [][]int {
	for floor := int(constant.FIRST_FLOOR); floor < len(matrix[constant.UP_BUTTON]); floor++ {
		for elev := int(constant.FIRST_ELEV); elev < len(matrix); elev++ {
			matrix[elev][floor] = 0
		}
	}
	return matrix
}

/* Order distribution algorithm */
func calculateElevatorStops(matrix [][]int) [][]int {
	var flagOrderSet bool
	rowLength := len(matrix[constant.UP_BUTTON])
	colLength := len(matrix)

	for floor := int(constant.FIRST_FLOOR); floor < rowLength; floor++ {
		flagOrderSet = false
		if matrix[constant.UP_BUTTON][floor] == 1 || matrix[constant.DOWN_BUTTON][floor] == 1 {
			// Check if there's an elevator in the floor
			for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
				// If in floor, give order if elevator is idle, stopped or has doors open
				if matrix[elev][constant.FLOOR] == floor && (matrix[elev][constant.ELEV_STATE] == int(constant.IDLE) ||
					matrix[elev][constant.ELEV_STATE] == int(constant.STOP) || matrix[elev][constant.ELEV_STATE] == int(constant.DOORS_OPEN)) {
					matrix[elev][floor] = 1 // Stop here
					flagOrderSet = true
					break
				}
			}

			//For både opp og ned bestilling
			if flagOrderSet == false && matrix[constant.UP_BUTTON][floor] == 1 && matrix[constant.DOWN_BUTTON][floor] == 1 {
				index := 1
				for {
					for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
						//Sjekker under meg, som har retning opp innenfor grense
						if flagOrderSet == false && (matrix[elev][constant.FLOOR] == (floor - int(constant.FIRST_FLOOR) - index)) && (matrix[elev][constant.DIR] == int(elevio.MD_Up) || matrix[elev][constant.DIR] == int(elevio.MD_Stop)) && (floor-index >= int(constant.FIRST_FLOOR)) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
						//Sjekk over meg, som har retning ned og innenfor grensa
						if flagOrderSet == false && (matrix[elev][constant.FLOOR] == (floor - int(constant.FIRST_FLOOR) + index)) && (matrix[elev][constant.DIR] == int(elevio.MD_Down) || matrix[elev][constant.DIR] == int(elevio.MD_Stop)) && (floor+index <= int(constant.FIRST_FLOOR)+constant.N_FLOORS) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
					}
					//Gått igjennom alle heisene
					index++
					//Hvis ordre gitt eller utenfor bounds UTEN å ha funnet kandidat
					if flagOrderSet == true || ((floor-index) < int(constant.FIRST_FLOOR)) && (floor+index > (int(constant.FIRST_FLOOR)+constant.N_FLOORS)) {
						break
					}
				}
				// --------------------------------------------------------------------
				// For OPP bestilling
			} else if flagOrderSet == false && matrix[constant.UP_BUTTON][floor] == 1 {
				index := 1
				for {
					for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
						//Sjekker under meg, som har retning opp innenfor grense
						if flagOrderSet == false && (matrix[elev][constant.FLOOR] == (floor - int(constant.FIRST_FLOOR) - index)) && (matrix[elev][constant.DIR] == int(elevio.MD_Up) || matrix[elev][constant.DIR] == int(elevio.MD_Stop)) && (floor-index >= int(constant.FIRST_FLOOR)) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
						//Sjekk over meg, som har retning ned og innenfor grensa
						if flagOrderSet == false && (matrix[elev][constant.FLOOR] == (floor - int(constant.FIRST_FLOOR) + index)) && (matrix[elev][constant.DIR] == int(elevio.MD_Stop)) && (floor+index <= int(constant.FIRST_FLOOR)+constant.N_FLOORS) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
					}
					//Gått igjennom alle heisene
					index++
					//Hvis ordre gitt eller utenfor bounds UTEN å ha funnet kandidat
					if flagOrderSet == true || ((floor-index) < int(constant.FIRST_FLOOR)) && (floor+index > (int(constant.FIRST_FLOOR)+constant.N_FLOORS)) {
						break
					}
				}
				// --------------------------------------------------------------------
				//For bestilling NED
			} else if flagOrderSet == false && matrix[constant.DOWN_BUTTON][floor] == 1 {
				index := 1
				for {
					for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
						//Sjekker under meg, som har retning opp innenfor grense
						if flagOrderSet == false && (matrix[elev][constant.FLOOR] == (floor - int(constant.FIRST_FLOOR) - index)) && (matrix[elev][constant.DIR] == int(elevio.MD_Stop)) && (floor-index) >= int(constant.FIRST_FLOOR) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
						//Sjekk over meg, som har retning ned og innenfor grensa
						if flagOrderSet == false && (matrix[elev][constant.FLOOR] == (floor - int(constant.FIRST_FLOOR) + index)) && (matrix[elev][constant.DIR] == int(elevio.MD_Down) || matrix[elev][constant.DIR] == int(elevio.MD_Stop)) && (floor+index <= int(constant.FIRST_FLOOR)+constant.N_FLOORS) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
					}
					//Gått igjennom alle heisene
					index++
					//Hvis ordre gitt eller utenfor bounds UTEN å ha funnet kandidat
					if flagOrderSet == true || ((floor-index) < int(constant.FIRST_FLOOR)) && (floor+index > (int(constant.FIRST_FLOOR)+constant.N_FLOORS)) {
						break
					}
				}
			}

			// Give to master if no elevator has gotten the order
			if flagOrderSet == false {
				for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
					if matrix[elev][constant.SLAVE_MASTER] == int(constant.MASTER) {
						matrix[elev][floor] = 1
					}
				}
			}

		} // End order condition
	} // End inf loop
	// fmt.Println("calculateElevatorStops: Orders calculated.")
	return matrix
} // End floor loop

/*Broadcasts last item over ch_repeatedBcast */
func repeatedBroadcast(ch_repeatedBcast <-chan [][]int, ch_updateInterval <-chan int, ch_transmit chan<- [][]int, ch_transmitSlave chan<- [][]int) {
	var matrix [][]int
	matrix = <-ch_repeatedBcast

	for {
		select {
		case matrix = <-ch_repeatedBcast:
		default:
			<-ch_updateInterval      // Send over channel once each UPDATE_INTERVAL
			switch flagMasterSlave { // Send over channel dependent on MASTER/SLAVE state
			case constant.MASTER:
				ch_transmit <- matrix
			case constant.SLAVE:
				ch_transmitSlave <- matrix
			}
		}
	}
}

func debugCheckMatrixEqual(m1 [][]int, m2 [][]int) bool {
	length_m1 := len(m1)
	length_m2 := len(m2)
	if length_m1 != length_m2 {
		return false
	}
	for row := 0; row < length_m1; row++ {
		for col := 0; col < len(m1[0]); col++ {
			if m1[row][col] != m2[row][col] {
				return false
			}
		}
	}
	return true
}

/*  Converts the IP to an int. Example:
    "10.100.23.253" -> 253 */
// func getLocalIP() int {
// 	returnedIP, err := localip.LocalIP()
// 	if err != nil {
// 		fmt.Println(err)
// 		returnedIP = "DISCONNECTED"
// 	}
// 	IPlength := len(returnedIP)
// 	for i := IPlength - 1; i > 0; i-- {
// 		if returnedIP[i] == '.' {
// 			returnedIP = returnedIP[i+1 : IPlength]
// 			break
// 		}
// 	}
// 	return file_IO.StringToNumbers(returnedIP)[0] // Vector of 1 element
// }
