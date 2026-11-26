// Command iface_private_structs keeps unexported struct types in an
// interface and asserts them back out: Dyn, which is public, holds
// private Rust structs, so the crate allows private_interfaces.
package main

import "fmt"

type shape interface {
	area() float64
}

type rect struct {
	w, h float64
}

func (r rect) area() float64 { return r.w * r.h }

type square struct {
	side float64
}

func (s square) area() float64 { return s.side * s.side }

func describe(s shape) {
	if r, ok := s.(rect); ok {
		fmt.Println("rect", r.w, r.h, s.area())
		return
	}
	fmt.Println("other", s.area())
}

func main() {
	describe(rect{w: 3, h: 4})
	describe(square{side: 2})
}
