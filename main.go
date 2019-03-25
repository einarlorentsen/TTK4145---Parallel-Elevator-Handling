package main

import (
	// "./elevator/elevio"
	// "./elevator/order_handler"
	"./constant"
	"./elevator"
	"./elevator/elevio"
	"./master_slave_fsm"

	// "./elevator/fsm"

	"fmt"
)

func main() {
	fmt.Println("Main program started...")

	ch_elevTransmit := make(chan [][]int) // Elevator transmission, FROM elevator
	ch_elevRecieve := make(chan [][]int)  // Elevator reciever,	TO elevator

	ch_quit_program := make(chan bool)

	// fsm.Init() // Goto closest Floor
	elevio.Init("localhost:15657", constant.N_FLOORS) // Init elevatorServer
	master_slave_fsm.SetLocalIP()                     // Set the global variable localIP.
	go elevator.InitElevator(ch_elevTransmit, ch_elevRecieve)
	go master_slave_fsm.InitMasterSlave(ch_elevTransmit, ch_elevRecieve)

	<-ch_quit_program
}
