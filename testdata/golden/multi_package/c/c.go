package c

import "fmt"

func init() { fmt.Println("init c") }

// New is the third New; String is a Rust name too.
func New() int { return String() }

func String() int { return 42 }

// trace prints as a package-level variable initializes.
func trace(s string) int {
	fmt.Println("init var", s)
	return len(s)
}

// Registered is never used by the program, but Go initializes it, so its
// initializer still prints; table, which only holds constants, is left out.
var Registered = trace("registered")

var table = []int{1, 2, 3}
