// Command suffix-sum adds a checksum over the tail of a large []int through
// a read-only suffix view. It exercises read(s[i:]) in the patina subset;
// it is a benchmark case (bench/README.md).
package main

import "fmt"

func checksum(s []int) {
	var total int
	for _, v := range s {
		total = total*33 + v
	}
	fmt.Println(total)
}

func main() {
	const n = 20_000_000
	const start = 1_000_000
	s := make([]int, n)
	for i := 0; i < n; i++ {
		s[i] = i%101 - 50
	}
	checksum(s[start:])
}
