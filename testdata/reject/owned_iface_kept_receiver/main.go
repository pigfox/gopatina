// Command owned_iface_kept_receiver calls, through an interface, a method
// that returns its pointer receiver, as pointers_methods did while pointers
// in interfaces were shared cells (RFC 0013). The interface owns the Rect,
// so the method would move it out while the interface still holds it, a
// second owner: refused (RFC 0017).
package main

import "fmt"

// Rect is a rectangle.
type Rect struct{ W, H int }

func (r Rect) Area() int { return r.W * r.H }

// Self returns its receiver: the same object.
func (r *Rect) Self() Shape { return r }

// Shape has an area and itself.
type Shape interface {
	Area() int
	Self() Shape
}

func main() {
	var s Shape = &Rect{W: 2, H: 5}
	t := s.Self()
	fmt.Println(s.Area(), t.Area(), s == t)
}
