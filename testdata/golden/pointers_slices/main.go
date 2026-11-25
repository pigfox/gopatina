// Slices of pointers (RFC 0013): a tree of nodes, each owned by the Vec
// of its parent's Kids (RFC 0017), appended to, indexed and ranged over.
package main

import "fmt"

type Node struct {
	V    int
	Kids []*Node
}

func leaf(v int) *Node { return &Node{V: v} }

// add appends k to n's kids, which own it.
func (n *Node) add(k *Node) { n.Kids = append(n.Kids, k) }

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
	root.Kids[0].add(leaf(4))
	root.Kids[1].V += 10
	all := []*Node{leaf(5), leaf(6)}
	all[1].V += 10
	ps := make([]*Node, 3)
	ps[0] = leaf(7)
	fmt.Println(sum(root), len(all), all[0] == all[1], ps[1] == nil, ps[0].V, root.Kids[1].V)
	for i := range all {
		all[i].V *= 2
		fmt.Println(i, all[i].V)
	}
	fmt.Println(root.Kids[0].V, len(root.Kids[0].Kids), sum(root.Kids[0]), sum(root.Kids[1]))
	fmt.Println(sum(root), sum(all[0])+sum(all[1]))
}
