package master_slave_fsm

import (
	"time"

	"../constant"
	"../elevator"
	"../elevator/elevio"
	"../network/bcast"
	"../network/peers"
)

/* masterMatrix dim: (2+N_ELEVATORS) x (5+N_FLOORS) */
/*           | IP | DIR | FLOOR | ELEV_STATE | Slave/Master | Stop1 | .. | Stop N | */
/* UP lights | x  |  x  |       |      x     |       x      |       | .. |    x   | */
/* DN lights | x  |  x  |       |      x     |       x      |   x   | .. |        | */
/* ELEV 1    |    |     |       |            |              |       | .. |        | */
/* ...       |    |     |       |            |              |       | .. |        | */
/* ELEV N    |    |     |       |            |              |       | .. |        | */

/* GLOBAL VARIABLES */
var flagDisconnectedPeer bool = false
var flagMasterSlave constant.STATE
var previousMaster int = -1

func ConnectToElevator(elevatorAddress string) {
	elevio.Init(elevatorAddress, constant.N_FLOORS) // Init elevatorServer
}

func InitMasterSlave() {
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
	ch_buttonPressed := make(chan bool)

	elevator.TakeElevatorToNearestFloor()

	go elevator.InitElevator(ch_elevTransmit, ch_elevRecieve, ch_buttonPressed)

	go localOrderHandler(ch_recieveLocal, ch_transmitSlave, ch_elevRecieve, ch_elevTransmit, ch_recieveSlaveLocal)

	go peers.Transmitter(constant.PORT_peers, string(constant.LocalIP), ch_peerEnable)
	go peers.Receiver(constant.PORT_peers, ch_peerUpdate)

	go bcast.Transmitter(constant.PORT_bcast, ch_transmit)
	go bcast.Receiver(constant.PORT_bcast, ch_recieve)
	go bcast.Transmitter(constant.PORT_slaveBcast, ch_transmitSlave)
	go bcast.Receiver(constant.PORT_slaveBcast, ch_recieveSlave)

	go repeatedBroadcast(ch_repeatedBcast, ch_updateInterval, ch_transmit, ch_transmitSlave)
	// Start the update_interval ticker.
	go tickCounter(ch_updateInterval)
	go checkDisconnectedPeers(ch_peerUpdate, ch_peerDisconnected)

	stateChange(constant.SLAVE, ch_recieve, ch_recieveSlave, ch_peerDisconnected, ch_repeatedBcast, ch_recieveLocal, ch_recieveSlaveLocal, ch_buttonPressed)
}

/* Continously swapping states */
func stateChange(currentState constant.STATE, ch_recieve <-chan [][]int, ch_recieveSlave <-chan [][]int, ch_peerDisconnected <-chan string, ch_repeatedBcast chan<- [][]int, ch_recieveLocal chan<- [][]int, ch_recieveSlaveLocal <-chan [][]int, ch_buttonPressed <-chan bool) {
	var orderMatrix [][]int
	for {
		switch currentState {
		case constant.MASTER:
			currentState = stateMaster(orderMatrix, ch_recieve, ch_recieveSlave, ch_peerDisconnected, ch_repeatedBcast, ch_recieveLocal, ch_buttonPressed)
		case constant.SLAVE:
			currentState, orderMatrix = stateSlave(ch_recieve, ch_repeatedBcast, ch_recieveLocal, ch_recieveSlaveLocal, ch_buttonPressed)
		}
	}
}

