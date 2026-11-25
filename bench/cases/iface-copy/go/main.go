// Command iface-copy copies interface values and only reads the copies
// (bench/README.md): Go copies a pointer to an immutable value, and
// GoPatina lends the source instead, since nothing changes it while the
// copy is in use (RFC 0012 with RFC 0017).
package main

import "fmt"

// Shape is a shape with an area.
type Shape interface{ Area() int }

// Rect is a named rectangle.
type Rect struct {
	W, H int
	Name string
}

func (r Rect) Area() int { return r.W*r.H + len(r.Name) }

// Circle is a named circle, its area rounded down.
type Circle struct {
	R    int
	Name string
}

func (c Circle) Area() int { return 3*c.R*c.R + len(c.Name) }

func measure(s Shape) int { return s.Area() }

func run(n int) {
	var a Shape = Rect{W: 2, H: 3, Name: "a rectangle with a name"}
	total := 0
	for i := 0; i < n; i++ {
		b := a // a copy only read: lent
		total += measure(b) + i%3
		if i%1000 == 0 {
			if i%2000 == 0 {
				a = Circle{R: i % 7, Name: "a circle"}
			} else {
				a = Rect{W: i % 5, H: 3, Name: "a rectangle with a name"}
			}
		}
	}
	fmt.Println(total)
}

// size is the benchmark's size; the test makes it small.
var size = 200000000

func main() { run(size) }
