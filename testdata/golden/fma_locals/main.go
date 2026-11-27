// Command fma_locals keeps a product in a local and adds to it later: gc
// carries the product's SSA value across statements and fuses the add on
// arm64 and on amd64 from GOAMD64=v3, float32 too, while the local itself
// keeps the rounded product. A factor changed in between does not change
// the fused result: gc's value is the factor's at the multiply.
package main

import (
	"fmt"
	"math"
)

// mix folds f into h (FNV-1a over its text, the shortest digits that
// read back as f: every value but NaN keeps its bits, -0 is "-0", and
// every NaN is "NaN", whose payload the nan_payloads golden checks).
func mix(h uint64, f float64) uint64 {
	s := fmt.Sprint(f)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

//go:noinline
func narrow(fs []float64) {
	f32 := float32(fs[1]) * -7.5
	fs[1] = 99 // after the multiply: the fused add has the old factor
	fmt.Println(f32, f32*f32, f32/3, -f32, f32+0.1, float64(f32), fs[1])
}

//go:noinline
func twice(a, b, c, d float64) (float64, float64, float64) {
	p := a * b
	return p, p + c, d - p
}

func main() {
	narrow([]float64{0, -0.0156946679, 2})
	narrow([]float64{0, 95.09, 2})
	p, q, r := twice(0.1, 0.2, 0.3, 0.7)
	fmt.Println(p, q, r)

	h, h32 := uint64(14695981039346656037), uint64(14695981039346656037)
	x := uint64(88172645463325252)
	for i := 0; i < 50000; i++ {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		a := float64(int64(x)>>11) / 9007199254740992
		b := float64(int64(x*2862933555777941757)>>11) / 4503599627370496
		if i%997 == 0 {
			a = math.Float64frombits(x | 0x7ff0000000000001) // a NaN, signaling or quiet
		}
		p := a * b
		c := -float64(a * b)
		h = mix(h, p+c)
		h = mix(h, c-p)
		q := float32(a) * float32(b)
		h32 = mix(h32, float64(q+float32(c)))
	}
	fmt.Println(h, h32)
}
