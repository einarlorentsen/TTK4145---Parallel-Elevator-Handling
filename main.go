package main

const N_FLOORS := 4

// .. To traverse reverse
import(
	// "./elevator/elevio"
	// "./elevator/order_handler"
	"fmt"
	"./master_slave_handler"
	"./elevator/fsm"
)



func main(){
	fsm.Init() // Goto closest Floor
	go master_slave.Init()
	// fmt.Println(master_slave_handler.MASTER)
	// master_slave_handler.Init()
}
