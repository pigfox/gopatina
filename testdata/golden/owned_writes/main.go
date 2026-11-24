// Command owned_writes changes numbers and strings through owned pointers
// (RFC 0017, step 3): a pointer written through is an exclusive borrow,
// &mut T, or Option<&mut T> where it may be nil, and a method that
// changes its receiver takes &mut self. The same pointers are lent to
// functions that only read them, as &T or Option<&T>, and between the
// two kinds of borrow.
package main

import "fmt"

type Node struct {
	Next *Node
	V    int
	S    string
}

func mk(v int) *Node { return &Node{V: v, Next: &Node{V: v + 1, Next: &Node{V: v + 2}}} }

// get only reads n, before anything else: a &Node.
func get(n *Node) int { return n.V }

// count may get nil: an Option<&Node>.
func count(n *Node) int {
	if n == nil {
		return 0
	}
	return 1 + count(n.Next)
}

// bump changes n first: a &mut Node.
func bump(n *Node, d int) {
	n.V += d
	n.S = "bumped"
}

// tick changes n first, by ++: a &mut Node.
func tick(n *Node) {
	n.V++
}

// grow changes its receiver: &mut self. It lends it on, as &T and &mut T,
// and changes the next node inside if let Some.
func (n *Node) grow() {
	n.V++
	d := get(n) // not bump(n, get(n)): n lent to bump, which changes it, beside another use
	bump(n, d)
	if n.Next != nil {
		n.Next.V *= 2
		tick(n.Next)
	}
	fmt.Println("grow", count(n), get(n))
}

// maybeSet may get nil: an Option<&mut Node>, lent on to readers and
// changers both.
func maybeSet(n *Node) {
	if n == nil {
		return
	}
	n.V = 100
	fmt.Println("set", get(n), count(n))
	bump(n, 1)
	again(n)
}

// again may get nil too.
func again(n *Node) {
	if n == nil {
		return
	}
	(n).V -= 3
	if n.Next != nil {
		tick(n.Next)
		n.Next.V++
	}
}

// finish lends its Option<&mut Node> last as a &mut Node.
func finish(n *Node) {
	if n == nil {
		return
	}
	bump(n, 2)
}

// twice lends its Option<&mut Node> twice, and in a loop: reborrowed
// each time but the last.
func twice(n *Node) {
	again(n)
	for i := 0; i < 2; i++ {
		again(n)
	}
	again(n)
}

// later reads its receiver only after a call: an Option<&mut Node>.
func (n *Node) later(d int) {
	fmt.Println("later")
	n.V += d
}

func main() {
	a := mk(1)
	a.V = 5
	bump(a, 3)
	bump(a, get(a))   // get(a) is computed first, before a is lent to bump
	bump(a.Next, a.V) // distinct fields of a
	a.grow()
	a.Next.grow()
	maybeSet(a.Next)
	maybeSet(a)
	maybeSet(nil)
	again(a)
	tick(a)
	a.later(2)
	a.Next.later(1)
	var b *Node
	if a.V > 0 {
		b = mk(7)
	}
	b.V--
	b.S = "b"
	bump(b, 1)
	maybeSet(b)
	if b.Next != nil {
		b.Next.Next.V = 50
		b.Next.S = "next of b"
	}
	if a.Next != nil {
		a.Next.grow()
	}
	twice(a)
	twice(nil)
	finish(a.Next)
	finish(nil)
	fmt.Println(a.V, a.S, a.Next.V, b.V, b.S, b.Next.S, get(b), count(b))
}
