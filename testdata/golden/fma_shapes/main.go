// Command fma_shapes computes the float multiply-adds gc fuses into one
// rounding on arm64 and on amd64 from GOAMD64=v3 (FMADD, FMSUB, FNMSUB,
// VFMADD231SD), and the shapes it leaves unfused, and hashes each
// result's bits over random bit patterns (NaN payloads and infinities
// included) and over values that cancel, where one rounding and two
// differ. GoPatina must fuse exactly where gc does for the target.
package main

import (
	"fmt"
	"math"
)

//go:noinline
func addMul(a, b, c float64) float64 { return a*b + c }

//go:noinline
func mulAddLeft(a, b, c float64) float64 { return c + a*b }

//go:noinline
func twoMuls(a, b, c, d float64) float64 { return a*b + c*d }

//go:noinline
func subMul(a, b, c float64) float64 { return c - a*b }

//go:noinline
func mulSub(a, b, c float64) float64 { return a*b - c }

//go:noinline
func parens(a, b, c float64) float64 { return (a * b) + (c) }

//go:noinline
func half(a, c float64) float64 { return a*0.5 + c }

//go:noinline
func three(a, c float64) float64 { return a*3 + c }

//go:noinline
func double(a, c float64) float64 { return a*2 + c } // gc's a+a: not fused

//go:noinline
func rounded(a, b, c float64) float64 { return float64(a*b) + c } // the conversion rounds

//go:noinline
func phi(a, b, c, d float64) float64 {
	t := a * b
	if c > 0 {
		t = c
	}
	return t + d // t is a phi of a*b and c: not fused
}

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

type hashes struct {
	add, left, two, sub, msub, par, half, three, double, rounded, phi uint64
}

func (h *hashes) add4(a, b, c, d float64) {
	h.add = mix(h.add, addMul(a, b, c))
	h.left = mix(h.left, mulAddLeft(a, b, c))
	h.two = mix(h.two, twoMuls(a, b, c, d))
	h.sub = mix(h.sub, subMul(a, b, c))
	h.msub = mix(h.msub, mulSub(a, b, c))
	h.par = mix(h.par, parens(a, b, c))
	h.half = mix(h.half, half(a, c))
	h.three = mix(h.three, three(a, c))
	h.double = mix(h.double, double(a, c))
	h.rounded = mix(h.rounded, rounded(a, b, c))
	h.phi = mix(h.phi, phi(a, b, c, d))
}

func (h *hashes) print(what string) {
	fmt.Println(what, "add", h.add, "left", h.left, "two", h.two, "sub", h.sub, "msub", h.msub, "par", h.par)
	fmt.Println(what, "half", h.half, "three", h.three, "double", h.double, "rounded", h.rounded, "phi", h.phi)
}

func newHashes() hashes {
	const basis = 14695981039346656037
	return hashes{basis, basis, basis, basis, basis, basis, basis, basis, basis, basis, basis}
}

func main() {
	x := uint64(88172645463325252)
	next := uint64(0)
	h := newHashes()
	for i := 0; i < 50000; i++ {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		a := math.Float64frombits(x)
		next = x*2862933555777941757 + 3037000493
		b := math.Float64frombits(next)
		next = next*2862933555777941757 + 3037000493
		c := math.Float64frombits(next)
		next = next*2862933555777941757 + 3037000493
		h.add4(a, b, c, math.Float64frombits(next))
	}
	h.print("bits")
	h = newHashes()
	for i := 0; i < 50000; i++ {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		a := float64(int64(x)>>11) / 9007199254740992
		b := float64(int64(x*2862933555777941757)>>11) / 4503599627370496
		c := -float64(a * b) // cancels a*b but for its rounding
		d := float64(int64(x^0x5555555555555555)>>11) / 1e15
		h.add4(a, b, c, d)
	}
	h.print("cancel")
	fmt.Println(addMul(0.1, 0.2, -0.02), subMul(0.1, 0.2, 0.02), mulSub(0.1, 0.2, 0.02))
	fmt.Println("-0 and 0 hash apart:", mix(1, math.Copysign(0, -1)) != mix(1, 0))
}
