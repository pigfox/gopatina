// Command iface_panic_uncomparable compares two interface values holding
// slices of the same type: Go panics (RFC 0012). Values of different
// dynamic types compare unequal without panicking.
package main

import "fmt"

type Path []int

func main() {
	var a, b any = Path{1}, []int{1}
	var c any = "x"
	fmt.Println(a == b, a == c, b != c)
	fmt.Println(a == a)
}
