// Command owned_kept_addr gives the address of a local value to a function
// that keeps it, and uses the value no more: the value moves into a new
// Box at the &, which the function keeps (RFC 0017), as
// keep_in(&mut h, Some(Box::new(tt))). An address given to a function that
// only lends it is a borrow instead.
package main

import "fmt"

// T holds a slice and a count.
type T struct {
	items []int
	n     int
}

// Holder keeps a T.
type Holder struct {
	t    *T
	more []*T
}

func keepIn(h *Holder, t *T) { h.t = t }

func add(h *Holder, t *T) { h.more = append(h.more, t) }

func peek(t *T) int { return t.n }

func main() {
	h := &Holder{}
	tt := T{items: []int{1, 2}, n: 3}
	tt.n++
	keepIn(h, &tt) // tt moves into a new Box, which h keeps
	var u T
	u.n = 7
	u.n++
	fmt.Println(peek(&u), u.n) // lent, not kept: a borrow
	for i := 0; i < 2; i++ {
		v := T{n: i}
		add(h, &v) // a new v each time round, moved each time
	}
	fmt.Println(h.t.n, h.t.items, len(h.more), h.more[1].n)
}
