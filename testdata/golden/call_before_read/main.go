// Command call_before_read reads a variable, or a place under one, and in
// the same expression calls a function or method that changes it. gc
// makes the calls of an expression before it reads the variables, so the
// read sees the change; the Go spec leaves that order unspecified, and
// GoPatina matches gc, its reference. Rust evaluates left to right, so
// GoPatina makes the calls first, into locals, as gc does.
package main

import "fmt"

// Acc accumulates.
type Acc struct{ N int }

func newAcc() *Acc { return &Acc{N: 1} }

// add changes the accumulator through its pointer.
func add(a *Acc, v int) int {
	a.N += v
	return a.N
}

// Bump changes the accumulator through its receiver.
func (a *Acc) Bump() int {
	a.N *= 2
	return a.N
}

// peek only reads.
func peek(a *Acc) int { return a.N }

// first changes the first element.
func first(s []int) int {
	s[0] += 100
	return s[0]
}

// addTo changes a number through its pointer.
func addTo(p *int, v int) int {
	*p += v
	return *p
}

// Pos is a local value with a method that changes it.
type Pos struct{ X int }

// Next moves the position.
func (p *Pos) Next() int {
	p.X++
	return p.X
}

func main() {
	a := newAcc()
	fmt.Println(a.N+add(a, 10), a.N)
	fmt.Println(a.N*a.Bump(), a.N)
	v := (*a).N + add(a, 1)
	fmt.Println(v)
	fmt.Println(add(a, 2)+a.N, a.N)
	fmt.Println(a.N+peek(a), a.N)

	xs := []int{1, 2}
	fmt.Println(xs[0]+first(xs), xs[0])
	x := 4
	fmt.Println(x+addTo(&x, 3), x)
	var p Pos
	fmt.Println(p.X+p.Next(), p.X)
	ps := []Pos{{X: 1}, {X: 2}}
	fmt.Println(ps[1].X+ps[1].Next(), ps[1].X)

	switch x + addTo(&x, 2) {
	case 11:
		fmt.Println("eleven", x)
	default:
		fmt.Println("other", x)
	}
	if x+addTo(&x, 1) > 100 {
		fmt.Println("big", x)
	} else if x+addTo(&x, 1) > 10 {
		fmt.Println("medium", x)
	}
}
