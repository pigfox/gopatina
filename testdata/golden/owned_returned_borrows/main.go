// Command owned_returned_borrows calls functions and methods that return
// one of their pointer inputs, or a place through it. The result lends the
// input back, as Rust's elided lifetime does (RFC 0017, decision 4): the
// caller's result is a borrow of its argument, not a new owner, and the
// argument stays the owner.
package main

import "fmt"

// Node is a list node.
type Node struct {
	V    int
	Next *Node
}

func newNode(v int, next *Node) *Node { return &Node{V: v, Next: next} }

// second returns the node after n, nil at the end.
func second(n *Node) *Node { return n.Next }

// third passes second's borrow on.
func third(n *Node) *Node { return second(n.Next) }

// same returns n itself.
func same(n *Node) *Node {
	_ = n.V
	return n
}

// Self returns its receiver.
func (n *Node) Self() *Node { return n }

// after returns the node after a, reading b: the result borrows from a,
// so Rust names the lifetime, fn after<'a>(a: &'a .., b: ..) -> &'a ...
func after(a, b *Node) *Node {
	if b.V > 100 {
		return nil
	}
	return a.Next
}

// named returns the node after n, reading a string too.
func named(s string, n *Node) *Node {
	_ = len(s)
	return n.Next
}

// Pick is a method returning a borrow of its parameter, not its receiver.
func (n *Node) Pick(m *Node) *Node {
	_ = n.V
	return m.Next
}

// bumped changes n and returns the node after it: an exclusive borrow
// lent back, Option<&mut Node>.
func bumped(n *Node) *Node {
	n.V += 100
	return n.Next
}

// Me returns its receiver, changed through by its caller.
func (n *Node) Me() *Node { return n }

// Add changes the node through its receiver.
func (n *Node) Add(v int) { n.V += v }

// total adds up a list.
func total(n *Node) int {
	t := 0
	for p := n; p != nil; p = p.Next {
		t += p.V
	}
	return t
}

func main() {
	l := newNode(1, newNode(2, newNode(3, nil)))
	t := second(l)
	fmt.Println(t.V, third(l).V, l.V)
	fmt.Println(same(l).V, l.Self().V, total(second(l)))
	other := newNode(9, nil)
	fmt.Println(after(l, other).V, named("x", l).V, other.Pick(l).V, other.V)
	if second(t) != nil {
		fmt.Println("more after", t.V)
	}

	b := bumped(l) // l is in use only through b until b's last use
	b.V += 5
	fmt.Println(b.V)
	s := l.Me() // Me only reads, but s is changed through: &mut
	s.Add(1000)
	fmt.Println(l.V, l.Next.V)
}
