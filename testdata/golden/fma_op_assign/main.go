// Command fma_op_assign accumulates float products with += and -=, which
// gc fuses into a multiply-add on arm64 and from GOAMD64=v3 (issue #223):
// patina mode writes x = the same fused operation, so the result is Go's
// bit for bit on every target.
package main

import "fmt"

type acc struct {
	total float64
}

func main() {
	count, price := []int{2, 3, 7}, []float64{1.1, 0.3, 2.9}
	worth := 0.0
	for i := range count {
		worth += float64(count[i]) * price[i]
	}
	fmt.Println(worth)

	drift := 10.0
	for _, p := range price {
		drift -= p * 0.7
	}
	fmt.Println(drift)

	sums := []float64{0, 0}
	for i, p := range price {
		sums[i%2] += p * p
	}
	a := acc{}
	a.total += price[0] * price[2]
	fmt.Println(sums, a.total)
}
