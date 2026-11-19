package main

import "fmt"

// A program's own panic text can look exactly like Rust's runtime message.
// The runtime hook must print it verbatim, not translate it.
func main() {
	fmt.Println("before")
	panic("index out of bounds: the len is 3 but the index is 9")
}
