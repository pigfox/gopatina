// Command sieve counts the primes below a bound with the sieve of
// Eratosthenes: a large slice, nested loops and element writes. It is a
// benchmark case (bench/README.md).
package main

import "fmt"

func countPrimes(n int) int {
	composite := make([]bool, n)
	count := 0
	for i := 2; i < n; i++ {
		if composite[i] {
			continue
		}
		count++
		for j := i * i; j < n; j += i {
			composite[j] = true
		}
	}
	return count
}

func main() {
	fmt.Println(countPrimes(20000000))
}
