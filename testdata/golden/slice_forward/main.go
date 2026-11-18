package main

import "fmt"

func forward(s []int) {
	forwardAgain(s)
}

func forwardAgain(s []int) {
	write(s)
}

func write(s []int) {
	s[0] = 9
}

func main() {
	s := []int{1, 2}
	forward(s)
	fmt.Println(s, len(s))
}
