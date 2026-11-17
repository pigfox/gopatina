package main

import "fmt"

func at(s []int, i int) int {
	return s[i]
}

func main() {
	s := []int{1, 2, 3}
	fmt.Println("before")
	for i := 0; i < 5; i++ {
		fmt.Println(at(s, i))
	}
	fmt.Println("never")
}
