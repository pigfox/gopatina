package main

import "fmt"

func f(a, b int) int { return a / b }

func main() {
	fmt.Println(f(6, 3))
	n := 0
	fmt.Println(f(1, n))
}
