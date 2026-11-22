// Types through which a cycle of pointers can form (RFC 0013): -explain
// notes each field of one, since a cycle is not reclaimed. A doubly linked
// list closed into a ring prints as Go prints it; its memory is kept.
package main

import "fmt"

// Edge and Vertex can point at each other.
type Edge struct {
	To   *Vertex
	Cost int
}

type Vertex struct {
	Name string
	Out  []*Edge
}

// Outer reaches itself through a struct value.
type Inner struct{ Back *Outer }

type Outer struct {
	In Inner
	N  int
}

// Item never leads back to Owner, and Chain only to itself: no note on
// Owner.Item or Start.Head.
type Item struct{ V int }

type Owner struct{ Item *Item }

type Chain struct{ Next *Chain }

type Start struct{ Head *Chain }

// Ring is a doubly linked ring: every node is in two cycles.
type Ring struct {
	V          int
	Prev, Next *Ring
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
	a := &Vertex{Name: "a"}
	b := &Vertex{Name: "b"}
	a.Out = append(a.Out, &Edge{To: b, Cost: 2})
	b.Out = append(b.Out, &Edge{To: a, Cost: 3})
	fmt.Println(a.Out[0].To.Name, a.Out[0].To.Out[0].To.Name, a.Out[0].Cost+b.Out[0].Cost)
	o := &Outer{N: 1}
	o.In.Back = o
	fmt.Println(o.In.Back.In.Back.N, o.In.Back == o)
	w := Owner{Item: &Item{V: 4}}
	s := Start{Head: &Chain{Next: &Chain{}}}
	fmt.Println(w.Item.V, s.Head.Next.Next == nil)
	sum := 0
	for i := 0; i < 3; i++ {
		r := ring(5)
		for p := r.Next; p != r; p = p.Next {
			sum += p.V + p.Prev.V
		}
	}
	fmt.Println(sum)
}
