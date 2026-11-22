// i*j needs a bound on both a and b, two conditions; RFC 0006 allows one,
// so this nest is not specialized, and -explain says why.
package main

import "fmt"

func table(a, b int) int {
	s := make([]int, 100)
	for i := 0; i < a; i++ {
		for j := 0; j < b; j++ {
			s[i*j] += i + j
		}
	}
	return s[4] + s[16]
}

func main() {
	fmt.Println(table(5, 5))
}
