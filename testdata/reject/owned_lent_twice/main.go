// Command owned_lent_twice lends one owned pointer (RFC 0016) twice in a
// call that changes it through one of them: two names for one object
// while it is changed, which one owner per object refuses (RFC 0017).
package main

import "fmt"

type Node struct {
	Next *Node
	V    int
}

func mk(v int) *Node { return &Node{V: v, Next: &Node{V: v + 1}} }

func (n *Node) add(m *Node) { n.V += m.V }

func both(n *Node, m *Node) { n.V += m.V }

func main() {
	a := mk(1)
	a.add(a)
	both(a, a)
	both(a, a.Next)
	fmt.Println(a.V)
}
