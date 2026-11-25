// Command owned_struct_value_cycle stores an Outer in a field of a struct
// value inside itself: o.In.Back = o, a cycle, as pointers_cycles did
// while Outer's pointers were shared cells (RFC 0013). A struct value now
// owns the pointers in its fields, so o moves into o.In.Back and is used
// after: refused (RFC 0017).
package main

import "fmt"

// Inner holds an Outer.
type Inner struct{ Back *Outer }

// Outer holds an Inner by value.
type Outer struct {
	In Inner
	N  int
}

func main() {
	o := &Outer{N: 1}
	o.In.Back = o
	fmt.Println(o.In.Back.N)
}
