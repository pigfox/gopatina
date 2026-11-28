// Command panic_named_string panics with a value of a named string type,
// which Go's runtime prints as the type and the string: main.state("idle").
package main

import "fmt"

type state string

func main() {
	s := state("idle")
	fmt.Println("state", s)
	panic(s)
}
