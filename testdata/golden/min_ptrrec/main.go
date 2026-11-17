package main

import "fmt"

func hanoi(n int, moves *int) {
	if n == 0 {
		return
	}
	hanoi(n-1, moves)
	*moves++
	hanoi(n-1, moves)
}

func main() {
	m := 0
	hanoi(3, &m)
	fmt.Println(m)
}
