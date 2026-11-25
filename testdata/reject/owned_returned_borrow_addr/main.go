// Command owned_returned_borrow_addr keeps, in a local, the borrow a
// function lends back of a variable's address (keepC := hold(&c)), and
// changes c by name while keepC is still in use, as pointers_cells did
// while c lived in a shared cell (RFC 0013). keepC borrows c, so c cannot
// change until keepC's last use: refused (RFC 0017). Used one after the
// other, the two compile (testdata/golden/pointers_cells).
package main

import "fmt"

// Counter counts hits.
type Counter struct{ hits int }

func hold(c *Counter) *Counter { return c }

func (c *Counter) Hit() int {
	c.hits++
	return c.hits
}

func main() {
	var c Counter
	keepC := hold(&c)
	c.Hit()
	keepC.Hit()
	fmt.Println(c.hits, keepC.hits)
}