func stateMaster(orderMatrix [][]int, ch_recieve <-chan [][]int, ch_recieveSlave <-chan [][]int, ch_peerDisconnected <-chan string, ch_repeatedBcast chan<- [][]int, ch_recieveLocal chan<- [][]int, ch_buttonPressed <-chan bool) constant.STATE {
	flagMasterSlave = constant.MASTER
	ch_sendMasterMatrix := make(chan bool)
	ch_calcElevatorStops := make(chan bool)
	flagOrderMatrixSet := false

	go tickCounterCustom(ch_sendMasterMatrix)
	go tickCounterCustomCalcOrders(ch_calcElevatorStops)

	masterMatrix := InitMasterMatrix(orderMatrix)
	ch_repeatedBcast <- masterMatrix

	for {
		select {
		case newMasterMatrix := <-ch_recieve:
			for elev := int(constant.FIRST_ELEV); elev < len(newMasterMatrix); elev++ {
				if newMasterMatrix[elev][constant.IP] == previousMaster {
					newMasterMatrix = append(newMasterMatrix[:elev], newMasterMatrix[elev+1:]...)
					if flagOrderMatrixSet == false {
						masterMatrix = newMasterMatrix
						flagOrderMatrixSet = true
					}
				}
			}
			if checkMaster(newMasterMatrix) == constant.SLAVE {
				return constant.SLAVE
			}
		case disconnectedIP := <-ch_peerDisconnected:
			masterMatrix = deleteDisconnectedPeer(masterMatrix, disconnectedIP)
			flagDisconnectedPeer = false
		case recievedMatrix := <-ch_recieveSlave:
			masterMatrix = mergeRecievedInfo(masterMatrix, recievedMatrix)
			masterMatrix = mergeUnconfirmedOrders(masterMatrix, recievedMatrix)
			masterMatrix = checkOrderServed(masterMatrix, recievedMatrix)
		case <-ch_calcElevatorStops:
			masterMatrix = clearCurrentOrders(masterMatrix)
			masterMatrix = calculateElevatorStops(masterMatrix)
		case <-ch_sendMasterMatrix:
			sendMasterMatrixToElevator(ch_buttonPressed, ch_recieveLocal, masterMatrix)
			ch_repeatedBcast <- masterMatrix
		default:
		}
	}
}

func sendMasterMatrixToElevator(ch_buttonPressed <-chan bool, ch_recieveLocal chan<- [][]int, masterMatrix [][]int) {
	<-ch_buttonPressed
	ch_recieveLocal <- masterMatrix
}

/* Slave state, checks for alive masters. Transitions if no masters on UDP. */
func stateSlave(ch_recieve <-chan [][]int, ch_repeatedBcast chan<- [][]int, ch_recieveLocal chan<- [][]int, ch_recieveSlaveLocal <-chan [][]int, ch_buttonPressed <-chan bool) (constant.STATE, [][]int) {
	ch_slaveAlone := make(chan bool)
	ch_killTimer := make(chan bool)
	ch_sendMasterMatrix := make(chan bool, 2*constant.N_FLOORS)
	flagSlaveAlone := true
	var orderMatrix [][]int
	for i := 0; i <= 1; i++ {
		orderMatrix = append(orderMatrix, make([]int, 5+constant.N_FLOORS))
	}
	go tickCounterCustom(ch_sendMasterMatrix)
	var masterMatrix [][]int

	for {
		select {
		case localMatrix := <-ch_recieveSlaveLocal:
			ch_repeatedBcast <- localMatrix
		case masterMatrix = <-ch_recieve: // Recieves masterMatrix on channel from master over UDP.
			// Update orderMatrix to contain all hallOrders
			orderMatrix[0] = masterMatrix[0]
			orderMatrix[1] = masterMatrix[1]

			for elev := int(constant.FIRST_ELEV); elev < len(masterMatrix); elev++ {
				if masterMatrix[elev][constant.SLAVE_MASTER] == int(constant.MASTER) {
					previousMaster = masterMatrix[elev][constant.IP]
				}
			}
			if flagSlaveAlone == false {
				ch_killTimer <- true
				flagSlaveAlone = true
			}
		case <-ch_sendMasterMatrix:
			if len(masterMatrix) > 0 {
				go sendToChannel(ch_recieveLocal, masterMatrix)
			}
		case <-ch_slaveAlone:
			return constant.MASTER, orderMatrix
		default:
			if flagSlaveAlone == true {
				go slaveTimer(ch_slaveAlone, ch_killTimer)
				flagSlaveAlone = false
			}
		}

	}
}

/* Broadcast masterMatrix on ch_transmit */
func sendToChannel(ch_transmit chan<- [][]int, matrix [][]int) {
	ch_transmit <- matrix
}

