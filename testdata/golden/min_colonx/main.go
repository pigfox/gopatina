package main

import "fmt"

func main() {
	s := "abc"
	for i := 0; i < len(s); i++ {
		fmt.Printf("%d:%x ", i, s[i])
	}
	fmt.Println()
	fmt.Printf("%x %d\n", s[1], s[0])
}
