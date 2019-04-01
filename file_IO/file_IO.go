package file_IO

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

/* StringToNumbers, used:
https://stackoverflow.com/questions/43599253/read-space-separated-integers-from-stdin-into-int-slice
Separates a string into a vector of integers.*/
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

/* ReadFile: Returns 2D slice of type [][]int */
func ReadFile(filename string) [][]int {
	matrix := make([][]int, 0)
	matrixRow := make([]int, 0)
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("No file found!")
		return matrix
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		matrixRow = StringToNumbers(scanner.Text())
		matrix = append(matrix, matrixRow)
	}
	return matrix
}

/* WriteFile: Writes 2D slice of type [][]int to file as strings. */
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
