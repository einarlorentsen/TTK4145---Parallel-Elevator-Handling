package main

import (
	// "./elevator/elevio"
	// "./elevator/order_handler"
	"./master_slave_fsm"
	// "./elevator/fsm"

	"fmt"
)

func main(){
	fmt.Println("Main program started...")
	// ch_quit_program := make(chan bool)
	// fsm.Init() // Goto closest Floor

	master_slave_fsm.InitMasterSlave()
	// <-ch_quit_program
}
