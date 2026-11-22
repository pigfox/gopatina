// make with a length larger than its capacity panics the way Go does.
package main

import "fmt"

func size(k int) int { return k - 5 }

func main() {
	fmt.Println("before")
	s := make([]int, 5, size(7))
	fmt.Println(len(s))
}
