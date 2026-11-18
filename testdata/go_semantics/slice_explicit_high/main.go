package main

import "fmt"

func show(s []int) {
	fmt.Println(s, len(s))
}

func main() {
	s := make([]int, 2, 4)
	s[1] = 7
	show(s[1:4])
}
