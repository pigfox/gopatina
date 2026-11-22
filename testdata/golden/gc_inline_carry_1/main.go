// Found by the generated differential tests (seed 1, program 169026): with
// optimization and inlining, gc (Go 1.25.0 to 1.27.1, amd64) prints 4489 for
// v2, while the specification, a hand calculation and gc with -gcflags=-l or
// -N -l give -287762804343. GoPatina must match the reference build.
package main

import "fmt"

func h169026(a, b int) int {
	return ((a + b) + b)
}
func p169026() {
	v0 := 4294967297
	v1 := 213
	v2 := 280
	v3 := -2147483648
	s := make([]int, 2)
	var t []int
	str := ""
	if !(len(t) != (-v0)) {
		for i0 := 0; i0 < 3; i0++ {
			v1 &= ((-v2) % (h169026(v0, v1) | 1))
		}
	}
	s[((v0/(v0|1))%2+2)%2] = ((-v2) + v1)
	v2 = min((v0 ^ min(v0, v1)), min(max(v3, v3), (v3^v1)))
	if (v0+v0) > v3 || (-v1) <= len(t) {
		str += fmt.Sprintf("%5d,", (v0 & v2))
		s[((v1>>47)%2+2)%2] = s[(max(v1, v2)%2+2)%2]
	}
	v2 = (h169026(s[(v0%2+2)%2], (-v2)) * s[(v3%2+2)%2])
	m1 := 28
	w1 := make([]int, m1)
	fmt.Println(w1[0], w1[m1-1], len(w1))
	sum := 0
	fmt.Println(v0, v1, v2, v3, sum, len(t), len(str))
}

func main() { p169026() }
