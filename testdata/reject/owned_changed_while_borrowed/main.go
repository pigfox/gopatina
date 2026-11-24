// Command owned_changed_while_borrowed changes, and moves, the owner of a
// list while a temporary borrow of it (RFC 0017, step 5) is still in use:
// in Rust, as in patina mode, the owner cannot change until the borrow's
// last use.
package main

import "fmt"

type List struct {
	V    int
	Next *List
}

func push(n int) *List {
	var head *List
	for i := 0; i < n; i++ {
		x := &List{V: i}
		x.Next = head
		head = x
	}
	return head
}

func main() {
	head := push(3)
	p := head
	head.V = 2
	fmt.Println(p.V)
	q := head
	head = nil
	fmt.Println(q.V)
	r := push(2)
	s := r
	t := &List{Next: r}
	fmt.Println(s.V, t.V)
}
