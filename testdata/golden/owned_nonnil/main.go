// Command owned_nonnil uses owned pointers (RFC 0016) that GoPatina proves
// are never nil (RFC 0017, step 2), and some it cannot. A function whose
// every return is a new pointer returns a Box; a local declared from one
// and never assigned again is a Box; a parameter or receiver read through
// before anything else is a plain borrow (&T, &self), which a caller
// holding an Option unwraps at the call, where Go's callee would panic.
// The others stay Options, tested with if let Some.
package main

import "fmt"

type Node struct {
	Next *Node
	V    int
}

// fresh always returns a new pointer: a Box.
func fresh(v int) *Node { return &Node{V: v} }

// pair returns a new pointer holding another: a Box, whose field is an
// Option.
func pair(v int) *Node {
	n := &Node{V: v, Next: fresh(v + 1)}
	return n
}

// maybe may return nil: an Option.
func maybe(v int) *Node {
	if v < 0 {
		return nil
	}
	return pair(v)
}

// val reads through n first: n is a &Node.
func val(n *Node) int { return n.V }

// twice reads through n first, then passes its &Node on, as a &Node and
// as an Option.
func twice(n *Node) int {
	if n.V > 0 {
		return val(n) + count(n)
	}
	return 0
}

// later passes n on before reading through it: an Option.
func later(n *Node) int { return val(n) + n.V }

// count tests n for nil: an Option<&Node>.
func count(n *Node) int {
	if n == nil {
		return 0
	}
	return 1 + count(n.Next)
}

// sum reads through its receiver first: &self.
func (n *Node) sum() int {
	if n.Next != nil {
		return n.V + n.Next.sum()
	}
	return n.V
}

// last walks down with if let Some.
func last(n *Node) int {
	if nil != n.Next {
		return last(n.Next)
	}
	return n.V
}

// pick returns its local when it is not nil: the Option itself is
// returned, so the test stays a plain is_some.
func pick(v int) *Node {
	q := maybe(v)
	if q != nil {
		return q
	}
	return fresh(0)
}

// reset assigns the pointer it tests.
func reset(v int) int {
	q := maybe(v)
	if q != nil {
		q = nil
	}
	return count(q)
}

// again tests n.Next a second time inside the first test: no binding.
func again(n *Node) int {
	if n.Next != nil {
		if n.Next != nil {
			return 2
		}
	}
	return n.V
}

// add reads through its receiver: a call long enough to break inside.
func (n *Node) add(a, b, c int) int { return n.V + a + b + c }

func keep(x int) int { return x }

// both tests a pointer and then reads through it.
func both(n *Node) bool {
	if n != nil && n.V > 1 {
		return true
	}
	return false
}

func main() {
	var top *Node = pair(1)
	m := maybe(-1)
	has := m != nil
	fmt.Println(val(top), twice(top), later(top), count(top), top.sum(), last(top), has)
	fmt.Println(count(nil), count(m), both(top), both(m), both(top.Next))
	fmt.Println(pick(2).V, pick(-2).V, reset(3), reset(-3))
	fmt.Println(val(fresh(7)), fresh(8).sum(), pair(9).Next.sum())
	if m != nil {
		fmt.Println(val(m))
	}
	big, bigger := 1000000000000000, 2000000000000000
	total := keep(top.add(big*big+bigger*bigger+big, bigger*bigger*big+big*big, bigger*big+bigger))
	fmt.Println(total, again(top), again(fresh(1)))
}
