package main

import "fmt"

func main() {
	s := []int{1, 2, 3, 4}
	for i, v := range s {
		s[len(s)-1-i] = v * 10
	}
	fmt.Println(s)
}
