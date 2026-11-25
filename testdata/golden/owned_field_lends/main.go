// Command owned_field_lends lends the fields of owned pointers to calls
// (RFC 0017): &p.n to a function and (&p.in).Inc() to a method, as Rust's
// &mut p.n and p.in.inc(), whether p is never nil (a Box) or may be (an
// Option<Box<T>>, unwrapped with as_mut). A read through the same pointer
// in another argument is made before the lend, as Go evaluates it.
package main

import "fmt"

// In counts.
type In struct{ n int }

func (i *In) Inc() { i.n++ }

// T holds data the box owns.
type T struct {
	in    In
	n     int
	items []int
	name  string
	next  *T
}

func inc(x *int) { *x++ }

func addTo(x *int, d int) { *x += d }

func push(s *[]int, v int) { *s = append(*s, v) }

func addLen(x *int, s string) { *x += len(s) }

func keep(n int) *T { return &T{items: []int{1, 2}, n: n} }

// grow changes the field of its receiver's next T, which may be nil.
func (t *T) grow() {
	if t.next != nil {
		inc(&t.next.n)
		t.next.in.Inc()
	}
}

// both reads t's key and lends t to a call that changes it.
func both(t *T) (int, int) {
	return t.n, bump(t)
}

func bump(t *T) int {
	t.n++
	return t.n
}

func main() {
	p := keep(4)
	inc(&p.n)
	(&p.in).Inc()
	p.in.Inc()
	push(&p.items, 3)
	fmt.Println(p.n, p.in.n, p.items)

	var q *T // may be nil: an Option
	if p.n > 0 {
		q = keep(10)
	}
	q.next = keep(20)
	addTo(&q.n, q.next.n) // q.next.n is read first
	q.name = "four"
	addLen(&q.n, q.name) // and a copy of q.name taken first
	q.grow()
	q.grow()
	fmt.Println(q.n, q.next.n, q.next.in.n)
	k, b := both(q)
	fmt.Println(k, b)
	fmt.Println(q.n)
}
