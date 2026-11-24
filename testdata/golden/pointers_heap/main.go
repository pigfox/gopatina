package main

import "fmt"

type Node struct {
	Val  int
	Next *Node
}

type Counter struct{ N int }

func (c *Counter) Inc() { c.N++ }

func push(head *Node, v int) *Node { return &Node{Val: v, Next: head} }

func newCounter() *Counter { return &Counter{} }

// startAt names the new pointer before returning it, which the crate
// returns directly.
func startAt(n int) *Counter {
	c := &Counter{N: n}
	return c
}

func main() {
	var list *Node
	for i := 1; i <= 3; i++ {
		list = push(list, i)
	}
	for n := list; n != nil; n = n.Next {
		fmt.Print(n.Val, " ")
	}
	fmt.Println()
	c := newCounter()
	d := newCounter()
	d.Inc()
	c.Inc()
	c.Inc()
	fmt.Println(c.N, d.N, c == d, c == newCounter(), c != c)
	s := startAt(9)
	s.Inc()
	fmt.Println(s.N)
	x, y := 5, 5
	p := &x
	*p = 7
	q := &y
	fmt.Println(*p, *q, p == q)
	fmt.Println(x, y)
}
