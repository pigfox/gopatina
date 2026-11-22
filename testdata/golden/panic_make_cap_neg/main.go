// make with a zero length and a negative capacity panics the way Go does.
package main

import "fmt"

func size(k int) int { return k - 5 }

func main() {
	fmt.Println("before")
	b := make([]byte, 0, size(1))
	fmt.Println(len(b))
}
