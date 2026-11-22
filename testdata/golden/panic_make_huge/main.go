// make with a length whose elements exceed Go's largest allocation panics
// before allocating, the way Go does.
package main

import "fmt"

func size(k int) int { return k - 5 }

func main() {
	fmt.Println("before")
	s := make([]int, size(1<<46))
	fmt.Println(len(s))
}
