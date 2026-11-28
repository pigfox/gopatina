// Command closures_recursive writes recursive closures (RFC 0018, step 4):
// var f func(...); f = func(...) {...}, a literal that calls itself through
// its variable and uses nothing else of its enclosing function, is a
// nested Rust fn.
package main

import "fmt"

func apply(f func(int) int, x int) int {
	return f(x)
}

func main() {
	var fib func(n int) int
	fib = func(n int) int {
		if n < 2 {
			return n
		}
		return fib(n-1) + fib(n-2)
	}
	fmt.Println(fib(7), fib(20))

	var fact func(int) int
	fact = func(n int) int {
		if n <= 1 {
			return 1
		}
		return n * fact(n-1)
	}
	fmt.Println(fact(10), apply(fact, 5))

	var even func(int) bool
	even = func(n int) bool {
		if n == 0 {
			return true
		}
		if n == 1 {
			return false
		}
		return even(n - 2)
	}
	fmt.Println(even(10), even(7))

	var halve func(float64, int) float64
	halve = func(x float64, times int) float64 {
		if times == 0 {
			return x
		}
		return halve(x/2, times-1)
	}
	fmt.Println(halve(100, 3))
}
