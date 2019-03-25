package constant

const N_FLOORS = 4

/* Enumeration STATE */
type STATE int

const (
	// Master/slave states
	SLAVE  STATE = 0
	MASTER STATE = 1

	// Elevator states
	INIT         STATE = 0
	IDLE         STATE = 1
	MOVE         STATE = 2
	STOP         STATE = 3
	DOORS_OPEN   STATE = 4
	DOORS_CLOSED STATE = 5
)

/* Indices to masterMatrix */
/* | IP | DIR | FLOOR | ELEV_STATE | Slave/Master | Stop1 | .. | Stop N | */
type FIELD int

const (
	IP           FIELD = 0
	DIR          FIELD = 1
	FLOOR        FIELD = 2
	ELEV_STATE   FIELD = 3
	SLAVE_MASTER FIELD = 4

	FIRST_FLOOR FIELD = 5
	FIRST_ELEV  FIELD = 2

	UP_BUTTON   FIELD = 0
	DOWN_BUTTON FIELD = 1
	CAB         FIELD = 2
)

const UPDATE_INTERVAL = 250 // Tick time in milliseconds
const BACKUP_FILENAME = "backup.txt"
const PORT_bcast = 16309      //16569
const PORT_slaveBcast = 14152 //16570
const PORT_peers = 14150      //15647
