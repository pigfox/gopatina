// Command owned_aliases_compared keeps a second pointer to an object beside
// its first and compares the two, as pointers_alias, pointers_heap and
// cell_string_param and pointers_cells did while comparing pointers shared them (RFC 0013).
// Comparing is now a read by address (std::ptr::eq), so these are what
// they are, two owners of one object, and refused (RFC 0017).
package main

import "fmt"

// Counter counts.
type Counter struct{ N int }

// Inc counts one.
func (c *Counter) Inc() { c.N++ }

func newCounter() *Counter { return &Counter{} }

func addr(x float64) *float64 { return &x }

// Stack holds a pointer to another Stack.
type Stack struct {
	n   int
	top *Stack
}

func main() {
	c := newCounter()
	d := c // a copy of c: the Counter moves to d
	d.Inc()
	c.Inc()
	fmt.Println(c.N, d.N, c == d)

	x := addr(5)
	y := x // a copy of x: the float64 moves to y
	*y += *x
	fmt.Println(*x, *y, x == y)

	s := &Stack{n: 1}
	s.top = s // a Stack owning itself
	fmt.Println(s.top.n, s.top == s)
}
