// Command iface_copies copies interface values (RFC 0012 with RFC 0017).
// An interface value cannot be changed through itself, so a copy only read
// while its source does not change is a borrow of the source,
// let b = &a, with no clone. A copy that is kept, changed, or in use when
// its source changes is a clone, Go's copy. A struct with an interface
// field, put in an interface, makes the interface type recursive: a Box.
package main

import "fmt"

// Shape has an area.
type Shape interface{ Area() int }

// Rect is a named rectangle.
type Rect struct {
	W, H int
	Name string
}

func (r Rect) Area() int { return r.W*r.H + len(r.Name) }

// Labeled holds a shape and a tag: an interface inside a dynamic value.
type Labeled struct {
	S   Shape
	Tag string
}

func (l Labeled) Area() int { return l.S.Area() + len(l.Tag) }

func (l Labeled) String() string { return "labeled " + l.Tag }

func measure(s Shape) int { return s.Area() }

// same compares a parameter, a borrow, with a value.
func same(s Shape) bool {
	var r Shape = Rect{W: 2, H: 3, Name: "r"}
	return s == r
}

func keep(all []Shape, s Shape) []Shape {
	all = append(all, s)
	return all
}

func main() {
	var a Shape = Rect{W: 2, H: 3, Name: "r"}
	b := a // only read while a is unchanged: lent
	fmt.Println(measure(b), b.Area(), measure(a), b == a, a != b)
	a = Rect{W: 4, H: 1, Name: "s"} // b's last use is before this
	b2 := a                         // a's value not used after: moved
	a = Rect{W: 6, H: 1, Name: "s"}
	fmt.Println(measure(b2), measure(a))

	c := a // a may change while c is still in use: cloned
	if measure(a) > 5 {
		a = Rect{W: 1, H: 1, Name: "t"}
	}
	fmt.Println(measure(c), measure(a))

	d := a // handed on, kept in a slice: cloned
	var all []Shape
	all = keep(all, d)
	fmt.Println(len(all), measure(a))

	e := a // assigned again: cloned
	e = Rect{W: 5, H: 5, Name: "u"}
	fmt.Println(measure(e), measure(a))

	f := a // its address taken: cloned
	pf := &f
	fmt.Println(measure(*pf), measure(a))

	var l Shape = Labeled{S: a, Tag: "tag"}
	g := l
	fmt.Println(measure(g), measure(l), l, same(b2))
}
