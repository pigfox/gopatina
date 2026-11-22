// A declaration outside the subset in another package of the program,
// reached from main, is reported where it is, with how main reaches it.
package main

import (
	"fmt"

	"example.com/reach/a"
)

func main() { fmt.Println(a.Count("x")) }
