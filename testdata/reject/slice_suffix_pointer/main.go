package main

import "fmt"

func read(s []int) {
	fmt.Println(s)
}

func main() {
	s := []int{1, 2, 3}
	p := &s
	read(s[1:])
	*p = []int{4, 5, 6}
}
