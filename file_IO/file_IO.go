package file_IO

import (
	// "io/ioutil"
	// "bytes"
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

/* Error check */
func check(err error) {
	if err != nil {
		panic(err)
	}
}

/* For stringToNumbers, used:
https://stackoverflow.com/questions/43599253/
read-space-separated-integers-from-stdin-into-int-slice
Separates a string into a vector of integers.
*/
func StringToNumbers(str string) []int {
	var arr []int
	for _, f := range strings.Fields(str) {
		integer, err := strconv.Atoi(f)
		if err == nil {
			arr = append(arr, integer)
		}
	}
	return arr
}

func ReadFile(filename string) [][]int {
	matrix := make([][]int, 0)  // Init empty 2D slice
	matrixRow := make([]int, 0) // Init empty 1D slice

	file, err := os.Open(filename) // Read backupfile
	if err != nil {
		return matrix // Return empty array if no file
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() { // Create 2D slice
		matrixRow = StringToNumbers(scanner.Text())
		matrix = append(matrix, matrixRow) // Append row to 2D slice
	}
	return matrix
}

func WriteFile(filename string, matrix [][]int) {
	file, err := os.Create(filename)
	if err != nil {
		fmt.Println("Failed writing ", filename)
		return
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	defer writer.Flush()

	for i := 0; i < len(matrix); i++ {
		for j := 0; j < len(matrix[i]); j++ {
			fmt.Fprint(writer, matrix[i][j], " ")
		}
		fmt.Fprintln(writer)
	}
}

// How to: MATRICES
// package main
//
// import (
// 	"fmt"
// )
//
// func main() {
//
// 	arr := [][]int{ {2,4}, {6,8}, }
// 	fmt.Println(arr)
// 	for i := 0; i < len(arr); i++{
// 		for j := 0; j < len(arr[i]); j++{
// 			fmt.Printf("%d ", arr[i][j])
// 		}
// 		fmt.Printf("\n")
// 	}
// }

// WORKING APPEND-CODE
//package main
//import (
//	"fmt"
//)
//func main() {
//	matrix := make([][]int, 0)
//	nRows := 6
//	for row := 0; row < nRows; row++ {
//	fmt.Println(row)
//		varRow := []int{row+1,row+2,row+3,row+4}
//		matrix = append(matrix, varRow)
//	}
//
//	for i := 0; i < len(matrix); i++ {
//		fmt.Println(matrix[i])
//	}
//}
