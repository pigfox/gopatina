// Command closures_defer writes deferred closures (RFC 0018, step 5):
// defer func() {...}() runs the literal when its function returns, after
// the deferred calls that follow it and before those that precede it. It
// owns copies of what it uses, which nothing changes after the defer.
package main

import (
	"fmt"
	"math"
)

func work(n int) int {
	start := n * 2
	defer func() {
		fmt.Println("work done, started at", start)
	}()
	if n > 5 {
		return n - 5
	}
	return n + 1
}

func order() {
	defer fmt.Println("first deferred, runs last")
	x := 3
	defer func() {
		fmt.Printf("closure sees x=%d, sqrt=%.3f\n", x, math.Sqrt(float64(x)))
	}()
	defer fmt.Println("last deferred, runs first")
	fmt.Println("order body")
}

func main() {
	fmt.Println(work(3))
	fmt.Println(work(9))
	order()
	total := 0
	for i := 1; i <= 4; i++ {
		total += i
	}
	ok := total == 10
	defer func() {
		label := "total"
		if !ok {
			label = "wrong"
		}
		fmt.Println(label, total, total/2)
	}()
	fmt.Println("main body")
}
