// Command elem_print prints an element of a slice in the same call that
// appends to the slice.
package main

import "fmt"

func grow(xs *[]int) int {
	*xs = append(*xs, 1)
	return len(*xs)
}

func main() {
	xs := []int{7}
	fmt.Println(xs[0], grow(&xs))
}
