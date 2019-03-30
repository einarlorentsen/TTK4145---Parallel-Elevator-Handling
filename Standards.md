For å sette en spesifikk port:
go build -ldflags "-X main.elevatorPort=15659" -o elev1
./elev1
go run main.go --> Defaulter til port 15657 for heis.




Funksjonsnavn:
 - Kalles utenifra modulen: Stor forbokstav
 - 

Pseudo-enum/const struct:
 - Stor forbokstav, splitt med _ for mellomrom



BACKUP FILSTRUKTUR
N = N_FLOORS (default: 4)
Cab order:	[1	2	3	...	N-1	N]
