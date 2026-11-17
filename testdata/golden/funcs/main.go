// Command funcs exercises functions: multiple and named results,
// recursion, and parallel assignment.
package main

import "fmt"

func fib(n int) int {
	if n < 2 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

func minMax(xs []int) (lo, hi int) {
	lo, hi = xs[0], xs[0]
	for _, x := range xs {
		if x < lo {
			lo = x
		}
		if x > hi {
			hi = x
		}
	}
	return
}

func split(full string) (string, string, int) {
	return full, full + full, len(full)
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func main() {
	for i := 0; i < 10; i++ {
		fmt.Print(fib(i), " ")
	}
	fmt.Println()
	lo, hi := minMax([]int{4, -2, 9, 3})
	fmt.Println(lo, hi)
	a, b, n := split("ab")
	fmt.Println(a, b, n)
	x, y := 1, 2
	x, y = y, x
	fmt.Println(x, y, gcd(84, 36))
	s, t := "left", "right"
	s, t = t, s
	fmt.Println(s, t)
	_, second, _ := split("z")
	fmt.Println(second)
}
