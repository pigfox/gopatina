// Borrows of the pointers interface values own (RFC 0012 with RFC 0017):
// x.(*T) without ok, read and changed through; errors.As changing the
// error it finds; a pointer to a number held as the value itself; value
// receivers and nil-tolerant receivers called through an interface.
package main

import (
	"errors"
	"fmt"
)

type Box struct{ W, H int }

func (b *Box) Area() int { return b.W * b.H }

func (b *Box) Grow(n int) { b.W += n }

type Temp float64

func (t *Temp) Area() int { return int(*t) }

func (t *Temp) Grow(n int) { *t += Temp(n) }

func newTemp(v float64) *Temp {
	t := Temp(v)
	return &t
}

type Pt struct{ X, Y int }

func (p Pt) Area() int { return p.X * p.Y }

func (p Pt) Grow(n int) {}

// Pair's Area changes its own copy: the interface's object stays as it is.
type Pair struct {
	A, B int
	Tag  string
}

func (q Pair) Area() int {
	q.A += q.B
	q.Tag += "!"
	return q.A + len(q.Tag)
}

func (q Pair) Grow(n int) {}

type Maybe struct{ N int }

func (m *Maybe) Area() int {
	if m == nil {
		return -1
	}
	return m.N
}

func (m *Maybe) Grow(n int) {}

type Shape interface {
	Area() int
	Grow(n int)
}

type Fail struct{ Code int }

func (f *Fail) Error() string { return fmt.Sprint("fail ", f.Code) }

func run(code int) error {
	if code == 0 {
		return nil
	}
	return &Fail{Code: code}
}

// Code is an error held as the number itself (no Box).
type Code int

func (c *Code) Error() string {
	if c == nil {
		return "no code"
	}
	return fmt.Sprint("code ", int(*c))
}

func code(v int) error {
	c := Code(v)
	return &c
}

type Node struct {
	V    int
	Next *Node
}

func newNode(v int, next *Node) *Node { return &Node{V: v, Next: next} }

func (n *Node) Area() int { return n.V }

func (n *Node) Grow(k int) { n.V += k }

func main() {
	shapes := []Shape{&Box{W: 2, H: 3}, newTemp(4), Pt{X: 2, Y: 5}, &Pair{A: 1, B: 2, Tag: "p"}}
	b := shapes[0].(*Box) // b borrows the *Box shapes[0] owns
	fmt.Println(b.W, b.Area())
	g := shapes[0].(*Box)
	g.Grow(3) // changed through the borrow: the interface's object changes
	fmt.Println(g.W, shapes[0].Area())
	t, ok := shapes[1].(*Temp)
	fmt.Println(ok, t.Area())
	m, ok := shapes[1].(*Temp)
	m.Grow(2)
	fmt.Println(ok, *m, shapes[1].Area(), shapes[2].Area(), shapes[3].Area(), shapes[3].Area())
	var maybe *Maybe
	var nothing Shape = maybe
	fmt.Println(nothing.Area(), maybe == nil)

	base := run(7)
	err := fmt.Errorf("run: %w", base)
	var f *Fail
	if errors.As(err, &f) {
		f.Code++ // changed through the borrow: err's *Fail changes
	}
	fmt.Println(err.Error())
	cerr := code(3)
	var c *Code
	if errors.As(cerr, &c) {
		fmt.Println("code", *c)
	}

	var head *Node // may be nil: Go makes the calls before reading head.V
	if len(shapes) > 1 {
		head = newNode(1, nil)
	}
	list := newNode(head.V, newNode(head.V+1, newNode(5, nil))) // a list of its own
	var s Shape = list
	s.Grow(head.V)
	fmt.Println(s.Area(), head.V)
}
