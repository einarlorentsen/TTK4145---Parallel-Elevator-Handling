
/* Order distribution algorithm */
func calculateElevatorStops(matrix [][]int) [][]int {
	// fmt.Println("Recieved: ", matrix)
	// // fmt.Println("calculateElevatorStops: Calculate stops")
	var flagOrderSet bool
	rowLength := len(matrix[0])
	colLength := len(matrix)
	var currentFloor int

	for floor := int(constant.FIRST_FLOOR); floor < rowLength; floor++ {
		currentFloor = floor - int(constant.FIRST_FLOOR)
		// fmt.Println("Floor: ", floor)
		flagOrderSet = false

		//SJEKKER OM VI HAR EN BESTILLING SOM MÅ DELEGERES
		if matrix[constant.UP_BUTTON][floor] == 1 || matrix[constant.DOWN_BUTTON][floor] == 1 {

			//Sjekker om jeg har en heis i etasjen
			for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
				if matrix[elev][constant.FLOOR] == floor && (matrix[elev][constant.ELEV_STATE] != int(constant.MOVE)) {
					matrix[elev][floor] = 1 // Stop here
					flagOrderSet = true
					break
				}
			}

			//For både opp og ned bestilling
			if flagOrderSet == false && matrix[constant.UP_BUTTON][floor] == 1 && matrix[constant.DOWN_BUTTON][floor] == 1 {
				index := 1
				for {
					for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
						fmt.Println("index up and down: ", index)
						//Sjekker under meg, som har retning opp innenfor grense
						if flagOrderSet == false &&
							(matrix[elev][constant.FLOOR] == (currentFloor - index)) &&
							(matrix[elev][constant.DIR] == int(elevio.MD_Up) || matrix[elev][constant.ELEV_STATE] == int(constant.IDLE)) &&
							(floor-index >= int(constant.FIRST_FLOOR)) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
						//Sjekk over meg, som har retning ned og innenfor grensa
						if flagOrderSet == false &&
							(matrix[elev][constant.FLOOR] == (currentFloor + index)) &&
							(matrix[elev][constant.DIR] == int(elevio.MD_Down) || matrix[elev][constant.ELEV_STATE] == int(constant.IDLE)) &&
							(floor+index <= int(constant.FIRST_FLOOR)+constant.N_FLOORS) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
					}
					//Gått igjennom alle heisene
					index++
					//Hvis ordre gitt eller utenfor bounds UTEN å ha funnet kandidat
					if flagOrderSet == true || ((floor-index) < int(constant.FIRST_FLOOR)) && (floor+index > (int(constant.FIRST_FLOOR)+constant.N_FLOORS)) {
						break
					}
				}
				// --------------------------------------------------------------------
				// For OPP bestilling
			} else if flagOrderSet == false && matrix[constant.UP_BUTTON][floor] == 1 {
				index := 1
				for {
					for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
						//Sjekker under meg, som har retning opp innenfor grense
						if flagOrderSet == false &&
							(matrix[elev][constant.FLOOR] == (currentFloor - index)) &&
							(matrix[elev][constant.DIR] == int(elevio.MD_Up) || matrix[elev][constant.ELEV_STATE] == int(constant.IDLE)) &&
							(floor-index >= int(constant.FIRST_FLOOR)) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
						//Sjekk over meg, som har retning ned og innenfor grensa ///HEEEER!!!! Eller rening NED. Hva med state?
						if flagOrderSet == false &&
							(matrix[elev][constant.FLOOR] == (currentFloor + index)) &&
							(matrix[elev][constant.ELEV_STATE] == int(constant.IDLE)) &&
							(floor+index <= int(constant.FIRST_FLOOR)+constant.N_FLOORS) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
					}
					//Gått igjennom alle heisene
					index++
					//Hvis ordre gitt eller utenfor bounds UTEN å ha funnet kandidat
					if flagOrderSet == true || ((floor-index) < int(constant.FIRST_FLOOR)) && (floor+index > (int(constant.FIRST_FLOOR)+constant.N_FLOORS)) {
						break
					}
				}
				// --------------------------------------------------------------------
				//For bestilling NED
			} else if flagOrderSet == false && matrix[constant.DOWN_BUTTON][floor] == 1 {
				index := 1
				for {
					for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
						//Sjekk over meg, som har retning ned og innenfor grensa
						if flagOrderSet == false && (matrix[elev][constant.FLOOR] == (currentFloor + index)) && (matrix[elev][constant.DIR] == int(elevio.MD_Down) || matrix[elev][constant.ELEV_STATE] == int(constant.IDLE)) && (floor+index <= int(constant.FIRST_FLOOR)+constant.N_FLOORS) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
						//Sjekker under meg, som har retning opp innenfor grense
						if flagOrderSet == false && (matrix[elev][constant.FLOOR] == (currentFloor - index)) && (matrix[elev][constant.ELEV_STATE] == int(constant.IDLE)) && (floor-index) >= int(constant.FIRST_FLOOR) {
							matrix[elev][floor] = 1
							flagOrderSet = true
							break
						}
					}
					//Gått igjennom alle heisene
					index++
					//Hvis ordre gitt eller utenfor bounds UTEN å ha funnet kandidat
					if flagOrderSet == true || ((floor-index) < int(constant.FIRST_FLOOR)) && (floor+index > (int(constant.FIRST_FLOOR)+constant.N_FLOORS)) {
						break
					}
				}
			}

			// Give to master if no elevator has gotten the order
			// if flagOrderSet == false {
			// 	for elev := int(constant.FIRST_ELEV); elev < colLength; elev++ {
			// 		if matrix[elev][constant.SLAVE_MASTER] == int(constant.MASTER) {
			// 			matrix[elev][floor] = 1
			// 		}
			// 	}
			// }

		} // End order condition
	} // End inf loop
	// // fmt.Println("calculateElevatorStops: Orders calculated.")
	// fmt.Println("Calculated Stops: ", matrix)
	return matrix
} // End floor loop
