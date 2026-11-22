// Command iface_panic_uncomparable compares two interface values holding
// slices of the same type: Go panics (RFC 0012). Values of different
// dynamic types compare unequal without panicking.
package main

import (
	"errors"
	"fmt"
)

type Path []int

func (p Path) Error() string { return "path" }

func main() {
	var a, b any = Path{1}, []int{1}
	var c any = "x"
	fmt.Println(a == b, a == c, b != c)
	var e error = Path{2}
	fmt.Println(errors.Is(e, e), errors.Is(e, errors.New("x")))
	fmt.Println(a == a)
}