/* Timer to check if slave is alone. If true transition to master  */
func slaveTimer(ch_slaveAlone chan<- bool, ch_killTimer <-chan bool) {
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

/* Communicates the master matrix to the elevator, and recieves data of the elevators current state which is broadcast to master over UDP. */
func localOrderHandler(ch_recieveLocal <-chan [][]int, ch_transmitSlave chan<- [][]int, ch_elevRecieve chan<- [][]int, ch_elevTransmit <-chan [][]int, ch_recieveSlaveLocal chan<- [][]int) {
	localMatrix := InitLocalMatrix()
	ch_sendToElevTick := make(chan bool)
	go tickCounterCustom(ch_sendToElevTick)
	ch_recieveSlaveLocal <- localMatrix
	prevMasterMatrix := <-ch_recieveLocal
	for {
		select {
		case localMatrix = <-ch_elevTransmit: // localMatrix FROM elevator
			localMatrix[constant.UP_BUTTON][constant.SLAVE_MASTER] = int(flagMasterSlave)
			localMatrix[constant.UP_BUTTON][constant.IP] = constant.LocalIP
			ch_transmitSlave <- localMatrix
			if flagMasterSlave == constant.SLAVE {
				ch_recieveSlaveLocal <- localMatrix
			}

		case masterMatrix := <-ch_recieveLocal:
			prevMasterMatrix = masterMatrix
		case <-ch_sendToElevTick:
			ch_elevRecieve <- prevMasterMatrix // masterMatrix TO elevator
		}
	}
}

/* Initialize local matrix, 2x(5+N_FLOORS) contains information about local elevator and UP/DOWN hall lights. */
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

/* Check if there are other masters in the recieved matrix. Lowest IP remains master. */
func checkMaster(matrix [][]int) constant.STATE {
	rows := len(matrix)
	for row := int(constant.FIRST_ELEV); row < rows; row++ {
		if matrix[row][constant.SLAVE_MASTER] == int(constant.MASTER) {
			if matrix[row][constant.IP] < constant.LocalIP {
				return constant.SLAVE
			}
		}
	}
	return constant.MASTER
}

/* Initialize master matrix, 3x(5+N_FLOORS) contains information about every elevator and UP/DOWN hall lights. */
func InitMasterMatrix(orderMatrix [][]int) [][]int {
	masterMatrix := make([][]int, 0)
	for i := 0; i <= 2; i++ {
		masterMatrix = append(masterMatrix, make([]int, 5+constant.N_FLOORS))
	}
	if len(orderMatrix) == 2 {
		masterMatrix[0] = orderMatrix[0]
		masterMatrix[1] = orderMatrix[1]
	}
	masterMatrix[constant.FIRST_ELEV][constant.IP] = constant.LocalIP
	masterMatrix[constant.FIRST_ELEV][constant.DIR] = int(elevio.MD_Stop)
	masterMatrix[constant.FIRST_ELEV][constant.FLOOR] = elevio.GetFloorInit()
	masterMatrix[constant.FIRST_ELEV][constant.ELEV_STATE] = int(constant.IDLE)
	masterMatrix[constant.FIRST_ELEV][constant.SLAVE_MASTER] = int(constant.MASTER)
	return masterMatrix
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

/* Check for disconnected peers, pass ID as string over channel */
func checkDisconnectedPeers(ch_peerUpdate <-chan peers.PeerUpdate, ch_peerDisconnected chan<- string) {
	for {
		if flagDisconnectedPeer == false {
			peerUpdate := <-ch_peerUpdate
			peerUpdateLost := peerUpdate.Lost
			if len(peerUpdate.Lost) >= 1 {
				flagDisconnectedPeer = true
				ch_peerDisconnected <- peerUpdateLost[0]
			}
		}
	}
}

/* Delete peer with the corresponding IP */
func deleteDisconnectedPeer(masterMatrix [][]int, disconnectedIP string) [][]int {
	var peer int
	for _, c := range disconnectedIP {
		peer = int(c)
	}
	for row := int(constant.FIRST_ELEV); row < len(masterMatrix); row++ {
		if masterMatrix[row][constant.IP] == peer {
			masterMatrix = append(masterMatrix[:row], masterMatrix[row+1:]...)
		}
	}
	return masterMatrix
}

/* Merge info from recievedMatrix, append if new slave */
func mergeRecievedInfo(masterMatrix [][]int, recievedMatrix [][]int) [][]int {
	slaveIP := recievedMatrix[constant.UP_BUTTON][constant.IP]
	flagSlaveExist := false
	for row := int(constant.FIRST_ELEV); row < len(masterMatrix); row++ {
		if masterMatrix[row][constant.IP] == slaveIP {
			masterMatrix[row][constant.DIR] = recievedMatrix[constant.UP_BUTTON][constant.DIR]
			masterMatrix[row][constant.FLOOR] = recievedMatrix[constant.UP_BUTTON][constant.FLOOR]
			masterMatrix[row][constant.ELEV_STATE] = recievedMatrix[constant.UP_BUTTON][constant.ELEV_STATE]
			flagSlaveExist = true
		}
	}
	if flagSlaveExist == false {
		newSlave := make([]int, constant.FIRST_FLOOR+constant.N_FLOORS)
		copy(newSlave[0:constant.SLAVE_MASTER+1], recievedMatrix[constant.UP_BUTTON][0:constant.SLAVE_MASTER+1]) // Copy not inclusive for last index
		masterMatrix = append(masterMatrix, newSlave)
	}
	return masterMatrix
}

/* Removes served orders in the current floor of recievedMatrix */
func checkOrderServed(masterMatrix [][]int, recievedMatrix [][]int) [][]int {
	currentFloor := recievedMatrix[constant.UP_BUTTON][constant.FLOOR]
	if checkStoppedOrDoorsOpen(recievedMatrix) == true {
		masterMatrix[constant.UP_BUTTON][int(constant.FIRST_FLOOR)+currentFloor] = 0
		masterMatrix[constant.DOWN_BUTTON][int(constant.FIRST_FLOOR)+currentFloor] = 0
	}
	return masterMatrix
}

/* If COMMENT MORE, recieved matrix?? */
func checkStoppedOrDoorsOpen(recievedMatrix [][]int) bool {
	if recievedMatrix[constant.UP_BUTTON][constant.ELEV_STATE] == int(constant.STOP) {
		return true
	}
	if recievedMatrix[constant.UP_BUTTON][constant.ELEV_STATE] == int(constant.DOORS_OPEN) {
		return true
	}
	return false
}

/* Insert unconfirmed orders UP/DOWN into masterMatrix */
func mergeUnconfirmedOrders(masterMatrix [][]int, recievedMatrix [][]int) [][]int {
	for row := constant.UP_BUTTON; row <= constant.DOWN_BUTTON; row++ {
		for col := constant.FIRST_FLOOR; col < (constant.N_FLOORS + constant.FIRST_FLOOR); col++ {
			if recievedMatrix[row][col] == 1 {
				masterMatrix[row][col] = 1
			}
		}
	}
	return masterMatrix
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
	rowLength := len(matrix[0])
	colLength := len(matrix)
	var currentFloor int
	for floor := int(constant.FIRST_FLOOR); floor < rowLength; floor++ {
		currentFloor = floor - int(constant.FIRST_FLOOR)
		flagOrderSet = false
		//Check if an order need to get served
		if matrix[constant.UP_BUTTON][floor] == 1 || matrix[constant.DOWN_BUTTON][floor] == 1 {
			//Check if an elevator is already in the desired floor
			for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
				if matrix[elev][constant.FLOOR] == currentFloor && (matrix[elev][constant.ELEV_STATE] != int(constant.MOVE)) {
					matrix[elev][floor] = 1
					flagOrderSet = true
					break
				}
			}
			//Both up and down order
			if flagOrderSet == false && matrix[constant.UP_BUTTON][floor] == 1 && matrix[constant.DOWN_BUTTON][floor] == 1 {
				//Index is used to iterate both up and down from the ordered floor.
				index := 1
			labelOuterLoop1:
				for {
					for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
						//Check if elevator under desired floor with motordirection up
						if flagOrderSet == false &&
							(matrix[elev][constant.FLOOR] == (currentFloor - index)) &&
							(matrix[elev][constant.DIR] == int(elevio.MD_Up) || matrix[elev][constant.ELEV_STATE] == int(constant.IDLE)) &&
							(floor-index >= int(constant.FIRST_FLOOR)) {
							matrix = deleteOtherOrdersOnFloor(matrix, elev, floor)
							matrix[elev][floor] = 1
							flagOrderSet = true
							break labelOuterLoop1
						}
						//Check if elevator above desired floor with motordirection down
						if flagOrderSet == false &&
							(matrix[elev][constant.FLOOR] == (currentFloor + index)) &&
							(matrix[elev][constant.DIR] == int(elevio.MD_Down) || matrix[elev][constant.ELEV_STATE] == int(constant.IDLE)) &&
							(floor+index <= int(constant.FIRST_FLOOR)+constant.N_FLOORS) {
							matrix = deleteOtherOrdersOnFloor(matrix, elev, floor)
							matrix[elev][floor] = 1
							flagOrderSet = true
							break labelOuterLoop1
						}
					}
					//Increase the search radius
					index++
					// If the order is given or exceeded the bounds without given order -> break
					if flagOrderSet == true || ((floor-index) < int(constant.FIRST_FLOOR)) && (floor+index > (int(constant.FIRST_FLOOR)+constant.N_FLOORS)) {
						break
					}
				}
				// --------------------------------------------------------------------
				// Only up order
			} else if flagOrderSet == false && matrix[constant.UP_BUTTON][floor] == 1 {
				index := 1
			labelOuterLoop2:
				for {
					for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
						//Check if elevator under desired floor with motordirection up
						if flagOrderSet == false &&
							(matrix[elev][constant.FLOOR] == (currentFloor - index)) &&
							(matrix[elev][constant.DIR] == int(elevio.MD_Up) || matrix[elev][constant.ELEV_STATE] == int(constant.IDLE)) &&
							(floor-index >= int(constant.FIRST_FLOOR)) {
							matrix = deleteOtherOrdersOnFloor(matrix, elev, floor)
							matrix[elev][floor] = 1
							flagOrderSet = true
							break labelOuterLoop2
						}
						//Check if elevator above desired floor with motordirection down
						if flagOrderSet == false &&
							(matrix[elev][constant.FLOOR] == (currentFloor + index)) &&
							(matrix[elev][constant.ELEV_STATE] == int(constant.IDLE) || matrix[elev][constant.DIR] == int(elevio.MD_Down)) &&
							(floor+index <= int(constant.FIRST_FLOOR)+constant.N_FLOORS) {
							matrix = deleteOtherOrdersOnFloor(matrix, elev, floor)
							matrix[elev][floor] = 1
							flagOrderSet = true
							break labelOuterLoop2
						}
					}
					//Increase the search radius
					index++
					// If the order is given or exceeded the bounds without given order -> break
					if flagOrderSet == true || ((floor-index) < int(constant.FIRST_FLOOR)) && (floor+index > (int(constant.FIRST_FLOOR)+constant.N_FLOORS)) {
						break
					}
				}
				// --------------------------------------------------------------------
				//Only down order
			} else if flagOrderSet == false && matrix[constant.DOWN_BUTTON][floor] == 1 {
				index := 1
			labelOuterLoop3:
				for {
					for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
						//Check if elevator above desired floor with motordirection down
						if flagOrderSet == false && (matrix[elev][constant.FLOOR] == (currentFloor + index)) && (matrix[elev][constant.DIR] == int(elevio.MD_Down) || matrix[elev][constant.ELEV_STATE] == int(constant.IDLE)) && (floor+index <= int(constant.FIRST_FLOOR)+constant.N_FLOORS) {
							matrix = deleteOtherOrdersOnFloor(matrix, elev, floor)
							matrix[elev][floor] = 1
							flagOrderSet = true
							break labelOuterLoop3
						}
						//Check if elevator under desired floor with motordirection up
						if flagOrderSet == false && (matrix[elev][constant.FLOOR] == (currentFloor - index)) && (matrix[elev][constant.ELEV_STATE] == int(constant.IDLE) || matrix[elev][constant.DIR] == int(elevio.MD_Up)) && (floor-index) >= int(constant.FIRST_FLOOR) {
							matrix = deleteOtherOrdersOnFloor(matrix, elev, floor)
							matrix[elev][floor] = 1
							flagOrderSet = true
							break labelOuterLoop3
						}
					}
					//Increase the search radius
					index++
					// If the order is given or exceeded the bounds without given order -> break
					if flagOrderSet == true || ((floor-index) < int(constant.FIRST_FLOOR)) && (floor+index > (int(constant.FIRST_FLOOR)+constant.N_FLOORS)) {
						break
					}
				}
			}
		}
	}
	return matrix
}

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
	matrix = <-ch_repeatedBcast
	for {
		select {
		case msg := <-ch_repeatedBcast:
			matrix = msg
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
