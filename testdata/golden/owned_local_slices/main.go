// Command owned_local_slices keeps owned pointers (RFC 0016) in a local
// slice, a Vec owning each (RFC 0017): built by a literal of new pointers,
// appended to with new and moved ones, changed in place by index, lent to
// a function that ranges over it, and moved whole to another variable.
package main

import "fmt"

type Node struct {
	V    int
	Next *Node
}

func mk(v int) *Node { return &Node{V: v} }

func sum(ns []*Node) int {
	t := 0
	for _, n := range ns {
		t += n.V
	}
	return t
}

func main() {
	s := []*Node{mk(1), mk(2)}
	x := mk(3)
	s = append(s, x, mk(4))
	s[0].V += 10
	fmt.Println(sum(s), len(s), s[1].V)
	for i := range s {
		s[i].V *= 2
	}
	fmt.Println(sum(s))
	t := s // the whole slice moves: s is dead from here
	fmt.Println(len(t), t[3].V)
}
