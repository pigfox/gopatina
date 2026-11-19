// Command minmax exercises Go 1.21's min and max built-ins with integers.
package main

import "fmt"

func main() {
	a, b := 7, -3
	fmt.Println(min(a, b), max(a, b))
	fmt.Println(min(a, 2, 9, -1), max(a, 2, 9, -1))
	fmt.Println(min(min(a, b), max(a, b)), max(min(a, b), max(a, b)))

	var c int32 = -9
	fmt.Println(min(c, 4), max(c, 4))

	var d uint8 = 250
	fmt.Println(min(d, 3), max(d, 3))

	fmt.Println(min(0, 1), max(0, 1))
	fmt.Println(min(a, b)*2 + max(a, b)/2)
}
