// Constant bounds: specialized with no condition and no usize copies
// (RFC 0006). Induction variables used as Go ints are converted back.
package main

import "fmt"

func main() {
	s := make([]int, 64)
	for i := 0; i < 8; i++ {
		for j := 0; j <= 7; j++ {
			s[i*8+j] = i - j
		}
	}
	total := 0
	for k := 0; k < 64; k += 9 {
		total += s[k] * k
	}
	fmt.Println(s[9], s[63], total)
}
