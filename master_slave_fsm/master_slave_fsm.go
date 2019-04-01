package master_slave_fsm

import (
	"fmt"

	// "os" // For getPID
	"time"

	"../constant"

	"../elevator"
	"../elevator/elevio"
	"../file_IO"
	"../network/bcast"
	"../network/localip"
	"../network/peers"
)

var flagDisconnectedPeer bool = false
var flagMasterSlave constant.STATE

func ConnectToElevator(elevatorAddress string) {
	fmt.Println("Elevator address: ", elevatorAddress)
	elevio.Init(elevatorAddress, constant.N_FLOORS) // Init elevatorServer
}

func InitMasterSlave() {
	// fmt.Println("Initializing Master/Slave state machine...")
	var matrixMaster [][]int

	// fullLocalIP, _ := LocalIP.LocalIP() // CURRENTLY PASSED TO PEERS TRANSMITTER. UNSURE
	// fmt.Println("This machines LocalIP-ID is: ", LocalIP)

	ch_elevTransmit := make(chan [][]int, 2*constant.N_FLOORS) // Elevator transmission, FROM elevator
	ch_elevRecieve := make(chan [][]int, 2*constant.N_FLOORS)  // Elevator reciever,	TO elevator

	ch_updateInterval := make(chan int) // Periodic update-ticks
	ch_peerUpdate := make(chan peers.PeerUpdate)
	ch_peerEnable := make(chan bool)
	ch_transmit := make(chan [][]int, constant.N_FLOORS)          // Master matrix transmission
	ch_recieve := make(chan [][]int, constant.N_FLOORS)           // Master matrix reciever
	ch_transmitSlave := make(chan [][]int, constant.N_FLOORS)     // Slave matrix transmission
	ch_recieveSlave := make(chan [][]int, constant.N_FLOORS)      // Slave matrix reciever
	ch_recieveLocal := make(chan [][]int, constant.N_FLOORS)      // Local master matrix transfer
	ch_recieveSlaveLocal := make(chan [][]int, constant.N_FLOORS) // Local slave matrix transfer
	ch_peerDisconnected := make(chan string, constant.N_FLOORS)
	ch_repeatedBcast := make(chan [][]int, 2*constant.N_FLOORS)

	elevator.TakeElevatorToNearestFloor()

	ch_buttonPressed := make(chan bool) // Sends periodic updates

	go elevator.InitElevator(ch_elevTransmit, ch_elevRecieve, ch_buttonPressed)

	// Communicates with the local elevator
	go localOrderHandler(ch_recieveLocal, ch_transmitSlave, ch_elevRecieve, ch_elevTransmit, ch_recieveSlaveLocal)

	go peers.Transmitter(constant.PORT_peers, string(constant.LocalIP), ch_peerEnable)
	// go peers.Transmitter(PORT_peers, fullLocalIP, ch_peerEnable)
	go peers.Receiver(constant.PORT_peers, ch_peerUpdate)

	// Spawn transmission/reciever goroutines.
	go bcast.Transmitter(constant.PORT_bcast, ch_transmit)
	go bcast.Receiver(constant.PORT_bcast, ch_recieve)
	go bcast.Transmitter(constant.PORT_slaveBcast, ch_transmitSlave)
	go bcast.Receiver(constant.PORT_slaveBcast, ch_recieveSlave)

	// Start this one in master and slave state, kill the goroutine in between state change?
	// Must be able to use different channels depending on master or slave.
	// Or just pass all channels, and use a for-switch loop depending on master/slave
	go repeatedBroadcast(ch_repeatedBcast, ch_updateInterval, ch_transmit, ch_transmitSlave)
	// Start the update_interval ticker.
	go tickCounter(ch_updateInterval)

	// Check for DCed peers
	go checkDisconnectedPeers(ch_peerUpdate, ch_peerDisconnected)

	// fmt.Println("Master/Slave state machine initialized.")
	stateChange(matrixMaster, constant.SLAVE, ch_recieve, ch_recieveSlave, ch_peerDisconnected, ch_repeatedBcast, ch_recieveLocal, ch_recieveSlaveLocal, ch_buttonPressed)
}

