
'
'
/* Communicates the master matrix to the elevator, and recieves data of the
		elevators current state which is broadcast to master over UDP. */
func localOrderHandler(ch_recieve <-chan [][]int, ch_transmitSlave chan<- [][]int){
	// Channels to communicate with elevator
	ch_elevTransmit := make(chan [][]int)
	ch_elevRecieve := make(chan [][]int)

	for {
		select {
		case masterMatrix := <- ch_recieve:
			ch_elevTransmit <- masterMatrix
		case localMatrix := ch_elevTransmit
			localMatrix[UP_BUTTON][SLAVE_MASTER] = flagMasterSlave	// Ensure correct state
			localMatrix[UP_BUTTON][IP] = localIP										// Ensure correct IP
			ch_transmitSlave <- localMatrix
		}
	}
}


/* Initialize local matrix */
func initLocalMatrix() [][]int {
	localMatrix := make([][]int, 0)
	for i := 0; i <= 1; i++ {
		localMatrix = append(matrixMaster, make([]int, 5+N_FLOORS))
	}
	return localMatrix
}
