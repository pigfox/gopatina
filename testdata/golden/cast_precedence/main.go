// An operand that ends with a cast needs parentheses before <, <= and <<:
// Rust reads a as i64 < b, and a * b as i64 << 3, as the start of generic
// arguments. Conversions in the Go source, and loop variables written as
// usize (RFC 0006), both produce such casts. -(-x) must not read as --x.
package main

import "fmt"

func main() {
	var b int32 = 3
	x := 5
	s := make([]int, 4)
	for i := 0; i < 4; i++ {
		s[i] = i * i << 27
		if i*i < x {
			x += i
		}
		if i*2 <= x+i {
			x++
		}
		s[(i*i+1)%4] += i >> 1
	}
	y := int64(b) * int64(b) << 3
	z := int64(b)*int64(b) < int64(x)
	w := int64(b) <= int64(x)
	v := -(-x)
	u := -(-(-y))
	fmt.Println(s, x, y, z, w, v, u)
}
