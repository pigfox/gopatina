// Shifts by a constant count at least the operand's width, which Go
// defines (x << s and unsigned x >> s are 0, signed x >> s is 0 or -1) and
// Rust's shift operators reject. Found by the generated tests, 2026-11-22.
package main

import "fmt"

func noisy(x int32) int32 {
	fmt.Println("evaluated", x)
	return x
}

func main() {
	a, b := 5, -5
	var c, d int32 = 7, -7
	var e uint8 = 200
	var f int64 = -1
	fmt.Println(a<<64, a<<70, a>>64, b>>64, b>>100, a<<63, b>>63)
	fmt.Println(c<<32, c<<40, c>>32, d>>32, d>>33, c<<31, d>>31)
	fmt.Println(e<<8, e<<9, e>>8, e>>200, e<<7, e>>7)
	fmt.Println(f<<64, f>>64, noisy(3)<<40, noisy(-3)>>40)
	a <<= 64
	b >>= 64
	c <<= 32
	d >>= 40
	e >>= 8
	f <<= 99
	s := []int32{1, -1}
	s[1] >>= 32
	s[0] <<= 33
	fmt.Println(a, b, c, d, e, f, s)
}
