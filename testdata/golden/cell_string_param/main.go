// A string parameter whose address escapes (RFC 0013): with one owner per
// object it is a String of its own, moved out of the function that took
// its address (RFC 0017). Two such pointers are compared by address.
package main

import "fmt"

type Name string

func newName(n Name) *Name { return &n }

func label(s string) *string { return &s }

func main() {
	n := newName("go")
	*n += "pher"
	l := label("x")
	*l = *l + string(*n)
	k := label("x")
	fmt.Println(*n, *l, l == k, *k, len(*l))
}
