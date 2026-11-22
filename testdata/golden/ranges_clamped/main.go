// A bound with no fact about its sign is copied as n.max(0) as usize,
// which is exact only for i < n with i >= 0 (RFC 0006); n may be negative.
package main

import "fmt"

func fill(s []int, n int) int {
	sum := 0
	for i := 0; i < n; i++ {
		s[i] = i * 3
		sum += s[i]
	}
	return sum
}

func main() {
	s := make([]int, 5)
	fmt.Println(fill(s, 5), fill(s, 0), fill(s, -3), s)
}
