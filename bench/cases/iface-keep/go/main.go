// Command iface-keep copies interface values and keeps the copies in a
// slice (bench/README.md): Go copies a pointer to an immutable value;
// GoPatina clones the value, since each copy outlives the loop's next
// change to its source (RFC 0012).
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

func run(rounds, n int) {
	total := 0
	for r := 0; r < rounds; r++ {
		var a Shape = Rect{W: r % 5, H: 3, Name: "a rectangle with a name"}
		var kept []Shape
		for i := 0; i < n; i++ {
			kept = append(kept, a) // a copy kept: cloned
			if i%100 == 0 {
				a = Rect{W: i % 7, H: r % 3, Name: "a rectangle with a name"}
			}
		}
		for _, s := range kept {
			total += s.Area()
		}
	}
	fmt.Println(total)
}

// rounds and size are the benchmark's size; the test makes them small.
var rounds, size = 40, 100000

func main() { run(rounds, size) }
