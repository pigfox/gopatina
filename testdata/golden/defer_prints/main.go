// Command defer_prints defers prints: their arguments are evaluated at
// the defer statement, and they run last first when the function returns,
// after its result is computed, also from a loop.
package main

import "fmt"

func sum(xs []int) int {
	total := 0
	defer fmt.Println("sum done")
	for i, x := range xs {
		defer fmt.Printf("item %d is %d\n", i, x)
		total += x
	}
	return total
}

func label(n int) string {
	defer fmt.Print("label ", n, " deferred\n")
	if n < 0 {
		return "negative"
	}
	n *= 2 // the deferred print already has n
	return fmt.Sprint("doubled ", n)
}

func main() {
	x := 1
	defer fmt.Println("main done, x was", x, true, 2.5)
	x = 2
	fmt.Println("sum", sum([]int{3, 4, 5}))
	fmt.Println(label(4), label(-1))
	fmt.Println("x is", x)
}
