// Command owned_walked_borrow walks a list from its head to the node it
// returns: a borrow of the head all the way (RFC 0017), as Rust's
// fn find(head: Option<&Node>, v: i64) -> Option<&Node>. The walker is
// only given the head, a field of itself or nil, and only read, compared
// and returned.
package main

import "fmt"

// Node is a link of a list, newest first.
type Node struct {
	V    int
	Name string
	Next *Node
}

func push(h *Node, v int) *Node { return &Node{V: v, Name: fmt.Sprint("n", v), Next: h} }

// find returns the first node holding v, or nil.
func find(head *Node, v int) *Node {
	for p := head; p != nil; p = p.Next {
		if p.V == v {
			return p
		}
	}
	return nil
}

// last returns the list's last node.
func last(head *Node) *Node {
	p := head
	for p.Next != nil {
		p = p.Next
	}
	return p
}

// At walks to the i-th node from n, or nil.
func (n *Node) At(i int) *Node {
	p := n
	for ; i > 0 && p != nil; i-- {
		p = p.Next
	}
	return p
}

func main() {
	var l *Node
	for i := 1; i <= 5; i++ {
		l = push(l, i)
	}
	f := find(l, 3)
	fmt.Println(f.V, f.Name, f.Next.V, find(l, 9) == nil, find(l, 5) == l)
	fmt.Println(last(l).V, l.At(2).Name, l.At(9) == nil, l.At(0) == l)
	l.V = 50 // the borrows are done with: the list changes
	fmt.Println(l.V, find(l, 50).Name)
}
