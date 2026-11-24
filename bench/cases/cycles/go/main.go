// Command cycles builds and drops doubly linked lists (bench/README.md).
// A list's nodes link both ways, which gives each node two names, so under
// one owner (RFC 0017) the nodes live in one slice and link by index, as
// Rust programmers write such a list: dropping the slice frees the list.
package main

import "fmt"

// Node is a list node; Prev and Next are indexes into the list's nodes,
// -1 for none.
type Node struct {
	V          int
	Prev, Next int
}

// build returns the n nodes of a list, the head first.
func build(n int) []Node {
	nodes := []Node{{V: 0, Prev: -1, Next: -1}}
	tail := 0
	for i := 1; i < n; i++ {
		nodes = append(nodes, Node{V: i, Prev: tail, Next: -1})
		nodes[tail].Next = len(nodes) - 1
		tail = len(nodes) - 1
	}
	return nodes
}

// walk follows the list forward from its head, then back from its last
// node.
func walk(nodes []Node) int {
	s := 0
	last := -1
	for p := 0; p != -1; p = nodes[p].Next {
		s += nodes[p].V
		last = p
	}
	for p := last; p != -1; p = nodes[p].Prev {
		s -= nodes[p].V / 2
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
