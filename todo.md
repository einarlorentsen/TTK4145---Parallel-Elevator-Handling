ELEV NEVER RECIEVES NEW MATRIX WHEN MATER/SLAVE IS SLAVE?



FIX:
 - 0: Send matrixMaster to ElevFSM:
  - elevatorHandler not sending?
  - elevatorHandler not recieving masterMatrix.
  - localOrderHandler: Sends 8 pings, then dies?
  - elevatorHandler: Never recieves on ch_elevRecieve

 - 1: Lys i alle heiser
	


 - 2: Master dør, slave tar over. Kjører i IDLE men delegerer ikke ordre. 




Slave sender ut korrekt til localOrderHandler.
localOrderHandler sender ut til ch_elevRecieve korrekt.

Finn ut av hvor det går feil!



Når heiser snakker sammen riktig:
Hvis en heis når en etasje og stopper pga CabOrders,
sjekk så at alle andre ordre i den etasjen fjernes.
