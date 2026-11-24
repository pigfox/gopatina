// Command owned_param_assigned assigns a parameter of an owned pointer
// type (RFC 0016) a new pointer. The parameter lends the caller's object,
// a borrow in Rust, which a new object cannot replace (RFC 0017).
package main

import "fmt"

type Node struct {
	Next *Node
	V    int
}

func mk(v int) *Node { return &Node{V: v} }

func fresh(n *Node, v int) int {
	if v > 0 {
		n = mk(v)
	}
	return n.V
}

func main() {
	fmt.Println(fresh(mk(1), 2))
}
