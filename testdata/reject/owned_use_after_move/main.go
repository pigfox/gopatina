// Command owned_use_after_move uses owned pointers (RFC 0016) after they
// moved: once b := a moves the list to b, only b names it, and once b
// moves into a new node's literal, only that node does (RFC 0017).
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
	a := push(2)
	b := a
	c := &List{V: 1, Next: b}
	fmt.Println(a.V, b.V, c.V)
}
