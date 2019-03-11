package user_interface

import(
	"./elevio"
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
func
