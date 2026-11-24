// Types through which a cycle of pointers can form (RFC 0013): -explain
// notes each field of one, since a cycle is not reclaimed. A doubly linked
// list closed into a ring prints as Go prints it; its memory is kept.
package main

import "fmt"

// Edge and Vertex can point at each other. Here each Vertex has one owner,
// the Edge leading to it; a graph whose edges close a cycle is refused
// (testdata/reject/owned_graph_cycle).
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
	Seen       []int
	H          *Hand
}

// Note clears the last value seen, written through the ring's cell.
func (r *Ring) Note(v int) {
	r.Seen = append(r.Seen, v)
	r.Seen[len(r.Seen)-1] = 0
}

// Hand holds a place on a ring, and the ring node it is at holds it back:
// Ring.H leads back to a Ring through Hand.At.
type Hand struct {
	At *Ring
	N  int
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
	b.Out = append(b.Out, &Edge{To: &Vertex{Name: "c"}, Cost: 3})
	a.Out = append(a.Out, &Edge{To: b, Cost: 2})
	fmt.Println(a.Out[0].To.Name, a.Out[0].To.Out[0].To.Name, a.Out[0].Cost+a.Out[0].To.Out[0].Cost, a.Out[0].To == a.Out[0].To.Out[0].To)
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
	h := &Hand{At: ring(3), N: 1}
	h.At.H = h
	h.At.Next.Note(h.N)
	fmt.Println(h.At.Next.V, len(h.At.Next.Seen), h.At.Next.Seen[0], h.At.Prev.Next == h.At, h.At.H.N)
}
