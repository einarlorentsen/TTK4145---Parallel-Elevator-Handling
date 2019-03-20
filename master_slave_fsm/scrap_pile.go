/* Order distribution algorithm */
func calculateElevatorStops(matrix [][]int) [][]int {
	var flagOrderSet bool
	var aboveFloor int
	var belowFloor int
	rowLength := len(matrix[UP_BUTTON])
	colLength := len(matrix)

	// Check if both buttons are set



	for floor := int(FIRST_FLOOR); floor < rowLength; floor++ {
		for index := 1; index < N_FLOORS; index++ {
			for elev := int(FIRST_ELEV); elev < colLength; elev++ {
				aboveFloor = floor + index
				belowFloor = floor - index

				// Check above floors
				if aboveFloor < int(FIRST_FLOOR)+N_FLOORS {
					if matrix[UP_BUTTON][floor] = 1 && matrix[DOWN_BUTTON][floor] == 1 {
						if matrix[elev][FLOOR] == aboveFloor && matrix[elev][DIR] == elevio.MD_Down {	// Order above elevator
							flagOrderSet = true
							break
						}
					}
				}


				// Check below floors
				if belowFloor < int(FIRST_FLOOR) {
					if matrix[UP_BUTTON][floor] = 1 && matrix[DOWN_BUTTON][floor] == 1 {
						if matrix[elev][FLOOR] == aboveFloor && matrix[elev][DIR] == elevio.MD_Down {	// Order above elevator
							flagOrderSet = true
							break
						}
					}
				}


			}
			if flagOrderSet == true {
				break
			}
		}
	}


	// Check up button is set






	// Check down button is set











	//Check if we are in the same floor as an order, and can take it
	for floor := int(FIRST_FLOOR); floor < rowLength; floor++ {
		flagOrderSet = false
		// Assumes elevator stops if any order at Floor
		for elev := int(FIRST_ELEV); elev < colLength; elev++ {
			// If in floor, give order if elevator is idle, stopped or has doors open
			if (matrix[elev][FLOOR] == floor && (matrix[elev][STATE] == fsm.IDLE ||
				matrix[elev][STATE] == fsm.STOP || matrix[elev][STATE] == fsm.OPEN_DOORS )) {
					matrix[elev][floor] = 1	// Stop here
					flagOrderSet = true
					break
			}
		}
	}

	//Iterate both up and down from the floor that has ordered and elevator and find the
	//closest elevator
	for index := 1 ; index < N_FLOORS ; index++ {
		for floor := int(FIRST_ELEV) ; elev < rowLength ; floor++{

		}
	}



	//
	// 	if flagOrderSet == false && matrix[elev][UP_BUTTON] == 1 && matrix[elev][DOWN_BUTTON]{
	// 		for index := 1 ; index < N_FLOORS ; index ++{
	// 			for elev := int(FIRST_ELEV); elev < colLength; elev++ {
	// 				// Both direction buttons set
	// 				aboveFloor := floor + index
	// 				belowFloor := floor - index
	//
	// 				// UP button set
	//
	// 				// Down button set
	//
	// 			}
	// 		}
	// 	}







		// if (matrixMaster[UP_BUTTON][floor] || matrixMaster[UP_BUTTON][floor]) {
		// 	for elev := int(FIRST_ELEV); elev < colLength; elev++ {
		// 		// If in floor, give order if elevator is idle, stopped or has doors open
		// 		if (matrixMaster[elev][FLOOR] == floor && (matrixMaster[elev][STATE] == fsm.IDLE ||
		// 			matrixMaster[elev][STATE] == fsm.STOP || matrixMaster[elev][STATE] == fsm.OPEN_DOORS )) {
		// 				matrixMaster[elev][floor] = 1	// Stop here
		// 		}
		// 	}
		// 	for index := 1 ; index < N_FLOORS ; index++{
		// 		for elev := int(FIRST_ELEV); elev < colLength; elev++ {
		// 			aboveFloor := floor + index
		// 			belowFloor := floor - index
		// 			if (matrixMaster[elev][FLOOR] == aboveFloor && matrixMaster[elev][FLOOR] == elevio.MD_DOWN){
		// 				//Give order
		// 			}
		// 			else (matrixMaster[elev][FLOOR] == belowFloor && matrixMaster[elev][FLOOR] == elevio.MD_UP)
		// 		}
		// 	}


				// else iterate: floor+1
				// check for elevators down to floor or idle
				// else iterate: floor-1
				// check for elevators up to floor or idle
		}
	}
}
