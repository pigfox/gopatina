// Command math_exact checks the provided math functions that are exact by
// IEEE 754 bit for bit against Go: each function's result bits over edge
// values, random bit patterns and random values with fractions, hashed.
package main

import (
	"fmt"
	"math"
)

// mix folds the bits of f into the hash h (FNV-1a over the eight bytes).
func mix(h uint64, f float64) uint64 {
	b := math.Float64bits(f)
	for i := 0; i < 8; i++ {
		h ^= b & 0xff
		h *= 1099511628211
		b >>= 8
	}
	return h
}

// flag folds a bool into h.
func flag(h uint64, b bool) uint64 {
	if b {
		return mix(h, 1)
	}
	return mix(h, 0)
}

// hashes are the running hashes, one per function.
type hashes struct {
	sqrt, abs, floor, ceil, trunc, copysign, signbit, isnan, isinf, bits uint64
}

func (h *hashes) add(x, y float64) {
	h.sqrt = mix(h.sqrt, math.Sqrt(x))
	h.abs = mix(h.abs, math.Abs(x))
	h.floor = mix(h.floor, math.Floor(x))
	h.ceil = mix(h.ceil, math.Ceil(x))
	h.trunc = mix(h.trunc, math.Trunc(x))
	h.copysign = mix(h.copysign, math.Copysign(x, y))
	h.signbit = flag(h.signbit, math.Signbit(x))
	h.isnan = flag(h.isnan, math.IsNaN(x))
	h.isinf = flag(flag(flag(h.isinf, math.IsInf(x, -1)), math.IsInf(x, 0)), math.IsInf(x, 1))
	h.bits = mix(h.bits, math.Float64frombits(math.Float64bits(x)^0x8000000000000001))
}

func (h *hashes) print(what string) {
	fmt.Println(what, "sqrt", h.sqrt, "abs", h.abs, "floor", h.floor, "ceil", h.ceil, "trunc", h.trunc)
	fmt.Println(what, "copysign", h.copysign, "signbit", h.signbit, "isnan", h.isnan, "isinf", h.isinf, "bits", h.bits)
}

func newHashes() hashes {
	const basis = 14695981039346656037
	return hashes{basis, basis, basis, basis, basis, basis, basis, basis, basis, basis}
}

func main() {
	edges := []float64{
		0, math.Copysign(0, -1), 1, -1, 0.5, -0.5, 1.5, -1.5, 2.5, -2.5,
		0.49999999999999994, -0.49999999999999994, 4503599627370495.5, -4503599627370495.5,
		4503599627370496, -4503599627370496, 9007199254740993, 1e300, -1e300,
		5e-324, -5e-324, 2.2250738585072014e-308, math.MaxFloat64, -math.MaxFloat64,
		math.Inf(1), math.Inf(-1), math.NaN(),
		math.Float64frombits(0x7FF8000000000000), math.Float64frombits(0xFFF8000000000456),
		math.Float64frombits(0x7FF0000000000123), math.Float64frombits(0xFFF0000000000001),
		math.Pi, -math.E, math.Sqrt2,
	}
	h := newHashes()
	for i, x := range edges {
		h.add(x, edges[(i+7)%len(edges)])
		fmt.Println("edge", i, math.Float64bits(math.Sqrt(x)), math.Float64bits(math.Floor(x)),
			math.Float64bits(math.Ceil(x)), math.Float64bits(math.Trunc(x)), math.Float64bits(math.Abs(x)))
	}
	h.print("edges")
	fmt.Println("inf", math.Float64bits(math.Inf(1)), math.Float64bits(math.Inf(0)), math.Float64bits(math.Inf(-3)),
		"nan", math.Float64bits(math.NaN()))
	for sign := -2; sign <= 2; sign++ {
		fmt.Println("sign", sign, math.Float64bits(math.Inf(sign)),
			math.IsInf(math.Inf(1), sign), math.IsInf(math.Inf(-1), sign), math.IsInf(math.MaxFloat64, sign))
	}

	// Random bit patterns: every exponent, NaNs of every payload.
	x := uint64(88172645463325252)
	h = newHashes()
	for i := 0; i < 100000; i++ {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		y := x*2862933555777941757 + 3037000493
		h.add(math.Float64frombits(x), math.Float64frombits(y))
	}
	h.print("bits")

	// Values with fractions, where floor, ceil and trunc round.
	h = newHashes()
	for i := 0; i < 100000; i++ {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		f := float64(int64(x)>>11) / float64(uint64(1)<<(x%60))
		h.add(f, -f)
	}
	h.print("fractions")
}
