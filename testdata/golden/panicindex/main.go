// Command panicindex reads past the end of a slice. Go panics and exits
// with status 2; the patina binary must do the same.
package main

import "fmt"

func at(xs []int, i int) int {
	return xs[i]
}

func main() {
	xs := []int{1, 2, 3}
	fmt.Println("before")
	fmt.Print("partial line ")
	fmt.Println(at(xs, 5))
	fmt.Println("never printed")
}
