// q borrows the *Sq the interface owns (RFC 0017): the interface cannot
// change while q is in use.
package main

import "fmt"

type Sq struct{ S int }

func (s *Sq) Area() int { return s.S * s.S }

func (s *Sq) Grow(n int) { s.S += n }

type Shape interface{ Area() int }

func main() {
	shapes := []Shape{&Sq{S: 2}, &Sq{S: 3}}
	if q, ok := shapes[1].(*Sq); ok {
		shapes[1] = &Sq{S: 9}
		q.Grow(1)
		fmt.Println(q.S, shapes[1].Area())
	}
}
