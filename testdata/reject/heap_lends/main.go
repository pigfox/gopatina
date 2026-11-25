// A place held through a pointer that escapes is lent to a call inside
// the cell's one borrow (RFC 0013), so the call must not reach a cell.
package main

import "fmt"

type T struct {
	items []int
	words []string
	in    In
	bag   Bag
}

type Bag struct{ s []int }

func (b Bag) Sum() int { return len(b.s) }

type E struct {
	err error
	n   int
}

func getE() *E { return &E{n: 1} }

func fill(s *[]int) { (*s)[0] = 7 }

type In struct{ n int }

func (i *In) Inc() { i.n++ }

type L struct{ kids []L }

type Wrapper struct{ t *T }

func (w Wrapper) Use(s []int) { s[0] = 1 }

func get() *T { return &T{items: []int{1}, words: []string{"a"}} }

func two(a []int, b []string) {
	a[0] = 1
	b[0] = "x"
}

func reach(s []int, q *T) { s[0] = len(q.items) }

func withWrapper(s []int, w Wrapper) { s[0] = 2 }

func tree(s []int, l L) { s[0] = len(l.kids) }

func grow(i *In) { i.n++ }

func setBoth(a, b []int) {
	a[0] = 1
	b[0] = 2
}

func main() {
	p, q := get(), get()
	two(p.items, p.words)
	reach(p.items, q)
	withWrapper(p.items, Wrapper{t: q})
	Wrapper{t: q}.Use(p.items)
	lists := [][]int{{1}}
	setBoth(p.items, lists[len(q.items)-1])
	// Lent to calls that reach no cell: no diagnostics.
	tree(p.items, L{})
	grow(&p.in)
	(&p.in).Inc()
	fill(&p.items)
	e := *getE()
	var held any = get() // an interface holds a *T: its pointers are cells (RFC 0013)
	fmt.Println(p.items, p.in.n, p.bag.Sum(), e.n, held != nil)
}
