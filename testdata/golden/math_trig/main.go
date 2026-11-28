// Command math_trig computes math.Sin and math.Cos, which Go writes in Go on
// every target (sin.go, and trig_reduce.go from 2**29 on), and which gc
// fuses differently for each: on arm64 the reduction and every polynomial
// step, at GOAMD64=v3 each polynomial's outer add, below v3 nothing. It
// hashes their results' text over edge values and random arguments, so
// GoPatina must give each target's bits.
package main

import (
	"fmt"
	"math"
)

// mix folds f into h (FNV-1a over its text; every NaN is "NaN").
func mix(h uint64, f float64) uint64 {
	s := fmt.Sprint(f)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

func main() {
	edges := []float64{
		0, math.Copysign(0, -1), 1, -1, 0.5, 2, 3, 10, -10, 100,
		math.Pi, math.Pi / 2, math.Pi / 4, -math.Pi / 4, 3 * math.Pi / 4, 2 * math.Pi, 1e-300, 5e-324,
		1 << 29, 1<<29 - 1, 1 << 30, 1e10, -1e10, 1e22, 1e100, 1e300, math.MaxFloat64, -math.MaxFloat64,
		math.Inf(1), math.Inf(-1), math.NaN(), math.Float64frombits(0xFFF8000000000123),
	}
	for _, x := range edges {
		s, c := math.Sin(x), math.Cos(x)
		fmt.Println(x, s, math.Float64bits(s), c, math.Float64bits(c))
	}
	h := uint64(14695981039346656037)
	x := uint64(88172645463325252)
	for i := 0; i < 200000; i++ {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		// Every bit pattern, both signs, every exponent; and arguments
		// within 64 pi of zero, where most programs call them.
		a := math.Float64frombits(x)
		b := float64(int64(x)>>11) / 9007199254740992 * 64 * math.Pi
		h = mix(mix(mix(mix(h, math.Sin(a)), math.Cos(a)), math.Sin(b)), math.Cos(b))
	}
	fmt.Println("hash", h)
}
