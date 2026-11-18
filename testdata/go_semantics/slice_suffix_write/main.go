package main

import "fmt"

func forward(s []int) { write(s) }

func write(s []int) { s[0] = 99 }

func main() {
	s := []int{10, 20, 30}
	forward(s[1:])
	fmt.Println(s)
}
