// Command owned_returned_borrow_kept keeps the borrow a function lends
// back beside the object it borrows, as pointers_cells did while such a
// pointer was a shared cell (RFC 0013): c is changed by name while keepC,
// a borrow of it, is still in use, and a borrow is stored in a struct.
// One owner per object refuses both (RFC 0017).
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
	var c Counter
	keepC := hold(&c)
	c.Hit()
	keepC.Hit()
	fmt.Println(c.hits, keepC.hits)

	d := &Counter{}
	p := Pair{A: hold(d), B: &Counter{}}
	p.A.Hit()
	fmt.Println(d.hits, p.A.hits, p.B.hits)
}
