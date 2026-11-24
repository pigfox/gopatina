// Command owned_lent_addr keeps a struct value in a local variable and
// lends its address to calls: a method with a pointer receiver, functions
// that read or change through the pointer, and ones that test it for nil.
// No callee keeps the pointer, so the value stays a plain local, lent as
// &c or &mut c (or Some of one where the parameter may be nil), as Rust
// writes it (RFC 0017).
package main

import "fmt"

// Counter counts hits.
type Counter struct {
	Hits int
	Name string
}

// Hit counts one hit.
func (c *Counter) Hit() int {
	c.Hits++
	return c.Hits
}

// newCounter returns a counter that escapes: its pointers are owned.
func newCounter(name string) *Counter { return &Counter{Name: name} }

// peek reads through the pointer.
func peek(c *Counter) int { return c.Hits }

// reset changes through the pointer.
func reset(c *Counter) { c.Hits = 0 }

// hitsOr reads through a pointer that may be nil.
func hitsOr(c *Counter, d int) int {
	if c == nil {
		return d
	}
	return c.Hits
}

// bumpIf changes through a pointer that may be nil.
func bumpIf(c *Counter) {
	if c != nil {
		c.Hits += 100
	}
}

func main() {
	var c Counter
	c.Name = "local"
	c.Hit()
	(&c).Hit()
	fmt.Println(c.Hits, peek(&c), hitsOr(&c, -1), hitsOr(nil, -1))
	bumpIf(&c)
	bumpIf(nil)
	fmt.Println(c.Hits, c.Name)
	reset(&c)
	fmt.Println(peek((&c)), c.Hits)

	d := newCounter("boxed")
	d.Hit()
	fmt.Println(peek(d), d.Name)
}
