// Command owned_aliases_kept keeps second pointers to objects other places
// own, as pointers_slices, pointers_owned and pointers_cycles did while
// such pointers were shared cells (RFC 0013). Each object then has two
// owners, which one owner per object refuses (RFC 0017): a copy out of a
// slice changed through, a slice holding a node and its child, a node
// stored in a second slice, one tag in every leaf, a ring closed through
// Prev and Next, and a range variable changed through.
package main

import "fmt"

type Node struct {
	V    int
	Kids []*Node
	Tag  *Tag
}

type Tag struct{ Label string }

type Ring struct {
	V          int
	Prev, Next *Ring
}

func leaf(v int) *Node { return &Node{V: v} }

func build(depth int, tags []*Tag) *Node {
	if depth == 0 {
		return &Node{Tag: tags[depth%2]} // every leaf holds one of two tags
	}
	n := leaf(depth)
	n.Kids = append(n.Kids, build(depth-1, tags), build(depth-1, tags))
	return n
}

func ring(n int) *Ring {
	first := &Ring{V: 0}
	first.Prev, first.Next = first, first
	for i := 1; i < n; i++ {
		r := &Ring{V: i, Prev: first.Prev, Next: first}
		first.Prev.Next = r
		first.Prev = r
	}
	return first
}

func main() {
	root := leaf(1)
	root.Kids = append(root.Kids, leaf(2), leaf(3))
	k0 := root.Kids[0] // a copy out of the Vec, changed through below
	k0.Kids = append(k0.Kids, leaf(4))
	all := []*Node{root, root.Kids[1]}
	ps := make([]*Node, 2)
	ps[0] = root
	for _, p := range all {
		p.V *= 2
	}
	tags := []*Tag{{Label: "even"}, {Label: "odd"}}
	t := build(2, tags)
	r := ring(3)
	fmt.Println(root.V, len(all), ps[0].V, t.Kids[0].Kids[0].Tag.Label, r.Next.Next.Next.V)
}
