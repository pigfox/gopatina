// Found by the generated differential tests (seed 1, program 9143, the
// gate on 9dd7ba5, 2026-11-22): with optimization and inlining, gc (Go
// 1.22.0 to 1.27.1, amd64; 1.21.0 is right) prints 0 for lib.F9143(0,
// -2147483648), inlined across packages, while the specification, a hand
// calculation (4294967296 / -2147483647 is -2) and gc with -gcflags=-l or
// -N -l give -2. The same 2^32 carry as gc_inline_carry_1 and _2. GoPatina
// must match the reference build.

package main

import (
	"fmt"

	"example.com/gccarry3/lib"
)

func h9143(a, b int) int {
	return min((-b), (a - a))
}

type N9143 uint16

func (x N9143) Mix(y N9143) N9143 {
	return (((x + 3) | (y >> 7)) % (x | 1))
}

type L9143 []N9143

func (l *L9143) Add(x N9143) { *l = append(*l, x) }

type A9143 = N9143

func p9143() {
	v0 := 0
	v1 := -2147483648
	v2 := 466
	v3 := -284
	s := make([]int, 2)
	var t []int
	str := ""
	m0 := 11
	w0 := make([]int, m0)
	fmt.Println(w0[0], w0[m0-1], len(w0))
	for i1 := 0; i1 < 1; i1++ {
		switch v3 % 3 {
		default:
			v1 -= ((v0 >> 26) / (v1 | 1))
		}
	}
	v0 = h9143((-s[(v1%2+2)%2]), s[(h9143(v0, v1)%2+2)%2])
	sum := 0
	fmt.Println(v0, v1, v2, v3, sum, len(t), len(str))
	n := N9143(v2 * v2)
	m := n.Mix(N9143(v0))
	var l L9143
	pl := &l
	pl.Add(m.Mix(n))
	var a A9143 = m
	m = a ^ n
	fz := float64(v2 - v2)
	fs := []float64{float64(v0) * 1e3, float64(v0) / float64(v2|1) * 1e19, fz / fz, 1 / fz, -1 / fz}
	f32 := float32(fs[1]) * -7.5
	fmt.Println(int(f32), int8(f32), int16(f32), uint16(f32), uint32(f32), uint64(f32), uint(fs[0]), uint32(fs[1]), uint64(fs[4]))
	sa := uint8(v0)
	sc := uint8(v2 & 127)
	sa >>= sc / 3
	lp := lib.P9143{A: v0, B: v1}
	fmt.Println(lp.Sum(), lp.A, lib.F9143(v0, v1), lib.K9143, lib.N9143(v2), []lib.N9143{lib.N9143(v1)})
}

func main() { p9143() }
