// The sieve: specialized for n <= 3037000500 (RFC 0006), where i*i cannot
// wrap; the general copy keeps Go's checks.
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
	fmt.Println(countPrimes(100), countPrimes(2), countPrimes(0))
}
