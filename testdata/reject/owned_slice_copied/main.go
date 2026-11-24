// Command owned_slice_copied copies a slice of owned pointers (RFC 0016)
// and uses both: in Go the copy shares the elements, so each would have
// two owners, which one owner per object refuses (RFC 0017).
package main

import "fmt"

type Node struct{ V int }

func mk(v int) *Node { return &Node{V: v} }

func main() {
	s := []*Node{mk(1), mk(2)}
	t := s
	t[0].V = 5
	fmt.Println(s[0].V, t[0].V)
}
