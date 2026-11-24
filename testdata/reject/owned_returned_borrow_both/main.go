// Command owned_returned_borrow_both calls a method that returns a borrow
// of its receiver both from a caller that changes through the result and
// from one that only reads it while another borrow of the receiver is in
// use. The method has one mode, so it lends exclusively for both, and the
// reading call conflicts with the other borrow; a pair of methods, as Rust
// writes get and get_mut, is not generated yet (RFC 0017).
package main

import "fmt"

// Node is a list node.
type Node struct {
	V    int
	Next *Node
}

func newNode(v int, next *Node) *Node { return &Node{V: v, Next: next} }

// second returns the node after n.
func second(n *Node) *Node { return n.Next }

// Me returns its receiver.
func (n *Node) Me() *Node { return n }

// Add changes the node.
func (n *Node) Add(v int) { n.V += v }

func main() {
	l := newNode(1, newNode(2, nil))
	t := second(l)
	fmt.Println(l.Me().V, t.V) // only reads, but Me lends l exclusively
	s := l.Me()
	s.Add(10)
	fmt.Println(l.V)
}
