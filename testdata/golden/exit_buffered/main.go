package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Print("no newline")
	fmt.Println()
	for i := 0; i < 3; i++ {
		fmt.Printf("line %d\n", i)
	}
	fmt.Print("partial")
	if true {
		os.Exit(3)
	}
	fmt.Println("unreachable")
}
