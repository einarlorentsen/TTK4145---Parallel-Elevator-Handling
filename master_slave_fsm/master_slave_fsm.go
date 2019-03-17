package master_slave_fsm

import(
	"../network/bcast"
    "../network/localip"
    "../file_IO"
	"../elevator/elevio"
	"sync"
)

/* Enumeration STATE */
type STATE int
const (
	SLAVE 	STATE = 0
	MASTER 	STATE = 1
)

/* Indices to masterMatrix */
type FIELD int
const (
	IP 				FIELD = 0
	DIR 			FIELD = 1
	FLOOR 			FIELD = 2
	ELEV_STATE 		FIELD = 3
	SLAVE_MASTER 	FIELD = 4

	FIRST_FLOOR 	FIELD = 5
	FIRST_ELEV		FIELD = 3

)
/*           | IP | DIR | FLOOR | ELEV_STATE | Slave/Master | Stop1 | .. | Stop N | */

/* Tick time in milliseconds */
const UPDATE_INTERVAL := 250

/* Backup filename */
const BACKUP_FILENAME := "backup.txt"

func Init(){
	// var IP string
    ch_updateInterval := make(chan int)
    var matrixSlave [][]int
    var matrixMaster [][]int
    const PORT_bcast := 16569
	var initialCabOrders []init	// Vector for cab orders

	// localIP, err := localip.LocalIP()
	// if err != nil {
	// 	fmt.Println(err)
	// 	localIP = "DISCONNECTED"
	// }
	// // Set IP to last IP address field
    // IP = getIdentifierIP(localIP)

    // FROM UDP MODULE
    // We make channels for sending and receiving our custom data types
	ch_transmit := make(chan [][]int)
	ch_recieve := make(chan [][]int)

	go bcast.Transmitter(PORT_bcast, ch_transmit)
	go bcast.Receiver(PORT_bcast, ch_recieve)

    // Start the update_interval ticker.
    go tickCounter(ch_updateInterval)

    // CHECK FOR BACKUP FILE, CAB ORDERS
    cabOrders := file_IO.ReadFile(BACKUP_FILENAME)	// Matrix
    if len(cabOrders) == 0 {
        fmt.Println("No backups found.")
    } else {
        fmt.Println("Backup found.")
		initialCabOrders = cabOrders[0]
    }

	// Start in slave-state
	stateChange(SLAVE, cabOrders)


}





// init UDP
// Slave state

/* PLACEHOLDER TITLE */
func stateChange(currentState STATE, cabOrders []int){
    switch currentState {
    case MASTER:
      stateMaster()
    case SLAVE:
      stateSlave()
    }
}





/* matrixMaster dim: (2+N_ELEVATORS) x (5+N_FLOORS) */
/*           | IP | DIR | FLOOR | ELEV_STATE | Slave/Master | Stop1 | .. | Stop N | */
/* UP lights | x  |  x  |       |      x     |       x      |       | .. |    x   | */
/* DN lights | x  |  x  |       |      x     |       x      |   x   | .. |        | */
/* ELEV 1    |    |     |       |            |              |       | .. |        | */
/* ...       |    |     |       |            |              |       | .. |        | */
/* ELEV N    |    |     |       |            |              |       | .. |        | */
/* Matrix indexing: [ROW][COL] */


func stateMaster(matrixMaster [][]int, cabOrders []int){
	// if matrixMaster == empty
	// Generate for 1 elevator
	// then listen for slaves, goroutine
	localIP := getLocalIP()
	if matrixMaster == nil {
		matrixMaster = initMatrixMaster()
	}

	for {
		recievedMatrix := <- ch_recieve
		if checkMaster(recievedMatrix, localIP) == SLAVE {
			 break
		}

		// Slaves: 50ms pr message
		// Buffer at master, check newest messages
		// Use newest messages.
		// If no message from slave within tick period -> It has lost connection

	}

	// Listen to channel ch_recieve
		// Recieves a message
			// Update elevator fields (not stop)
			// OR the up/down order fields
			// Calculate new order distribution.
	// Update masterMatrix
	// Logic to break master-state if master is alone and recieves from larger master

	stateChange(SLAVE)
}




/* Check if there are other masters in the recieved matrix.
   Lowest IP remains master.
   Return true if remain master, false if transition to slave */
func checkMaster(matrix [][]int, localIP int) (STATE){
	rows := len(matrix)
	for row := 0; row < rows; row++ {
		if matrix[row][SLAVE_MASTER] == MASTER {
			if matrix[row][IP] < localIP {
				return SLAVE	// Transition to slave
			}
		}
	}
	return MASTER 				// Remain master
}

func initMatrixMaster() ([][]int){
	localIP := getLocalIP()
	matrixMaster := make([][]int, 0)
	for i := 0; i <= 2; i++ {			// For 1 elevator
		matrixMaster = append(matrixMaster, make([]int,4+N_FLOORS) )
	}
	matrixMaster[FIRST_ELEV][IP] = localIP
	matrixMaster[FIRST_ELEV][DIR] = elevio.MD_Stop
	matrixMaster[FIRST_ELEV][FLOOR] = elevio.getFloor
	matrixMaster[FIRST_ELEV][ELEV_STATE] = fsm.READY
	matrixMaster[FIRST_ELEV][SLAVE_MASTER] = MASTER

	return matrixMaster
}



/*  Converts the IP to an int. Example:
    "10.100.23.253" -> 253 */
func getLocalIP() ([]int){
	localIP, err := localip.LocalIP()
	if err != nil {
		fmt.Println(err)
		localIP = "DISCONNECTED"
	}

	IP_length := len(localIP)
	for i := IP_length-1; i > 0; i-- {
		if IP[i] == '.' {
			IP = IP[i+1:IP_length]
			break;
		}
	}
	return file_IO.StringToNumbers(IP)[0]  // Vector of 1 element
}

/* Ticks every UPDATE_INTERVAL milliseconds */
func tickCounter(ch_updateInterval chan<- int) {
    ticker := time.NewTicker(UPDATE_INTERVAL*time.Millisecond)
    for t := range ticker.C {
        ch_updateInterval <- 1
        t = t
    }
}
