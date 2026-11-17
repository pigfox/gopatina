package main

import "fmt"

func main() {
	s := []int{1, 2, 3}
	i := 2
	for ; i >= -1; i-- {
		fmt.Println(s[i])
	}
}
