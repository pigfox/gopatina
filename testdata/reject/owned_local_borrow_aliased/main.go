// Command owned_local_borrow_aliased uses a variable while a pointer to it,
// a local borrow (p := &x), is still in use. In Go both names reach one
// value; in Rust a &mut x is the only way to x while it lives, so reading
// or changing x before the pointer's last use is refused (RFC 0017).
// These are the parts of pointers_alias and pointers_more that did so.
package main

import "fmt"

func bump(p *int) int {
	*p++
	return *p
}

func pairOf(v int) (int, int) { return v, v + 1 }

func addr(v int) *int { return &v }

func main() {
	_ = *addr(0) // a *int that escapes: its pointers are owned

	n := 1
	m := &n
	r := bump(m) + n // n read while m, which changes it, is still in use
	fmt.Println(r, n, *m+bump(m), n)

	k, extra := pairOf(1)
	cell := &k
	k, extra = pairOf(2) // k changed while cell still reads it
	fmt.Println(*cell, extra)

	k, fresh := pairOf(3)
	cell2 := &fresh
	*cell2 += k
	fmt.Println(k, fresh, *cell2) // fresh read beside cell2, which changes it

}
