package master_slave_fsm

import(
	"../network/bcast"
    "../network/localip"
    "../file_IO"
)

/* Enumeration STATE */
type STATE int
const (
  SLAVE STATE = 0
  MASTER STATE = 1
)

/* Tick time in milliseconds */
const UPDATE_INTERVAL := 250

/* Backup filename */
const BACKUP_FILENAME := "backup.txt"

func Init(){
	var IP string
    ch_updateInterval := make(chan int)
    var matrixSlave [][]int
    var matrixMaster [][]int
    const PORT_bcast := 16569

	localIP, err := localip.LocalIP()
	if err != nil {
		fmt.Println(err)
		localIP = "DISCONNECTED"
	}
	// Set IP to last IP address field
    IP = getIdentifierIP(localIP)

    // FROM UDP MODULE

    // We make channels for sending and receiving our custom data types
	ch_transmit := make(chan [][]int)
	ch_recieve := make(chan [][]int)

	go bcast.Transmitter(PORT_bcast, ch_transmit)
	go bcast.Receiver(PORT_bcast, ch_recieve)

    // Start the update_interval ticker.
    tickCounter(ch_updateInterval)

    // CHECK FOR BACKUP FILE, CAB ORDERS
    cabOrders := file_IO.ReadFile(BACKUP_FILENAME)
    if len(cabOrders) == 0 {
        fmt.Println("No backups found.")
    } else {
        fmt.Println("Backup found.")
    }

}





// init UDP
// Slave state



/* Slave state function */
func stateSlave()



/*  Converts the IP to an int. Example:
    "10.100.23.253" -> 253 */
func getIdentifierIP(IP string) ([]int){
    IP_length := len(IP)
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

/* PLACEHOLDER TITLE */
func stateChange(currentState STATE, initValue int){
    switch currentState {
    case MASTER:
      stateMaster()
    case SLAVE:
      stateSlave()
    }
}
