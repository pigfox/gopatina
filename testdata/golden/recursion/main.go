package main

import "fmt"

func fib(n int) int {
	if n < 2 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

func gcd(a, b int) int {
	if b == 0 {
		return a
	}
	return gcd(b, a%b)
}

func hanoi(n int, from, to, via string, moves *int) {
	if n == 0 {
		return
	}
	hanoi(n-1, from, via, to, moves)
	*moves++
	hanoi(n-1, via, to, from, moves)
}

func collatz(n int, steps int) int {
	if n == 1 {
		return steps
	}
	if n%2 == 0 {
		return collatz(n/2, steps+1)
	}
	return collatz(3*n+1, steps+1)
}

func main() {
	fmt.Println(fib(20), gcd(1071, 462))
	m := 0
	hanoi(10, "a", "c", "b", &m)
	fmt.Println(m, collatz(27, 0))
}
