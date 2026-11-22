package c

import "fmt"

func init() { fmt.Println("init c") }

// New is the third New; String is a Rust name too.
func New() int { return String() }

func String() int { return 42 }
