package main

import "fmt"

// A program's own panic text can look exactly like Rust's runtime message.
// The runtime hook must print it verbatim, not translate it.
func main() {
	fmt.Println("before")
	panic("attempt to divide by zero")
}
