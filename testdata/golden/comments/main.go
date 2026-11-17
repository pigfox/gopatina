// Package comment
package main

import "fmt" // trailing import comment

/* block comment
   spanning lines */

// Point is a point.
type Point struct {
	X, Y int // coords
	// between fields
	Label string /* inline */
}

// Add adds.
func (p Point) Add(o Point) Point { // after sig
	// first line in body
	return Point{ /* in literal */ p.X + o.X, p.Y + o.Y, p.Label + o.Label}
}

func main() {
	// leading
	p := Point{1, 2, "a"} // trailing
	q := Point{
		X: 3, // three
		Y: 4,
		// no label
	}
	/* before stmt */ r := p.Add(q)
	if r.X > 2 { // cond
		// inside if
		fmt.Println("big" /* arg */, r.X)
	} else /* else comment */ {
		fmt.Println("small")
	}
	for i := 0; i < 2; i++ { // loop
		// body only comment
	}
	switch r.X {
	// comment before case
	case 4: // case line
		fmt.Println("four")
	}
	var x = /* mid */ 5
	fmt.Println(x, r.Label)
	// last comment
}
