package main

import "fmt"

func show([]int) {
	fmt.Println("entered")
}

func main() {
	s := []int{10}
	i := -1
	fmt.Println("before")
	show(s[i:])
	fmt.Println("after")
}
