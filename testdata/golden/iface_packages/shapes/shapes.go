// Package shapes declares an interface, types implementing it and a
// function that works through it, for iface_packages.
package shapes

import "fmt"

// Shape is anything with an area and a name.
type Shape interface {
	Area() int
	Name() string
}

// Rect is a rectangle.
type Rect struct{ W, H int }

// Area is the rectangle's area.
func (r Rect) Area() int { return r.W * r.H }

// Name is "rect".
func (r Rect) Name() string { return "rect" }

// Unit is a square of side n.
type Unit int

// Area is the square's area.
func (u Unit) Area() int { return int(u) * int(u) }

// Name is "unit".
func (u Unit) Name() string { return "unit" }

// Total adds the areas, and reports the kinds it saw.
func Total(all []Shape) int {
	t := 0
	for _, s := range all {
		switch s.(type) {
		case Rect:
			fmt.Println("a rect of", s.Area())
		default:
			fmt.Println("a", s.Name(), "of", s.Area())
		}
		t += s.Area()
	}
	return t
}

// Biggest returns the shape with the largest area, or nil.
func Biggest(all []Shape) Shape {
	var best Shape
	for _, s := range all {
		if best == nil || s.Area() > best.Area() {
			best = s
		}
	}
	return best
}
