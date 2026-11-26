// Freeing a long list frees one node at a time (RFC 0013), as Go's
// collector does: a drop of each Box in turn would go one call deeper per
// node.
package main

import "fmt"

type Node struct {
	V    int
	Next *Node
}

func build(n int) *Node {
	var head *Node
	for i := 0; i < n; i++ {
		head = &Node{V: i, Next: head}
	}
	return head
}

func main() {
	for _, n := range []int{1000, 100000, 1000000} {
		l := build(n)
		fmt.Println(n, l.V)
	}
	fmt.Println("done")
}
