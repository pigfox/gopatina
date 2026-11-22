// Found by the generated differential tests (seed 1, program 174986): with
// optimization and inlining, gc (Go 1.26.0 to 1.27.1, amd64; 1.25.10 is
// right) prints 0 for v3, while the specification, a hand calculation and gc with -gcflags=-l or
// -N -l give 4294967296. GoPatina must match the reference build.
package main

import "fmt"

func h174986(a, b int) int {
	return ((b + a) + b)
}
func p174986() {
	v0 := -403
	v1 := 787
	v2 := 64
	v3 := -2147483648
	v4 := 4294967297
	s := make([]int, 8)
	var t []int
	str := ""
	v0 = h174986(v0, v1)
	switch v0 % 3 {
	case 0:
		switch v2 % 3 {
		case 0:
			v0 = min(((v4 << 5) ^ (v1 % (v4 | 1))), (len(t) / ((-v2) | 1)))
		}
	case 1, -1:
		v2 = (h174986(max(v1, v1), s[(v1%8+8)%8]) << 19)
	}
	switch v0 % 3 {
	case 1, -1:
	default:
		if (v2 + v2) > (v4 >> 15) {
			v3 = s[(min((-v2), h174986(v0, v0))%8+8)%8]
		}
	}
	v3 &= h174986((v2 / (v4 | 1)), (-v3))
	m1 := 20
	w1 := make([]int, m1)
	fmt.Println(w1[0], w1[m1-1], len(w1))
	sum := 0
	fmt.Println(v0, v1, v2, v3, v4, sum, len(t), len(str))
}

func main() { p174986() }
