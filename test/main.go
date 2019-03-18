package main

import(
	"fmt"
	//"./file_IO"
)

func main(){


	matrix := [][]int{
		{0,0,0,0,0},
		{0,0,0,0,0},
		{252,1,0,0,0},
		{253,0,0,0,1},

	}
	rows := len(matrix)
	for index:= 0 ; index < rows ; index++ {
		if matrix[index][4] == 1 {
			fmt.Println("One at row index", index)
		}
	}
	fmt.Println(len(matrix[0]))
	fmt.Println(len(matrix))

	/*for index, value := range matrix[0][4] {
		if value == 1 {
			fmt.Println("One at row index", index)		
		}
	}*/
	//fmt.Println(matrix)


	
}
