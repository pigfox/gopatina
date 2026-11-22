// Command cycles builds and drops doubly linked lists, whose Prev and Next
// pointers form cycles (bench/README.md). Go's collector reclaims them;
// GoPatina's reference counts do not (RFC 0013's known limitation), so
// this case measures the memory a program that drops cycles keeps.
package main

import "fmt"

type Node struct {
	V          int
	Prev, Next *Node
}

func build(n int) *Node {
	head := &Node{}
	tail := head
	for i := 1; i < n; i++ {
		x := &Node{V: i, Prev: tail}
		tail.Next = x
		tail = x
	}
	return head
}

func walk(head *Node) int {
	s := 0
	var last *Node
	for p := head; p != nil; p = p.Next {
		s += p.V
		last = p
	}
	for p := last; p != nil; p = p.Prev {
		s -= p.V / 2
	}
	return s
}

func run(rounds, n int) {
	total := 0
	for r := 0; r < rounds; r++ {
		total += walk(build(n))
	}
	fmt.Println(total)
}

// rounds and size are the benchmark's size; the test makes them small.
var rounds, size = 300, 10000

func main() { run(rounds, size) }
