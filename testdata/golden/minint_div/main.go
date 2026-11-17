package main

import "fmt"

func div(a, b int64) (int64, int64) {
	return a / b, a % b
}

func main() {
	var m int64 = -9223372036854775808
	q, r := div(m, -1)
	fmt.Println(q, r)
	var n int32 = -2147483648
	var k int32 = -1
	fmt.Println(n/k, n%k)
	x, y := -7, 2
	fmt.Println(x/y, x%y, x/-y, x%-y)
	fmt.Printf("%d %d\n", -7/2, -7%2)
}
