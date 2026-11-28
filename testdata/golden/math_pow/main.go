// Command math_pow computes math.Pow, which Go writes in Go on every target
// (pow.go: Exp and Log for the fraction of y, then squarings for its
// integer part), with each target's Exp and Log. gc fuses one multiply-add
// of it on arm64 and from GOAMD64=v3, a square then its doubling, which
// rounds to the same value. It hashes the results' text over edge values
// and random arguments, so GoPatina must give each target's bits.
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

// r is the xorshift state next advances.
var r = uint64(88172645463325252)

func next() uint64 {
	r ^= r << 13
	r ^= r >> 7
	r ^= r << 17
	return r
}

func main() {
	xs := []float64{
		0, math.Copysign(0, -1), 1, -1, 2, -2, 0.5, -0.5, 3, 10, 0.1, 1e-300, 5e-324, 1e300,
		math.MaxFloat64, math.Inf(1), math.Inf(-1), math.NaN(), 0.7071067811865476, 1.0000001,
	}
	ys := []float64{
		0, math.Copysign(0, -1), 1, -1, 2, -2, 3, -3, 0.5, -0.5, 1.5, -1.5, 0.25, 10, 53, 1e10,
		1 << 63, -(1 << 63), 1 << 53, 1<<53 + 2, math.Inf(1), math.Inf(-1), math.NaN(), 1e-10,
	}
	for _, x := range xs {
		for _, y := range ys {
			p := math.Pow(x, y)
			fmt.Println(x, y, p, math.Float64bits(p))
		}
	}
	h := uint64(14695981039346656037)
	for i := 0; i < 100000; i++ {
		// Every bit pattern for x with integer and small y; x near 1 with
		// large y; and x and y over a wide range of magnitudes.
		a := math.Float64frombits(next())
		n := float64(int64(next()%2001) - 1000)
		b := 1 + float64(int64(next())>>11)/9007199254740992/float64(uint64(1)<<(next()%40))
		c := float64(int64(next())>>11) / 9007199254740992 * 1e6
		d := math.Float64frombits(next() >> 2)
		e := float64(int64(next())>>11) / 9007199254740992 * 40
		h = mix(mix(mix(mix(h, math.Pow(a, n)), math.Pow(b, c)), math.Pow(d, e)), math.Pow(-d, n))
	}
	fmt.Println("hash", h)
}
