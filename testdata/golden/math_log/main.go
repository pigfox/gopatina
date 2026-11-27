// Command math_log computes math.Log, which Go writes in assembly on amd64
// (log_amd64.s, whose Frexp leaves a subnormal argument as it is and whose
// reduction takes f1 <= Sqrt2/2) and in Go elsewhere (log.go, which gc
// fuses ten multiply-adds of on arm64), and hashes its results' text over
// edge values and random arguments, so GoPatina must give each target's
// bits.
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
		1, 2, 0.5, math.E, 10, 0.1, 1e-300, 1e300, math.MaxFloat64, math.SmallestNonzeroFloat64,
		2.2250738585072014e-308, 1e-310, 3e-320, 0.7071067811865476, 0.7071067811865475, 1.4142135623730951,
		0.7071067811865476 * 1024, math.Copysign(0, -1), 0, -1, -1e-300,
		math.Inf(1), math.Inf(-1), math.NaN(), math.Float64frombits(0xFFF8000000000123),
		1 + 1e-15, 1 - 1e-15, 1e-5,
	}
	for _, x := range edges {
		fmt.Println(x, math.Log(x), math.Float64bits(math.Log(x)))
	}
	h := uint64(14695981039346656037)
	x := uint64(88172645463325252)
	for i := 0; i < 200000; i++ {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		// Positive bit patterns: every exponent, subnormals included; and
		// arguments near 1.
		a := math.Float64frombits(x >> 1)
		b := 1 + float64(int64(x)>>11)/9007199254740992/float64(uint64(1)<<(x%50))
		h = mix(mix(h, math.Log(a)), math.Log(b))
	}
	fmt.Println("hash", h)
}
