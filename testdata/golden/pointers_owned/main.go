// Command pointers_owned uses pointers that are never shared (RFC 0016):
// trees built from new pointers and then only read through, tested for nil
// and lent, so GoPatina keeps each one in a Box with one owner.
package main

import "fmt"

// Tag is shared (a slice holds its pointers), so it stays a
// reference-counted cell, held inside the owned nodes.
type Tag struct{ Label string }

type Node struct {
	Left, Right *Node
	Key         int
	Name        string
	Tag         *Tag
}

func build(depth, key int, tags []*Tag) *Node {
	if depth == 0 {
		return &Node{Key: key, Name: fmt.Sprint("leaf", key), Tag: tags[key%2]}
	}
	n := &Node{Left: build(depth-1, 2*key, tags), Right: build(depth-1, 2*key+1, tags), Key: key, Name: "inner"}
	return n
}

func pair(a, b int) *Node {
	return &Node{&Node{Key: a}, &Node{Key: b}, a + b, "pair", nil}
}

func (n *Node) sum() int {
	if n == nil {
		return 0
	}
	return n.Key + n.Left.sum() + n.Right.sum()
}

func (n *Node) depth() int {
	if n.Left == nil && n.Right == nil {
		return 1
	}
	d := 0
	if n.Left != nil {
		d = n.Left.depth()
	}
	if n.Right != nil && n.Right.depth() > d {
		d = n.Right.depth()
	}
	return d + 1
}

func leftmost(n *Node) string {
	if n.Left == nil {
		return n.Name
	}
	return leftmost(n.Left)
}

func label(n *Node) string {
	if n.Tag == nil {
		return "none"
	}
	return n.Tag.Label
}

// List is a long chain of owned pointers: dropping it must not need one
// call per node.
type List struct {
	V    int
	Next *List
}

func chain(n int) *List {
	if n == 0 {
		return nil
	}
	return &List{V: n, Next: chain(n - 1)}
}

func length(l *List) int {
	if l == nil {
		return 0
	}
	return 1 + length(l.Next)
}

func main() {
	tags := []*Tag{{Label: "even"}, {Label: "odd"}}
	t := build(3, 1, tags)
	fmt.Println(t.sum(), t.depth(), leftmost(t), t.Key, t.Name)
	fmt.Println(label(t), label(t.Left.Left.Left), label(t.Right.Right.Right), t.Left.Right.Left.Tag.Label)
	p := pair(3, 4)
	fmt.Println(p.sum(), p.Left.Key, p.Right.Key, p.depth(), label(p))
	t = build(1, 5, tags)
	fmt.Println(t.sum(), t.Left == nil, t.Right.Left == nil, nil != t.Left.Tag)
	var empty *Node
	fmt.Println(empty.sum(), empty == nil)
	fmt.Println(build(2, 3, tags).sum(), length(chain(5000)))
	fmt.Println(empty.depth())
}
