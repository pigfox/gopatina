// Command math_exp computes math.Exp, which Go writes in assembly for each
// target (exp_amd64.s, fused when the processor has FMA; exp_arm64.s), and
// hashes its results' text over edge values (overflow, underflow and the
// subnormal results between, the 2^-28 cut, NaN and the infinities) and
// over random arguments, so GoPatina must give the same bits on each.
package main

import (
	"fmt"
	"math"
)

// mix folds f into h (FNV-1a over its text, the shortest digits that read
// back as f; every NaN is "NaN").
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
		0, math.Copysign(0, -1), 1, -1, 0.5, 2, 10, -10, 100, -100,
		709.782712893384, 709.7827128933841, 709.78271289338397, 710, -708.39641853226, -708.4, -720,
		-745.1332191019411, -745.1332191019412, -746, -1e300, 1e300,
		3.7252902984e-9, 3.7252902985e-9, -3.7252902984e-9, 1e-20, -1e-20, 5e-324,
		math.Inf(1), math.Inf(-1), math.NaN(), math.Ln2, -math.Ln2, 0.34657359027997264,
	}
	for _, x := range edges {
		fmt.Println(x, math.Exp(x))
	}
	h := uint64(14695981039346656037)
	x := uint64(88172645463325252)
	for i := 0; i < 200000; i++ {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		// Arguments over (-760, 760), and near 0.
		a := float64(int64(x)>>11) / 9007199254740992 * 760
		b := a / float64(uint64(1)<<(x%40))
		h = mix(mix(h, math.Exp(a)), math.Exp(b))
	}
	fmt.Println("hash", h)
}
