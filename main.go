package main

import (
	// "./elevator/elevio"
	// "./elevator/order_handler"
	"./constant"
	"./elevator"
	"./elevator/elevio"
	"./master_slave_fsm"

	"./elevator/fsm"

	"fmt"
)
// default port 15657
func main() {
	fmt.Println("Main program started...")
	elevio.Init("localhost:15657", constant.N_FLOORS) // Init elevatorServer

	ch_elevTransmit := make(chan [][]int, 2*constant.N_FLOORS) // Elevator transmission, FROM elevator
	ch_elevRecieve := make(chan [][]int, 2*constant.N_FLOORS)  // Elevator reciever,	TO elevator
	ch_buttonPressed := make(chan bool)

	ch_quit_program := make(chan bool)

	fsm.InitFSM()

	master_slave_fsm.SetLocalIP() // Set the global variable localIP.
	go elevator.InitElevator(ch_elevTransmit, ch_elevRecieve, ch_buttonPressed)
	go master_slave_fsm.InitMasterSlave(ch_elevTransmit, ch_elevRecieve, ch_buttonPressed)

	<-ch_quit_program
}
