// Command iface_panic_nil_call calls a method through a nil interface:
// Go panics with a nil pointer dereference (RFC 0012).
package main

import "fmt"

type Shape interface{ Area() float64 }

type Sq struct{ S float64 }

func (s Sq) Area() float64 { return s.S * s.S }

func pick(ok bool) Shape {
	if ok {
		return Sq{S: 2}
	}
	return nil
}

func main() {
	fmt.Println(pick(true).Area())
	fmt.Println(pick(false).Area())
}
