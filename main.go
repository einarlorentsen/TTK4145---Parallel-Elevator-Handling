package main

import (
	// "./elevator/elevio"
	// "./elevator/order_handler"

	"./master_slave_fsm"

	"fmt"
)

var elevatorPort string = "15657"

func main() {
	// constant.LocalIP = os.Getpid()

	fmt.Println("Main program started...")
	elevatorAddress := "localhost:" + elevatorPort
	master_slave_fsm.ConnectToElevator(elevatorAddress)

	ch_quit_program := make(chan bool)

	// fsm.InitFSM()

	go master_slave_fsm.InitMasterSlave()

	<-ch_quit_program
}
