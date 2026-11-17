package main

import "fmt"

func main() {
	for i := 0; i < 3; i++ {
		switch i {
		case 2:
			if i == 2 {
				break
			}
			fmt.Println("after break", i)
		default:
			fmt.Println("default", i)
		}
	}
}
