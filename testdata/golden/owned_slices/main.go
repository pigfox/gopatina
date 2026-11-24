// Command owned_slices builds a tree whose nodes hold their children in a
// slice of owned pointers (RFC 0016), a Vec owning each (RFC 0017, step
// 7): appended to with new nodes and with nodes moved in, read through by
// index and by range, whose variable borrows each child in turn.
package main

import "fmt"

type Node struct {
	V    int
	Kids []*Node
}

func leaf(v int) *Node { return &Node{V: v} }

// sum reads the tree by range: k borrows each child.
func sum(n *Node) int {
	t := n.V
	for _, k := range n.Kids {
		t += sum(k)
	}
	return t
}

// count reads the tree by index.
func count(n *Node) int {
	c := 1
	for i := 0; i < len(n.Kids); i++ {
		c += count(n.Kids[i])
	}
	return c
}

// widest returns the largest number of children of any node.
func widest(n *Node) int {
	w := len(n.Kids)
	for i, k := range n.Kids {
		if c := widest(k); c > w || i < 0 {
			w = c
		}
	}
	return w
}

// deep builds a chain of n nodes, each the only child of the last.
func deep(n int) *Node {
	root := leaf(0)
	for i := 1; i < n; i++ {
		p := leaf(i)
		p.Kids = append(p.Kids, root)
		root = p
	}
	return root
}

func main() {
	root := leaf(1)
	root.Kids = append(root.Kids, leaf(2), leaf(3))
	x := leaf(4)
	x.Kids = append(x.Kids, leaf(5), leaf(6), leaf(7))
	root.Kids = append(root.Kids, x) // x moves into root's children
	root.Kids[0].V += 10             // a child changed in place
	fmt.Println(sum(root), count(root), widest(root), len(root.Kids), root.Kids[1].V, root.Kids[2].Kids[0].V)
	n := 0
	for i := range root.Kids { // only the index
		n += i
	}
	d := deep(100000)
	fmt.Println(d.V, len(d.Kids), d.Kids[0].V, n)
}
