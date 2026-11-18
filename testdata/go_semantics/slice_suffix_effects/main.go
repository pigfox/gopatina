package main

import "fmt"

func low() int {
	fmt.Println("low")
	return 1
}

func show(s []int) {
	fmt.Println(s)
}

func main() {
	s := []int{10, 20}
	show(s[low():])
}
