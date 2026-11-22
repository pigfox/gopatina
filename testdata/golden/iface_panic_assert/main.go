// Command iface_panic_assert asserts an interface value to the wrong
// concrete type: Go panics naming both types (RFC 0012).
package main

import "fmt"

type Shape interface{ Area() float64 }

type Sq struct{ S float64 }

func (s Sq) Area() float64 { return s.S * s.S }

type Circle struct{ R float64 }

func (c Circle) Area() float64 { return 3 * c.R * c.R }

func main() {
	var s Shape = Sq{S: 2}
	fmt.Println(s.(Sq).S)
	fmt.Println(s.(Circle).R)
}
