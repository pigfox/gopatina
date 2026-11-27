// Command defer_panic panics in a function that defers prints, called
// from main, which defers too: Go runs every deferred call, innermost
// first, before it prints the panic and exits with status 2.
package main

import "fmt"

func check(xs []int, i int) int {
	defer fmt.Println("check deferred", i)
	return xs[i]
}

func main() {
	defer fmt.Println("main deferred")
	xs := []int{1, 2, 3}
	fmt.Println(check(xs, 1))
	fmt.Println(check(xs, 5))
	fmt.Println("not reached")
}
