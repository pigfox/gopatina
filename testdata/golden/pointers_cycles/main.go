// Types through which a cycle of pointers can form (RFC 0013): -explain
// notes each field of one, since a cycle is not reclaimed. With one owner
// per object (RFC 0017) none closes here: a program that closes one is
// refused (testdata/reject/owned_graph_cycle, owned_aliases_kept).
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

// Outer can reach itself through a struct value, and through a Link. A
// struct value holding a pointer is not owned yet, so Outer's pointers are
// cells.
type Inner struct{ Back *Outer }

type Outer struct {
	In   Inner
	N    int
	Seen []int
	Peer *Link
}

// Note clears the last value seen, written through the Outer's cell.
func (o *Outer) Note(v int) {
	o.Seen = append(o.Seen, v)
	o.Seen[len(o.Seen)-1] = 0
}

// Link leads back to an Outer.
type Link struct{ O *Outer }

// Item never leads back to Owner, and Chain only to itself: no note on
// Owner.Item or Start.Head.
type Item struct{ V int }

type Owner struct{ Item *Item }

type Chain struct{ Next *Chain }

type Start struct{ Head *Chain }

// Ring could close into a ring through Next and Prev; here it is a chain
// through Next, each node owned by the one before it.
type Ring struct {
	V          int
	Prev, Next *Ring
	Seen       []int
	H          *Hand
}

// Note clears the last value seen, written through the node.
func (r *Ring) Note(v int) {
	r.Seen = append(r.Seen, v)
	r.Seen[len(r.Seen)-1] = 0
}

// Hand holds a place on a ring, and a ring node can hold a Hand: Ring.H
// can lead back to a Ring through Hand.At.
type Hand struct {
	At *Ring
	N  int
}

func ring(n int) *Ring {
	var first *Ring
	for i := n - 1; i >= 0; i-- {
		first = &Ring{V: i, Next: first}
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
	o.In.Back = &Outer{N: 2}
	o.In.Back.N = 3
	o.In.Back.Note(4)
	o.Peer = &Link{}
	fmt.Println(o.In.Back.N, o.In.Back.In.Back == nil, o.In.Back == o, len(o.In.Back.Seen), o.Peer.O == nil)
	w := Owner{Item: &Item{V: 4}}
	s := Start{Head: &Chain{Next: &Chain{}}}
	fmt.Println(w.Item.V, s.Head.Next.Next == nil)
	sum := 0
	for i := 0; i < 3; i++ {
		r := ring(5)
		for p := r.Next; p != nil; p = p.Next {
			sum += p.V + r.V
		}
	}
	fmt.Println(sum)
	h := &Hand{At: ring(3), N: 1}
	h.At.H = &Hand{N: 2}
	h.At.Next.Note(h.N)
	fmt.Println(h.At.Next.V, len(h.At.Next.Seen), h.At.Next.Seen[0], h.At.Next.Next.Next == nil, h.At.Prev == nil, h.At.H.N)
}
