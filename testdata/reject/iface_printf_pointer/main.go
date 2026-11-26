// %v of an interface value that can hold a pointer without a String or
// Error method (found by gopatina-parity in the Go tour's
// interface-values): Go's fmt prints &{Hello} for it, which patina mode
// does not print yet, so it is refused as Println's operand is.
package main

import "fmt"

type Shower interface{ Show() }

type T struct{ S string }

func (t *T) Show() { fmt.Println(t.S) }

func describe(s Shower) {
	fmt.Printf("(%v, %T)\n", s, s)
}

func main() {
	describe(&T{S: "Hello"})
}