// func elevListen(ch_elevTransmit <-chan [][]int, ch_elevRecieve <-chan [][]int) {
// 	for {
// 		select {
// 		case <-ch_elevTransmit:
// 			// fmt.Println("elevListen: ch_elevTransmit")
// 		case <-ch_elevRecieve:
// 			// fmt.Println("elevListen: ch_elevRecieve")
// 		}
// 	}
// }

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
	ch_sendMatrixMaster := make(chan bool) // Updates the internal matrixMaster at intervals
	ch_calcElevatorStops := make(chan bool)
	// fmt.Println("Masterstate activated.")
	go tickCounterCustom(ch_sendMatrixMaster)
	go tickCounterCustomCalcOrders(ch_calcElevatorStops)

	// If matrixMaster is empty, generate masterMatrix for 1 elevator
	if matrixMaster == nil {
		matrixMaster = InitMatrixMaster()
	}
	ch_repeatedBcast <- matrixMaster // Start the correct masterMatrix UDP broadcast

	for {
		select {
		case newMatrixMaster := <-ch_recieve:
			// // fmt.Println("stateMaster: Recieved masterMatrix")
			if checkMaster(newMatrixMaster) == constant.SLAVE {
				// fmt.Println("stateMaster: checkMaster returned SLAVE")
				return constant.SLAVE // Change to slave
			}
		case disconnectedIP := <-ch_peerDisconnected:
			// fmt.Println("Recieved over ch_peerDisconnect")
			matrixMaster = deleteDisconnectedPeer(matrixMaster, disconnectedIP)
			flagDisconnectedPeer = false
			fmt.Println("flagDisconnectedPeer = false")
			// fmt.Println("case disconnectedIP: FINISHED")
		case recievedMatrix := <-ch_recieveSlave:
			// Merge info from recievedMatrix, append if new slave
			matrixMaster = mergeRecievedInfo(matrixMaster, recievedMatrix)
			// Insert unconfirmed orders UP/DOWN into matrixMaster
			matrixMaster = mergeUnconfirmedOrders(matrixMaster, recievedMatrix)
			// Remove served order at current floor in recievedMatrix
			matrixMaster = checkOrderServed(matrixMaster, recievedMatrix)
		case <-ch_calcElevatorStops:
			// Clear orders
			fmt.Println("matrixMaster before calculateElevatorStops: ", matrixMaster)
			matrixMaster = clearCurrentOrders(matrixMaster)
			// Calculate stop
			matrixMaster = calculateElevatorStops(matrixMaster)
			fmt.Println("----- matrixMaster after calculateElevatorStops: ", matrixMaster)
		case <-ch_sendMatrixMaster:
			// Broadcast the whole
			go sendMatrixMasterToElevator(ch_buttonPressed, ch_recieveLocal, matrixMaster)
			// // fmt.Println("MASTER: Sent on ch_recieveLocal")
			ch_repeatedBcast <- matrixMaster
			// // fmt.Println("MASTER: Sent on ch_repeatBcast")
		default:
		}
	}
}

func sendMatrixMasterToElevator(ch_buttonPressed <-chan bool, ch_recieveLocal chan<- [][]int, matrixMaster [][]int) {
	<-ch_buttonPressed
	ch_recieveLocal <- matrixMaster // Send to local elevator (localOrderHandler)
}

/* Slave state, checks for alive masters. Transitions if no masters on UDP. */
func stateSlave(ch_recieve <-chan [][]int, ch_repeatedBcast chan<- [][]int, ch_recieveLocal chan<- [][]int, ch_recieveSlaveLocal <-chan [][]int, ch_buttonPressed <-chan bool) constant.STATE {
	// var masterMatrix [][]int		// masterMatrix not used, only checks for signal on channel
	ch_slaveAlone := make(chan bool)
	ch_killTimer := make(chan bool)
	ch_sendMasterMatrix := make(chan bool, 2*constant.N_FLOORS) // Updates the internal matrixMaster at intervals
	flagSlaveAlone := true                                      // Assumes slave to be alone

	go tickCounterCustom(ch_sendMasterMatrix)
	var masterMatrix [][]int
	// fmt.Println("Slave-state initialized")
	// USE ch_repeatedBcast <- matrixMaster

	for {
		select {
		case localMatrix := <-ch_recieveSlaveLocal: // Update repeated Bcasts with last local state
			// fmt.Println("Slave iteration: ", i)
			// fmt.Println("stateSlave: localMatrix recieved")
			ch_repeatedBcast <- localMatrix
			// fmt.Println("stateSlave: localMatrix sent to ch_repeatedBcast")
		case masterMatrix = <-ch_recieve: // Recieves masterMatrix on channel from master over UDP. //masterMatrix = <-ch_recieve:
			// fmt.Println("SLAVE: UDP: ", masterMatrix)
			// fmt.Println("Slave iteration: ", i)
			// fmt.Println("stateSlave: ID = ", LocalIP)
			// fmt.Println("stateSlave: masterMatrix recieved: ", masterMatrix)
			if flagSlaveAlone == false {
				// fmt.Println("if flagSlaveAlone clause triggered")
				ch_killTimer <- true  // Kill time
				flagSlaveAlone = true // Reset timer-flag
				// fmt.Println("if flagSlaveAlone clause finished")
			}
		case <-ch_sendMasterMatrix:
			if len(masterMatrix) > 0 { // Requires an actual masterMatrix
				// ch_recieveLocal <- masterMatrix
				go sendToChannel(ch_recieveLocal, masterMatrix)
			}
			// fmt.Println("stateSlave: masterMatrix sent on ch_recieveLocal")
		case <-ch_slaveAlone:
			// fmt.Println("SLAVE ID ", LocalIP, "is transitioning to MASTER")
			return constant.MASTER
		default:
			if flagSlaveAlone == true {
				go slaveTimer(ch_slaveAlone, ch_killTimer)
				flagSlaveAlone = false
			}
		}

	}
	// return MASTER
}

