// Command owned_borrows walks lists of owned pointers (RFC 0016) with
// temporary borrows (RFC 0017, step 5): a local that only holds a
// pointer another place owns, and only reads through it, borrows it, so
// the owner is still there after the walk. for p := head; p != nil; p =
// p.Next is Rust's while let Some(n) = p.
package main

import "fmt"

type List struct {
	V    int
	Next *List
}

func push(n int) *List {
	var head *List
	for i := 1; i <= n; i++ {
		x := &List{V: i}
		x.Next = head
		head = x
	}
	return head
}

// total walks the list it is lent.
func total(head *List) int {
	s := 0
	for p := head; p != nil; p = p.Next {
		s += p.V
	}
	return s
}

// value reads a node it is lent.
func value(n *List) int { return n.V }

// evens skips the odd values, and stops at a large one.
func evens(head *List) int {
	s := 0
	for p := head; p != nil; p = p.Next {
		if p.V%2 == 1 {
			continue
		}
		if p.V > 8 {
			break
		}
		s += value(p)
	}
	return s
}

// last finds the last node, keeping a second borrow.
func last(head *List) int {
	var end *List
	for p := head; p != nil; p = p.Next {
		end = p
	}
	if end != nil {
		return end.V
	}
	return -1
}

func main() {
	head := push(10)
	sum := 0
	for p := head; p != nil; p = p.Next {
		sum += p.V
	}
	var cur *List = nil // a borrow, given a list below
	cur = head
	fmt.Println(cur.V)
	second := head.Next // a borrow of a field: head keeps the list
	third := second.Next
	fmt.Println(sum, total(head), evens(head), last(head), head.V, second.V, third.V)
	head.V = 100 // the borrows are done: head can change
	fmt.Println(head.V, total(head), last(nil))
}
