// A package-level variable's initializer that panics (RFC 0010): the
// variables before it are initialized, in Go's order, and the program ends
// with Go's message and status before init and main run.
package main

import "fmt"

var first = say("first")

var table = []int{1, 2}

var bad = table[pick()]

func say(s string) string {
	fmt.Println("initializing", s)
	return s
}

func pick() int {
	fmt.Println("picking")
	return len(first) + 1
}

func init() { fmt.Println("init, not reached") }

func main() { fmt.Println("main, not reached", first, bad) }
