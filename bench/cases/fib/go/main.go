// Command fib computes a Fibonacci number by plain recursion: function
// calls and integer arithmetic. It is a benchmark case (bench/README.md).
package main

import "fmt"

func fib(n int) int {
	if n < 2 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

func main() {
	fmt.Println(fib(35))
}
