// Command owned_plain_fields keeps plain data in structs behind owned
// pointers: a slice of numbers, a nested struct of numbers and a slice of
// strings. A Box owns them as it owns its other fields, so the pointers
// keep one owner and the data is changed in place through them
// (RFC 0017).
package main

import "fmt"

// Pos is plain data.
type Pos struct{ X, Y int }

// Stack holds numbers, a position, names and the stack below it.
type Stack struct {
	Items []int
	At    Pos
	Names []string
	Below *Stack
}

// newStack returns an empty stack named name, on top of below.
func newStack(name string, below *Stack) *Stack {
	return &Stack{Names: []string{name}, Below: below}
}

// push puts v on the stack.
func (s *Stack) push(v int) {
	s.Items = append(s.Items, v)
	s.At.X++
}

// pop reads the top value and clears its slot.
func (s *Stack) pop() int {
	v := s.Items[len(s.Items)-1]
	s.Items[len(s.Items)-1] = 0
	s.At.Y = v
	return v
}

// total adds up the stack and the stacks below it.
func total(s *Stack) int {
	t := 0
	for p := s; p != nil; p = p.Below {
		for _, v := range p.Items {
			t += v
		}
	}
	return t
}

func main() {
	low := newStack("low", nil)
	low.push(1)
	low.push(2)
	top := newStack("top", low)
	top.push(10)
	top.push(20)
	top.Names = append(top.Names, "second")
	fmt.Println(top.pop(), len(top.Items), top.At.X, top.At.Y)
	top.At = Pos{X: 7}
	fmt.Println(total(top), top.Names, top.At.X, top.At.Y, top.Below.Names[0], top.Below.At.X)
}
