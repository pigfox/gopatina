// Command pointers_alias reaches each object through its one owner and
// borrows of it (RFC 0017): fields, a Pair of Stacks, a tree, int
// pointers, and pointers compared by address. A second pointer to the same
// object, kept beside its first, is refused
// (testdata/reject/owned_aliases_compared).
package main

import "fmt"

type Stack struct {
	items []int
	name  string
	top   *Stack
}

func NewStack(name string) *Stack { return &Stack{name: name} }

func (s *Stack) Push(v int) { s.items = append(s.items, v) }

func (s *Stack) Pop() int {
	v := s.items[len(s.items)-1]
	s.items[len(s.items)-1] = 0
	return v
}

func (s *Stack) Len() int { return len(s.items) }

func (s *Stack) Name() string { return s.name + "!" }

type Pair struct{ A, B *Stack }

type Tree struct {
	Left, Right *Tree
	Key         int
}

func insert(t *Tree, k int) *Tree {
	if t == nil {
		return &Tree{Key: k}
	}
	if k < t.Key {
		t.Left = insert(t.Left, k)
	} else {
		t.Right = insert(t.Right, k)
	}
	return t
}

func walk(t *Tree) int {
	if t == nil {
		return 0
	}
	return walk(t.Left) + t.Key + walk(t.Right)
}

func addr(x int) *int { return &x }

func swap(a, b *int) { *a, *b = *b, *a }

func bump(p *int) int {
	*p += 10
	return *p
}

func main() {
	s := NewStack("s")
	s.Push(1)
	s.Push(2)
	s.Push(3)
	fmt.Println(s.Pop(), s.Len(), s.Name())
	s.name += "x"
	fmt.Println(s.name, s.items[0])
	s.items[0] = 42
	fmt.Println(s.items[0], len(s.items))
	s.top = NewStack("top")
	s.top.Push(7)
	s.top.top = NewStack("toptop")
	s.top.top.Push(8)
	fmt.Println(s.top.items[0], s.top.top.items[0], s.top == s, s.top.top == nil)
	p := Pair{A: s, B: NewStack("b")}
	p.A.Push(9)
	fmt.Println(p.A.Len(), p.A == p.B, p.B.Len(), p.A.top != nil)
	var root *Tree
	for _, k := range []int{5, 3, 8, 1, 4} {
		root = insert(root, k)
	}
	fmt.Println(walk(root), root.Left.Key, root.Right.Key)
	a, b := addr(1), addr(2)
	swap(a, b)
	fmt.Println(*a, *b)
	n := 1
	m := &n
	r := bump(m)
	fmt.Println(r, *m+bump(m), n)
	var e *Stack
	fmt.Println(e == nil, e != nil, p.A != nil)
	e = NewStack("e")
	e.name += "!"
	fmt.Println(e.name, e == p.B)
}
