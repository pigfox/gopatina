// Command arith exercises integer and floating-point arithmetic, including
// Go's wrapping overflow and its rules for division.
package main

import "fmt"

const (
	Big   = 1 << 40
	Small = -7
	Pi    = 3.14159
)

func divmod(a, b int) (int, int) {
	return a / b, a % b
}

func main() {
	x := 9223372036854775807
	x++
	fmt.Println("wrapped:", x)

	var b uint8 = 250
	b += 10
	fmt.Println("byte:", b)

	q, r := divmod(Small, 2)
	fmt.Println("divmod:", q, r)
	n := -9
	fmt.Println(n/4, n%4, n/-4, n%-4)

	minInt := -9223372036854775808
	d := -1
	fmt.Println(minInt/d, minInt%d)

	fmt.Println(Big, Big*2, Small)
	fmt.Println(5&3, 5|3, 5^3, 5&^3, ^5, -5>>1, 1<<10)

	var i32 int32 = 2147483647
	i32 += 2
	fmt.Println("int32:", i32)

	f := 1.0
	for i := 0; i < 5; i++ {
		f = f * 10.5
	}
	fmt.Println(f, f/3, -f, 0.1+0.2)
	ff := 3.99
	fmt.Println(Pi*2, float64(7)/2, int(ff), int(-ff))
	fmt.Println(1e6, 123456.0, 1234567.0, 0.0001, 0.00001, 1e21)

	var g float64
	fmt.Println(g, g == 0)

	m := int64(1000)
	k := int(m) * 3
	fmt.Println(k, uint8(k), int32(k))
}
