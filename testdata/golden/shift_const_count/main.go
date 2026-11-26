// Command shift_const_count shifts constants by counts known only at run
// time: the constant has the type Go gives it (uint64(1), or the type of
// the variable it initializes), not the i32 Rust would infer.
package main

import "fmt"

func main() {
	x := uint64(88172645463325252)
	var s8 uint8 = 3
	for i := 0; i < 6; i++ {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		b := uint64(1) << (x % 60)
		var c int64 = 1 << (x % 63)
		var d uint8 = 200 >> s8
		var e int32 = -1 << (x % 40)
		var f uint64 = 1<<(x%64) | 1
		fmt.Println(b, c, d, e, f, float64(b))
	}
}
