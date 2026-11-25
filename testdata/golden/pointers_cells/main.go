// Variables whose address escapes live in cells (RFC 0013), and a field or
// slice inside a cell is lent to a call inside the cell's one borrow.
package main

import "fmt"

type In struct{ n int }

func (i *In) Inc() { i.n++ }

// Get reads a copy.
func (i In) Get() int { return i.n + 1 }

// T's pointers are cells: Total and Zero take a copy of a T, which shares
// its items with the T in the box.
type T struct {
	in    In
	items []int
	name  string
	n     int
	bag   Bag
}

// Bag holds a slice: a copy of one shares it.
type Bag struct{ s []int }

// Sum reads a copy.
func (b Bag) Sum() int { return sum(b.s) }

// two returns two ints.
func two() (int, int) { return 6, 7 }

func (t T) Total() int { return sum(t.items) + t.n }

func (t T) Zero() { t.items[0] = 0 }

func sum(s []int) int {
	total := 0
	for _, x := range s {
		total += x
	}
	return total
}

func set(s []int, v int) { s[len(s)-1] = v }

func inc(x *int) { *x++ }

func keep(n int) *T { return &T{items: []int{1, 2, 3}, n: n, bag: Bag{s: []int{n, n}}} }

func addr(x float64) *float64 { return &x }

func pair() (*string, *string) {
	a, b := "a", "b"
	return &a, &b
}

type Counter struct{ hits int }

func hold(c *Counter) *Counter { return c }

func (c *Counter) Hit() int {
	c.hits++
	return c.hits
}

// Tag is a named string whose pointers are shared: an interface holds one.
type Tag string

// Len reads a copy.
func (t Tag) Len() int { return len(t) }

// Up changes its copy and returns it.
func (t Tag) Up() Tag { return t + "!" }

func newTag(s string) *Tag {
	t := Tag(s)
	return &t
}

// twice returns a tag and its length.
func twice(s string) (Tag, int) { return Tag(s + s), 2 * len(s) }

// Shelf is a struct whose pointers are shared: an interface holds one.
type Shelf struct{ N int }

// Count reads a copy.
func (s Shelf) Count() int { return s.N }

// Bump changes the shelf.
func (s *Shelf) Bump() { s.N++ }

// Keep returns its receiver.
func (s *Shelf) Keep() *Shelf { return s }

// show reads through the pointer.
func show(s *Shelf) int { return s.N }

// both returns two tags.
func both(s string) (Tag, Tag) { return Tag(s), Tag(s + "!") }

func main() {
	kv, ke := twice("a")
	var anyK any = &kv // an interface holds a *Tag: its pointers are cells
	kv, ke = twice("b")
	fmt.Println(kv, ke, anyK != nil)
	kv, kf := twice("c") // kv an existing cell, kf new
	kv, kw := both("d")  // kw new, and a cell too
	var anyW any = &kw
	fmt.Println(kv, kf, kw, anyW != nil)
	var shelf Shelf
	var anyS any = &shelf
	sp := &shelf
	(&shelf).Bump()
	kept := (&shelf).Keep()
	kept.Bump()
	fmt.Println(sp.Count(), show(&shelf), anyS != nil)
	ta := newTag("ab")
	var held any = newTag("in an interface")
	*ta += "c"
	fmt.Println(ta.Len(), ta.Up(), *ta, held != nil)
	p := keep(4)
	p.in.Inc()
	p.in.Inc()
	inc(&p.n)
	set(p.items, 30)
	fmt.Println(p.in.n, p.n, p.items, sum(p.items), p.Total(), len(p.items))
	p.Zero()
	p.items = append(p.items, 7, 8)
	p.name += "ab"
	p.name = p.name + p.name
	p.n *= 3
	p.n -= p.n / 4
	fmt.Println(p.items, p.name, p.n)
	pq := keep(5)
	pq.name = "abab"
	p.n, _ = two()
	set(p.items, p.n) // p.n read inside the cell's borrow, before the lend
	fmt.Println(p.name == pq.name, p.in.Get(), p.bag.Sum(), p.n, p.items)
	x := addr(5)
	y := addr(5)
	*y += *x
	fmt.Println(*x, *y, x == y, addr(1) == addr(1), x == x)
	a, b := pair()
	*a, *b = *b, *a
	fmt.Println(*a, *b)
	c := &Counter{}
	c.Hit()
	keepC := hold(c) // a borrow of c, used before c is again
	keepC.Hit()
	fmt.Println(keepC.Hit(), hold(keepC).hits)
	fmt.Println(c.hits)
	two := c.Hit() + c.Hit()
	fmt.Println(two, c.hits)
	cells := []int{c.hits, two, c.hits * 2}
	fmt.Println(cells)
	var q *T
	if q == nil {
		q = keep(1)
	}
	q.items[1] += 40
	q.in.n--
	fmt.Println(q.items[1], q.in.n, q != p)
}
