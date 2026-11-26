// A bare return in a function whose named result is a pointer it may lend
// back: it gives the named result, as Go's return does (found by
// gopatina-parity in regexp and text/template, where gopatina crashed).
package main

import "fmt"

type Node struct {
	V    int
	Next *Node
}

// find returns n if it holds v, or its named result r, still nil.
func find(n *Node, v int) (r *Node) {
	if n.V == v {
		return n
	}
	return
}

func main() {
	list := &Node{V: 1, Next: &Node{V: 2}}
	fmt.Println(find(list, 1) != nil, find(list, 3) == nil)
	if f := find(list, 1); f != nil {
		fmt.Println(f.V)
	}
}