func sendToChannel(ch_transmit chan<- [][]int, matrix [][]int) {
	ch_transmit <- matrix
	// fmt.Println("sendToChannel: ", matrix)
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
	ch_sendToElevTick := make(chan bool)
	go tickCounterCustom(ch_sendToElevTick)
	ch_recieveSlaveLocal <- localMatrix
	prevMasterMatrix := <-ch_recieveLocal
	i := 1
	for {
		select {
		case localMatrix = <-ch_elevTransmit: // localMatrix FROM elevator
			localMatrix[constant.UP_BUTTON][constant.SLAVE_MASTER] = int(flagMasterSlave) // Ensure correct state
			localMatrix[constant.UP_BUTTON][constant.IP] = constant.LocalIP
			// // fmt.Println("case mottar fra ch_elevTransmit") // Ensure correct IP
			// fmt.Println("localOrderHandler: Waiting on ch_transmitSlave... ")
			ch_transmitSlave <- localMatrix
			// fmt.Println("localOrderHandler: Sent to slave module: ", localMatrix)
			if flagMasterSlave == constant.SLAVE { // COMMENT THIS BACK IN
				ch_recieveSlaveLocal <- localMatrix
				// // fmt.Println("localOrderHandler: Sent localMatrix")
			}

		case masterMatrix := <-ch_recieveLocal:
			prevMasterMatrix = masterMatrix
			// fmt.Println("localOrderHandler: Recieved ch_recieveLocal; ")
			// fmt.Println(masterMatrix)

		case <-ch_sendToElevTick:
			// fmt.Println("localOrderHandler: Attempting to send to elevator...")
			ch_elevRecieve <- prevMasterMatrix // masterMatrix TO elevator
			// go sendToChannel(ch_elevRecieve, masterMatrix)
			// fmt.Println("localOrderHandler: Sent to elevator: ", prevMasterMatrix)
			// fmt.Println("ch_elevRecieve queue size: ", len(ch_elevRecieve))
			// fmt.Println("localOrderHandler: Iteration ", i)
			i++
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
	localMatrix[constant.UP_BUTTON][constant.IP] = constant.LocalIP
	localMatrix[constant.UP_BUTTON][constant.DIR] = int(elevio.MD_Stop)
	localMatrix[constant.UP_BUTTON][constant.FLOOR] = elevio.GetFloorInit()
	localMatrix[constant.UP_BUTTON][constant.ELEV_STATE] = int(constant.IDLE)
	localMatrix[constant.UP_BUTTON][constant.SLAVE_MASTER] = int(flagMasterSlave)
	return localMatrix
}

/* Check if there are other masters in the recieved matrix.
   Lowest IP remains master.
   Return MASTER if remain master, SLAVE if transition to slave */
func checkMaster(matrix [][]int) constant.STATE {
	rows := len(matrix)
	for row := int(constant.FIRST_ELEV); row < rows; row++ {
		if matrix[row][constant.SLAVE_MASTER] == int(constant.MASTER) {
			// // fmt.Println("checkMaster: Found master in matrix.")
			// // fmt.Println("matrix[row][IP] = ", matrix[row][constant.IP], ". LocalIP = ", LocalIP)
			if matrix[row][constant.IP] < constant.LocalIP {
				return constant.SLAVE //
			}
		}
	}

	return constant.MASTER //
}

func InitMatrixMaster() [][]int {
	matrixMaster := make([][]int, 0)
	for i := 0; i <= 2; i++ { // For 1 elevator, master is assumed alone
		matrixMaster = append(matrixMaster, make([]int, 5+constant.N_FLOORS))
	}
	matrixMaster[constant.FIRST_ELEV][constant.IP] = constant.LocalIP
	matrixMaster[constant.FIRST_ELEV][constant.DIR] = int(elevio.MD_Stop)
	matrixMaster[constant.FIRST_ELEV][constant.FLOOR] = elevio.GetFloorInit()
	matrixMaster[constant.FIRST_ELEV][constant.ELEV_STATE] = int(constant.IDLE)
	matrixMaster[constant.FIRST_ELEV][constant.SLAVE_MASTER] = int(constant.MASTER)
	return matrixMaster
}

/*  Converts the IP to an int. Example:
    "10.100.23.253" -> 253 */
func getLocalIP() int {
	returnedIP, err := localip.LocalIP()
	if err != nil {
		// fmt.Println(err)
		returnedIP = "DISCONNECTED"
	}

	IPlength := len(returnedIP)
	for i := IPlength - 1; i > 0; i-- {
		if returnedIP[i] == '.' {
			returnedIP = returnedIP[i+1 : IPlength]
			break
		}
	}
	return file_IO.StringToNumbers(returnedIP)[0] // Vector of 1 element
}

/* Ticks every UPDATE_INTERVAL milliseconds */
func tickCounter(ch_updateInterval chan<- int) {
	ticker := time.NewTicker(constant.UPDATE_INTERVAL * time.Millisecond)
	for range ticker.C {
		ch_updateInterval <- 1
	}
}
func tickCounterCustom(ch_updateInterval chan<- bool) {
	ticker := time.NewTicker(constant.UPDATE_MASTER_SLAVE * time.Millisecond)
	for range ticker.C {
		ch_updateInterval <- true
	}
}
func tickCounterCustomCalcOrders(ch_updateInterval chan<- bool) {
	ticker := time.NewTicker(constant.UPDATE_ORDER_CALCULATION * time.Millisecond)
	for range ticker.C {
		ch_updateInterval <- true
	}
}

/* *********************************************** */
/*               HELPER FUNCTIONS                  */

/* Check for disconnected peers, pass IP as int over channel */
// func checkDisconnectedPeers(ch_peerUpdate <-chan peers.PeerUpdate, ch_peerDisconnected chan<- int) {
// 	for {
// 		if flagDisconnectedPeer == false {
// 			peerUpdate := <-ch_peerUpdate
// 			if len(peerUpdate.Lost) > 0 { // A peer has DC'ed
// 				flagDisconnectedPeer = true
// 				peerIP := file_IO.StringToNumbers(peerUpdate.Lost[0])[0]
// 				ch_peerDisconnected <- peerIP
// 			}
// 		}
// 	}
// }

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
				// fmt.Println("checkDisconnectedPeers: flagDisconnectedPeer = true")
				fmt.Println("checkDisconnectedPeers: Lost peers: ", peerUpdate.Lost[0])
				ch_peerDisconnected <- peerUpdateLost[0]
				fmt.Println("checkDisconnectedPeers: Sent over ch_peerDisconnected")
			}
		}
	}
}

