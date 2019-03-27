

package main

import (
	"fmt"
)

func main() {
	ch1 := make(chan int)
	ch2 := make(chan int)
	go a(ch1)
	go b(ch2)
	i1 := 0
	checkFOR:
	for {
	select {
		case i1 = <- ch1:
		fmt.Println(i1)
		
		case <- ch2:
		break checkFOR // Break loop marked checkFOR
	}
	
	}
	fmt.Println("Hello, playground")
}



func a(ch1 chan<- int){
	a := 0
	for i := 0; i < 10000000; i++ {
		a++
	}
	ch1 <- a
}

func b(ch2 chan<- int){
	b := 0
	for i := 0; i < 100000; i++ {
		b++
	}
	ch2 <- b
}
