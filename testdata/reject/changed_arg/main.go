// Command changed_arg passes a struct to change, and computes a later
// argument by changing the same struct.
package main

import "fmt"

type Box struct{ N int }

func add(b *Box, k int) { b.N += k }

func next(b *Box) int {
	b.N++
	return b.N
}

func main() {
	b := Box{}
	add(&b, next(&b))
	fmt.Println(b.N)
}
