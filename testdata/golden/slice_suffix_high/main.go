package main

import "fmt"

func show([]int) {
	fmt.Println("entered")
}

func main() {
	s := make([]int, 2, 4)
	i := 3
	fmt.Print("before ")
	show(s[i:])
	fmt.Println("after")
}
