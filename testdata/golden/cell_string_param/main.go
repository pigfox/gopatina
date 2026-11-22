// A string parameter whose address escapes lives in a cell (RFC 0013); the
// parameter borrows a &str, and its cell holds a String of its own.
package main

import "fmt"

type Name string

func newName(n Name) *Name { return &n }

func label(s string) *string { return &s }

func main() {
	n := newName("go")
	m := n
	*m += "pher"
	l := label("x")
	*l = *l + string(*n)
	fmt.Println(*n, *m, n == m, *l, len(*l))
}