// func splitAtPeriodReturnLastItem(str string) int {
// 	s := strings.Split(str, ".")
// 	sInt, _ := strconv.Atoi(s[len(s)-1])
// 	return sInt
// }

/* Delete peer with the corresponding IP */
func deleteDisconnectedPeer(matrixMaster [][]int, disconnectedIP string) [][]int {
	var peer int
	for _, c := range disconnectedIP {
		peer = int(c) // Cast the rune from string into a process ID integer
	}
	for row := int(constant.FIRST_ELEV); row < len(matrixMaster); row++ {
		if matrixMaster[row][constant.IP] == peer {
			fmt.Println("peers: Old matrixMaster: ", matrixMaster)
			matrixMaster = append(matrixMaster[:row], matrixMaster[row+1:]...) // Delete row
			fmt.Println("peers: New matrixMaster: ", matrixMaster)
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
	// fmt.Println("Recieved: ", matrix)
	// // fmt.Println("calculateElevatorStops: Calculate stops")
	var flagOrderSet bool
	rowLength := len(matrix[0])
	colLength := len(matrix)
	var currentFloor int

	// fmt.Println(" ")
	// fmt.Println(" ")

	for floor := int(constant.FIRST_FLOOR); floor < rowLength; floor++ {
		currentFloor = floor - int(constant.FIRST_FLOOR)
		// fmt.Println("Floor: ", floor)
		flagOrderSet = false

		//SJEKKER OM VI HAR EN BESTILLING SOM MÅ DELEGERES
		if matrix[constant.UP_BUTTON][floor] == 1 || matrix[constant.DOWN_BUTTON][floor] == 1 {

			//Sjekker om jeg har en heis i etasjen
			for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
				if matrix[elev][constant.FLOOR] == currentFloor && (matrix[elev][constant.ELEV_STATE] != int(constant.MOVE)) {
					// fmt.Println("ORDER: CURRENT FLOOR: ")
					matrix[elev][floor] = 1 // Stop here
					flagOrderSet = true
					break
				}
			}

			//For både opp og ned bestilling
			if flagOrderSet == false && matrix[constant.UP_BUTTON][floor] == 1 && matrix[constant.DOWN_BUTTON][floor] == 1 {
				index := 1
			labelOuterLoop1:
				for {
					for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
						// fmt.Println("index up and down: ", index)
						//Sjekker under meg, som har retning opp innenfor grense
						if flagOrderSet == false &&
							(matrix[elev][constant.FLOOR] == (currentFloor - index)) &&
							(matrix[elev][constant.DIR] == int(elevio.MD_Up) || matrix[elev][constant.ELEV_STATE] == int(constant.IDLE)) &&
							(floor-index >= int(constant.FIRST_FLOOR)) {
							matrix = deleteOtherOrdersOnFloor(matrix, elev, floor)
							matrix[elev][floor] = 1
							flagOrderSet = true
							// fmt.Println("ORDER: BOTH BUTTONS - BELOW FLOOR - DIR UP: Elev: ", elev-int(constant.FIRST_ELEV), " Floor: ", floor-int(constant.FIRST_FLOOR))
							break labelOuterLoop1
						}
						//Sjekk over meg, som har retning ned og innenfor grensa
						if flagOrderSet == false &&
							(matrix[elev][constant.FLOOR] == (currentFloor + index)) &&
							(matrix[elev][constant.DIR] == int(elevio.MD_Down) || matrix[elev][constant.ELEV_STATE] == int(constant.IDLE)) &&
							(floor+index <= int(constant.FIRST_FLOOR)+constant.N_FLOORS) {
							matrix = deleteOtherOrdersOnFloor(matrix, elev, floor)
							matrix[elev][floor] = 1
							flagOrderSet = true
							// fmt.Println("ORDER: BOTH BUTTONS - ABOVE FLOOR - DIR DOWN: Elev: ", elev-int(constant.FIRST_ELEV), ", Floor: ", floor-int(constant.FIRST_FLOOR))
							break labelOuterLoop1
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
			labelOuterLoop2:
				for {
					for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
						//Sjekker under meg, som har retning opp innenfor grense
						if flagOrderSet == false &&
							(matrix[elev][constant.FLOOR] == (currentFloor - index)) &&
							(matrix[elev][constant.DIR] == int(elevio.MD_Up) || matrix[elev][constant.ELEV_STATE] == int(constant.IDLE)) &&
							(floor-index >= int(constant.FIRST_FLOOR)) {
							matrix = deleteOtherOrdersOnFloor(matrix, elev, floor)
							matrix[elev][floor] = 1
							flagOrderSet = true
							// fmt.Println("ORDER: UP BUTTONS - BELOW FLOOR - DIR UP: Elev: ", elev-int(constant.FIRST_ELEV), " Floor: ", floor-int(constant.FIRST_FLOOR))
							break labelOuterLoop2
						}
						//Sjekk over meg, som har retning ned og innenfor grensa ///HEEEER!!!! Eller rening NED. Hva med state?
						if flagOrderSet == false &&
							(matrix[elev][constant.FLOOR] == (currentFloor + index)) &&
							(matrix[elev][constant.ELEV_STATE] == int(constant.IDLE) || matrix[elev][constant.ELEV_STATE] == int(constant.MOVE)) &&
							(floor+index <= int(constant.FIRST_FLOOR)+constant.N_FLOORS) {
							matrix = deleteOtherOrdersOnFloor(matrix, elev, floor)
							matrix[elev][floor] = 1
							flagOrderSet = true
							// fmt.Println("ORDER: UP BUTTONS - ABOVE FLOOR - DIR DOWN: Elev: ", elev-int(constant.FIRST_ELEV), " Floor: ", floor-int(constant.FIRST_FLOOR))
							break labelOuterLoop2
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
			labelOuterLoop3:
				for {
					for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
						//Sjekk over meg, som har retning ned og innenfor grensa
						if flagOrderSet == false && (matrix[elev][constant.FLOOR] == (currentFloor + index)) && (matrix[elev][constant.DIR] == int(elevio.MD_Down) || matrix[elev][constant.ELEV_STATE] == int(constant.IDLE)) && (floor+index <= int(constant.FIRST_FLOOR)+constant.N_FLOORS) {
							matrix = deleteOtherOrdersOnFloor(matrix, elev, floor)
							matrix[elev][floor] = 1
							flagOrderSet = true
							// fmt.Println("ORDER: DOWN BUTTONS - ABOVE FLOOR - DIR DOWN: Elev: ", elev-int(constant.FIRST_ELEV), " Floor: ", floor-int(constant.FIRST_FLOOR))
							break labelOuterLoop3
						}
						//Sjekker under meg, som har retning opp innenfor grense
						if flagOrderSet == false && (matrix[elev][constant.FLOOR] == (currentFloor - index)) && (matrix[elev][constant.ELEV_STATE] == int(constant.IDLE) || matrix[elev][constant.ELEV_STATE] == int(constant.MOVE)) && (floor-index) >= int(constant.FIRST_FLOOR) {
							matrix = deleteOtherOrdersOnFloor(matrix, elev, floor)
							matrix[elev][floor] = 1
							flagOrderSet = true
							// fmt.Println("ORDER: DOWN BUTTONS - BELOW FLOOR - DIR UP: Elev: ", elev-int(constant.FIRST_ELEV), " Floor: ", floor-int(constant.FIRST_FLOOR))
							break labelOuterLoop3
						}
					}
					//Gått igjennom alle heisene
					index++
					//Hvis ordre gitt eller utenfor bounds UTEN å ha funnet kandidat
					if flagOrderSet == true || ((floor-index) < int(constant.FIRST_FLOOR)) && (floor+index > (int(constant.FIRST_FLOOR)+constant.N_FLOORS)) {
						// fmt.Println("Floor-iteration complete")
						break
					}
				}
			}
		} // End order condition
	} // End inf loop
	// // fmt.Println("calculateElevatorStops: Orders calculated.")
	// fmt.Println("Calculated Stops: ", matrix)
	return matrix
} // End floor loop
func deleteOtherOrdersOnFloor(matrix [][]int, elev int, floor int) [][]int {
	for row := int(constant.FIRST_ELEV); row < len(matrix); row++ {
		if row != elev {
			matrix[row][floor] = 0
		}
	}
	return matrix
}

/*Broadcasts last item over ch_repeatedBcast */
func repeatedBroadcast(ch_repeatedBcast <-chan [][]int, ch_updateInterval <-chan int, ch_transmit chan<- [][]int, ch_transmitSlave chan<- [][]int) {
	var matrix [][]int
	// // fmt.Println("repeatedBroadcast: Waiting on ch_repeatedBcast...")
	matrix = <-ch_repeatedBcast
	// prev_matrix := matrix
	// // fmt.Println("repeatedBroadcast: Recieved over ch_repeatedBcast: ", matrix)
	for {
		select {
		case msg := <-ch_repeatedBcast:
			// // fmt.Println("repeatedBroadcast: Recieved matrix over ch_repeatedBcast")

			//	// fmt.Println(msg)
			matrix = msg
			// if !debugCheckMatrixEqual(matrix, prev_matrix) {
			// 	// fmt.Println("Master matrix = ", matrix)
			// }
		default:
			<-ch_updateInterval      // Send over channel once each UPDATE_INTERVAL
			switch flagMasterSlave { // Send over channel dependent on MASTER/SLAVE state
			case constant.MASTER:
				ch_transmit <- matrix
			case constant.SLAVE:
				ch_transmitSlave <- matrix
				// // fmt.Println("repeatedBroadcast: Sent matrix over ch_transmitSlave (SLAVE)")
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
