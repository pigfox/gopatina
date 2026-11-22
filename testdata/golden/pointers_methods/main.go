// Methods called through an interface on a pointer (RFC 0013): of named
// non-struct types, value methods through the pointer, and a method that
// keeps its receiver.
package main

import "fmt"

type Count int

func (c *Count) Inc()    { *c++ }
func (c Count) Get() int { return int(c) * 10 }

type Name string

func (n Name) Up() string { return string(n) + "!" }

type Rect struct{ W, H int }

func (r Rect) Area() int { return r.W * r.H }

func (r *Rect) Self() Shape { return r }

type Counter interface {
	Inc()
	Get() int
}

type Upper interface{ Up() string }

type Shape interface {
	Area() int
	Self() Shape
}

func newCount(c Count) *Count { return &c }

func newName(n Name) *Name { return &n }

func main() {
	var c Counter = newCount(4)
	c.Inc()
	c.Inc()
	fmt.Println(c.Get())
	var u Upper = newName("go")
	fmt.Println(u.Up())
	var s Shape = &Rect{W: 2, H: 5}
	t := s.Self()
	fmt.Println(s.Area(), t.Area(), s == t)
}
