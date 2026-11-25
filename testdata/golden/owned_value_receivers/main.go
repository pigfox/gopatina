// Command owned_value_receivers calls methods with value receivers through
// owned pointers to types that hold a slice (RFC 0017). Go's copy of such
// a value shares the slice with the value in the box, as a Rust borrow
// does: a method that only reads is &self, one that changes the slice's
// elements is &mut self, never a clone.
package main

import "fmt"

// Stock holds a slice and a count.
type Stock struct {
	items []int
	n     int
}

// Total reads its copy: &self.
func (s Stock) Total() int {
	t := s.n
	for _, x := range s.items {
		t += x
	}
	return t
}

// Clear changes the elements its copy shares: &mut self.
func (s Stock) Clear() {
	for i := range s.items {
		s.items[i] = 0
	}
}

// Nums is a named slice.
type Nums []int

// Sum reads its copy.
func (ns Nums) Sum() int {
	t := 0
	for _, x := range ns {
		t += x
	}
	return t
}

// Set changes the first element its copy shares.
func (ns Nums) Set(v int) { ns[0] = v }

func newStock(n int) *Stock { return &Stock{items: []int{n, n + 1}, n: n} }

func newNums() *Nums {
	ns := Nums{1, 2, 3}
	return &ns
}

func main() {
	s := newStock(4)
	fmt.Println(s.Total())
	s.Clear()
	fmt.Println(s.Total(), s.items)
	var m *Stock // may be nil: an Option
	if s.n > 0 {
		m = newStock(7)
	}
	m.Clear()
	fmt.Println(m.Total(), m.items)
	ns := newNums()
	ns.Set(9)
	fmt.Println(ns.Sum(), *ns, len(*ns))
}
