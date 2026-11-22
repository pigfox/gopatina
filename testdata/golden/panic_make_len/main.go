// make with a negative length panics the way Go's makeslice does.
package main

import "fmt"

func size(k int) int { return k - 5 }

func main() {
	fmt.Println("before")
	s := make([]int, size(2))
	fmt.Println(len(s))
}
