// Slices of pointers (RFC 0013): a tree of nodes, ranged over through
// their cells, appended to and indexed.
package main

import "fmt"

type Node struct {
	V    int
	Kids []*Node
}

func leaf(v int) *Node { return &Node{V: v} }

func sum(n *Node) int {
	t := n.V
	for _, k := range n.Kids {
		t += sum(k)
	}
	return t
}

func main() {
	root := leaf(1)
	root.Kids = append(root.Kids, leaf(2), leaf(3))
	k0 := root.Kids[0]
	k0.Kids = append(k0.Kids, leaf(4))
	all := []*Node{root, root.Kids[1]}
	all[1].V += 10
	ps := make([]*Node, 3)
	ps[0] = root
	fmt.Println(sum(root), len(all), all[0] == root, ps[1] == nil, ps[0].V, root.Kids[1].V)
	for i, p := range all {
		p.V *= 2
		fmt.Println(i, p.V)
	}
	fmt.Println(sum(root))
}
