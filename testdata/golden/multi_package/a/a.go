// Package a is reached in part: Unused is outside the subset, and nothing
// calls it.
package a

import (
	"fmt"

	"example.com/multi/z"
)

// Max is a typed constant used by another package.
const Max = 10

// Hits is a package-level variable that main changes and reads.
var Hits int

// A Point has a String method, which fmt calls in every package.
type Point struct{ X, Y int }

func (p Point) String() string { return fmt.Sprintf("(%d, %d)", p.X, p.Y) }

func init() { fmt.Println("init a", z.Tag) }

// New has the same name as b.New.
func New(x, y int) Point { return Point{X: x, Y: y} }

// Sum adds the points.
func Sum(ps []Point) Point {
	var s Point
	for _, p := range ps {
		s.X += p.X
		s.Y += p.Y
	}
	return s
}

// Unused uses a map, which patina mode does not support yet.
func Unused() map[string]int { return map[string]int{"a": 1} }
