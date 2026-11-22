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
	d := c
	d.Inc()
	c.Inc()
	fmt.Println(c.N, d.N, c == d, c == newCounter())
	x := 5
	p := &x
	q := p
	*q = 7
	fmt.Println(x, *p, p == q)
}
