// Command range_changed both appends to and changes the slice it ranges
// over: what the loop sees depends on the slice's spare capacity.
package main

import "fmt"

func main() {
	xs := make([]int, 2, 10)
	for i, x := range xs {
		xs = append(xs, x)
		xs[i] = 7
	}
	fmt.Println(xs)
}
