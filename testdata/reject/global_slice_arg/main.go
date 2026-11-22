// A package-level slice that nothing changes (RFC 0010) passed to a
// function that changes its argument: every reader shares its elements.
package main

import "fmt"

var primes = []int{2, 3, 5}

func sum(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

func double(xs []int) {
	for i := range xs {
		xs[i] *= 2
	}
}

func main() {
	fmt.Println(sum(primes))
	double(primes)
	fmt.Println(primes)
}
