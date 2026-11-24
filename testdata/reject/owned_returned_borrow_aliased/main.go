// Command owned_returned_borrow_aliased uses an owner while an exclusive
// borrow a call returned is still in use: s := c.Self() lends c to s, and
// s is changed through, so in Rust s is the only way to the Counter until
// its last use (RFC 0017, decision 4).
package main

import "fmt"

// Counter counts.
type Counter struct{ Hits int }

// Hit counts one.
func (c *Counter) Hit() { c.Hits++ }

// Self returns its receiver.
func (c *Counter) Self() *Counter { return c }

func newCounter() *Counter { return &Counter{} }

func main() {
	c := newCounter()
	s := c.Self()
	c.Hit() // c used while s, which changes it, is still in use
	s.Hit()
	fmt.Println(c.Hits, s.Hits)
}
