package b

import (
	"fmt"

	"example.com/multi/a"
)

func init() { fmt.Println("init b") }

// Kind is a named type with a String method, used from main.
type Kind int

const Big Kind = 7

func (k Kind) String() string {
	if k > 5 {
		return "big"
	}
	return "small"
}

// Scale multiplies a point of package a.
func Scale(p a.Point, k int) a.Point { return a.Point{X: p.X * k, Y: p.Y * k} }

// New has the same name as a.New.
func New() string { return "b.New" }

// Shift shifts by a count known only at run time, through the runtime.
func Shift(x, s int) int { return x << s }
