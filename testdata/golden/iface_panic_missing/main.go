// Command iface_panic_missing asserts a value to an interface its type does
// not implement: Go panics naming the first missing method (RFC 0012).
package main

import "fmt"

type Named interface {
	Name() string
	Age() int
}

type Sq struct{ S float64 }

func (s Sq) Name() string { return "sq" }

func main() {
	var a any = Sq{S: 2}
	var b any = 3
	_, ok := b.(Named)
	fmt.Println(ok)
	fmt.Println(a.(Named).Name())
}
