package user_interface

import(
	"./elevio"
	"../file_IO"
)


/* Set lights */

/* Current floor light */
func CurrentFloorLight(floor int){
	if (floor != -1){
		elevio.SetFloorIndicator(floor)
	}
}


/*  */


/* */
func InitElevator(){


	cabOrders := file_IO.ReadFile(BACKUP_FILENAME) // Matrix
	if len(cabOrders) == 0 {
		fmt.Println("No backups found.")
	} else {
		fmt.Println("Backup found.")
		initialCabOrders = cabOrders[0]
	}



}
