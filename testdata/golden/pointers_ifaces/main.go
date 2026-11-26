// Pointers as dynamic types (RFC 0013): method calls, assertions, type
// switches, ==, %T, errors.As, and printing through String and Error,
// with fmt's <nil> for a nil pointer.
package main

import (
	"errors"
	"fmt"
)

type Shape interface {
	Area() int
	Grow(n int)
}

type Rect struct{ W, H int }

func (r *Rect) Area() int  { return r.W * r.H }
func (r *Rect) Grow(n int) { r.W += n }

type Sq struct{ S int }

func (s *Sq) Area() int  { return s.S * s.S }
func (s *Sq) Grow(n int) { s.S += n }
func (s *Sq) String() string {
	return fmt.Sprint("sq", s.S)
}

type NotFound struct{ Key string }

func (e *NotFound) Error() string { return "not found: " + e.Key }

func find(k string) error {
	if k == "" {
		return nil
	}
	return &NotFound{Key: k}
}

func newRect(w, h int) *Rect { return &Rect{W: w, H: h} }

func main() {
	shapes := []Shape{newRect(2, 3), &Sq{S: 4}} // each object moved into its interface
	for _, s := range shapes {
		s.Grow(1)
		fmt.Println(s.Area())
	}
	fmt.Println(shapes[0].Area(), shapes[0] == shapes[0], shapes[1] == shapes[0])
	if q, ok := shapes[1].(*Sq); ok {
		q.Grow(10) // q borrows the *Sq shapes[1] owns: the change is the interface's
		fmt.Println(q.S, q)
		fmt.Println(shapes[1].Area())
	}
	switch v := shapes[0].(type) {
	case *Sq:
		fmt.Println("sq", v.S)
	case *Rect:
		fmt.Println("rect", v.W, v.H)
	}
	fmt.Printf("%T %T\n", shapes[0], shapes[1])
	err := find("k")
	fmt.Println(err)
	fmt.Println(find("") == nil, err != nil)
	var nf *NotFound
	if errors.As(err, &nf) { // nf borrows the *NotFound err owns
		fmt.Println("as", nf.Key)
	}
	wrapped := fmt.Errorf("lookup: %w", err) // err moves into the new error
	var deep *NotFound
	if errors.As(wrapped, &deep) {
		fmt.Println("as", deep.Key, wrapped)
	}
	var sq *Sq
	fmt.Println(sq)
	var nilShape Shape = sq // sq is nil: no object to own, so it stays usable
	fmt.Println(nilShape != nil, sq == nil)
	sqs := []*Sq{{S: 1}, nil, {S: 3}}
	fmt.Println(sqs)
	var none Shape = sq
	fmt.Println(none != nil)
}
