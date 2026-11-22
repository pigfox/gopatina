// gc makes every call of a return of several values first, then reads
// the changed package-level variables beside them (RFC 0010).
package main

import "fmt"

var g = 1

func f() int {
	g = 10
	return 2
}

func two(k bool) (int, int) {
	if k {
		return g, f()
	}
	g = 1
	return g, f()
}

func three() (int, int, int) {
	return g + 1, f(), g
}

func main() {
	a, b := two(true)
	fmt.Println(a, b)
	g = 1
	a, b = two(false)
	fmt.Println(a, b)
	g = 1
	x, y, z := three()
	fmt.Println(x, y, z)
	g = 1
	c, d := g, f()
	fmt.Println(c, d)
}
