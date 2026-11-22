// i*i*i fits in an int for i <= 2097151, so the nest is specialized for
// n <= 2097152 (RFC 0006). This program runs with n = 2097153, one more, so the general copy runs: i*i*i wraps to a negative index and Go panics.
package main

import "fmt"

func cubes(n int) int {
	marked := make([]bool, n)
	count := 0
	for i := 1; i < n; i++ {
		for j := i * i * i; j < n; j += i {
			if !marked[j] {
				marked[j] = true
				count++
			}
		}
	}
	return count
}

func main() {
	fmt.Println(cubes(2097153))
}
