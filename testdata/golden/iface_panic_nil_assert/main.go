// Command iface_panic_nil_assert asserts a nil interface to a concrete
// type: Go panics saying the interface is nil (RFC 0012).
package main

import "fmt"

type Shape interface{ Area() float64 }

type Other interface{ Area() float64 }

type Sq struct{ S float64 }

func (s Sq) Area() float64 { return s.S * s.S }

func main() {
	var s Shape
	sq, ok := s.(Sq)
	fmt.Println(sq.S, ok, s == nil)
	o, ok2 := s.(Other)
	fmt.Println(o == nil, ok2)
	fmt.Println(s.(Sq).S)
}
