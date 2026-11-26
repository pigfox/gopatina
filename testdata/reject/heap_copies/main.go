// Values that share their elements are not copied out of a pointer that
// escapes (RFC 0013): a copy would not share them.
package main

import (
	"fmt"
	"slices"
	"strings"
)

type In struct{ s []int }

func (i In) Clear() { i.s = nil }

type T struct {
	items []int
	words []string
	in    In
	name  string
	n     int
}

func (t T) Bump() {
	t.n++
	t.items[0] = 1
}

func get(n int) *T { return &T{n: n, items: []int{1}} }

func keepIt(s []int) []int { return s }

func counter() *int {
	n := 0
	return &n
}

func items(p *T) []int {
	return p.items
}

func f() int { return 1 }

func main() {
	p := get(1)
	s := p.items
	fmt.Println(s, items(p))
	keepIt(p.items)
	for range p.items {
	}
	_ = []int(p.items)
	p.Bump()
	p.in.Clear()
	if p.n > f() {
	}
	for p.n < f() {
	}
	switch p.n + f() {
	}
	if c := counter(); *c > f() {
	}
	// Read through the box: no diagnostics.
	fmt.Println(p.items, len(p.items), (p.items)[0], p.name[0], slices.Clone(p.items), strings.Join(p.words, ","))
	p.items = append(p.items, 2)
	p.items[0]++          // so a range over a []int in a cell would not see Go's elements
	var held any = get(2) // an interface owns the *T it holds (RFC 0017)
	fmt.Println(held != nil)
}
