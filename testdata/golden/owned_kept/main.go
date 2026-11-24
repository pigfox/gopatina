// Command owned_kept passes owned pointers (RFC 0016) to functions that
// keep them (RFC 0017, step 8): a parameter that is returned, stored or
// moved on is moved in, an Option<Box<T>> by value, and the caller's
// variable is dead from there. A binary search tree's insert keeps its
// tree, and its caller hands a child over by taking it out of the field
// the result is put back into: t.Left = insert(t.Left, v).
package main

import "fmt"

type Tree struct {
	V           int
	Left, Right *Tree
}

// insert keeps t: it returns it.
func insert(t *Tree, v int) *Tree {
	if t == nil {
		return &Tree{V: v}
	}
	if v < t.V {
		t.Left = insert(t.Left, v)
	} else {
		t.Right = insert(t.Right, v)
	}
	return t
}

// walk only reads t.
func walk(t *Tree, out []int) []int {
	if t == nil {
		return out
	}
	out = walk(t.Left, out)
	out = append(out, t.V)
	return walk(t.Right, out)
}

type Stack struct {
	Top *Item
	N   int
}

type Item struct {
	V    int
	Next *Item
}

func newStack() *Stack { return &Stack{} }

// push keeps n: it stores it.
func (s *Stack) push(n *Item) {
	n.Next = s.Top
	s.Top = n
	s.N++
}

// relay keeps n by passing it to a function that keeps it.
func relay(s *Stack, n *Item) {
	s.push(n)
}

func main() {
	var root *Tree
	for _, v := range []int{5, 3, 8, 1, 4, 9, 7} {
		root = insert(root, v)
	}
	fmt.Println(walk(root, nil))
	s := newStack()
	for i := 0; i < 4; i++ {
		s.push(&Item{V: i})
	}
	x := &Item{V: 10}
	relay(s, x) // x moves into the stack
	sum := 0
	for p := s.Top; p != nil; p = p.Next {
		sum += p.V
	}
	fmt.Println(s.N, s.Top.V, sum)
}
