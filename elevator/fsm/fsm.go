package fsm

// Enumeration STATES
type STATE int

const (
	INIT  				STATE = 0
	IDLE	 				STATE = 1
	MOVE 					STATE = 2
	STOP  				STATE = 3
	DOORS_OPEN		STATE = 4
	DOORS_CLOSED	STATE = 5

)


func Init(){
	// Do nothing, nothing at all
}
