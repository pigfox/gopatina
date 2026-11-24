// Command binary_trees is the binary-trees benchmark (bench/README.md) with
// a smaller size, beside reference.rs, the Rust a person writes for it.
// closeness.txt says how close GoPatina's Rust reads to that reference.
// It allocates and frees many small trees through pointers: Go's garbage
// collector does that work in Go, and a Box with one owner in Rust.
package main

import "fmt"

type Node struct{ Left, Right *Node }

func bottomUp(depth int) *Node {
	if depth <= 0 {
		return &Node{}
	}
	return &Node{Left: bottomUp(depth - 1), Right: bottomUp(depth - 1)}
}

func (n *Node) check() int {
	if n.Left == nil {
		return 1
	}
	return 1 + n.Left.check() + n.Right.check()
}

func run(maxDepth int) {
	long := bottomUp(maxDepth)
	for d := 4; d <= maxDepth; d += 2 {
		iters := 1 << (maxDepth - d + 4)
		total := 0
		for i := 0; i < iters; i++ {
			total += bottomUp(d).check()
		}
		fmt.Printf("%d trees of depth %d check: %d\n", iters, d, total)
	}
	fmt.Printf("long lived tree of depth %d check: %d\n", maxDepth, long.check())
}

// maxDepth is the size: 18 in the benchmark (bench/cases/binary-trees).
var maxDepth = 10

func main() { run(maxDepth) }
