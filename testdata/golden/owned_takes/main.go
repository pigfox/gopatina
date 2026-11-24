// Command owned_takes moves owned pointers (RFC 0016) out of fields
// (RFC 0017, step 6). A field moved out of is refilled by the very next
// statement, so it never names an object something else owns:
// l := t.Left; t.Left = nil is Rust's let l = t.left.take().
package main

import "fmt"

type Tree struct {
	V           int
	Left, Right *Tree
}

func leaf(v int) *Tree { return &Tree{V: v} }

func size(t *Tree) int {
	if t == nil {
		return 0
	}
	return 1 + size(t.Left) + size(t.Right)
}

func main() {
	t := &Tree{V: 1, Left: leaf(2), Right: &Tree{V: 3, Left: leaf(5)}}
	l := t.Left // taken: t.Left is nil from here
	t.Left = nil
	r := t.Right // taken, and a new node put in its place
	t.Right = leaf(4)
	g := r.Left // a field of the taken node, taken too
	r.Left = leaf(9)
	var x = t.Right // a declaration takes it too
	t.Right = nil
	u := &Tree{V: 10, Left: l, Right: r} // the taken nodes move into a new tree
	fmt.Println(size(t), size(u), g.V, x.V, u.Right.Left.V, t.Left == nil, t.Right == nil)
}
