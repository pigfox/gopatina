// Command owned_returned_borrow_kept stores the borrow a function lends
// back in a struct, beside the object it borrows: the object has two
// owners, which one owner per object refuses (RFC 0017). A borrow kept in
// a local while its source changes is the borrow checker's error instead
// (testdata/reject/owned_returned_borrow_addr).
package main

import "fmt"

// Counter counts hits.
type Counter struct{ hits int }

func hold(c *Counter) *Counter { return c }

func (c *Counter) Hit() int {
	c.hits++
	return c.hits
}

// Pair holds two counters.
type Pair struct{ A, B *Counter }

func main() {
	d := &Counter{}
	p := Pair{A: hold(d), B: &Counter{}}
	p.A.Hit()
	fmt.Println(d.hits, p.A.hits, p.B.hits)
}
