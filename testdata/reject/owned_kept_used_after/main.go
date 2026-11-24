// Command owned_kept_used_after uses an owned pointer (RFC 0016) after a
// function that keeps it took it: once moved in, only the function's
// owner names it (RFC 0017).
package main

import "fmt"

type Item struct {
	V    int
	Next *Item
}

type Stack struct{ Top *Item }

func newStack() *Stack { return &Stack{} }

func (s *Stack) push(n *Item) {
	n.Next = s.Top
	s.Top = n
}

func main() {
	s := newStack()
	x := &Item{V: 1}
	s.push(x)
	fmt.Println(x.V, s.Top.V)
}
