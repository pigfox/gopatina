// A program of several packages (RFC 0009): types, methods, constants and
// functions across packages, equal names in different packages, Go's
// package initialization order (here c, z, a, b), a package reached only
// in part, and math/bits intrinsics, in and outside their documented domain.
package main

import (
	"fmt"
	"math/bits"

	"example.com/multi/a"
	"example.com/multi/b"
	"example.com/multi/c"
)

func main() {
	p := a.New(3, 4)
	q := b.Scale(p, a.Max)
	fmt.Println(p, q, a.Sum([]a.Point{p, q}), b.New(), c.New(), b.Kind(2))
	var k b.Kind = b.Big
	fmt.Printf("%v %d %s %d\n", k, k, k, b.Shift(3, a.Max))
	// Outside Go's documented domain (a carry other than 0 or 1): gc on
	// amd64 counts a carry whose low 32 bits are not zero as 1.
	m := uint64(1<<64 - 1)
	for _, cy := range []uint64{0, 1, 2, 1 << 32, 1 << 63} {
		s, co := bits.Add64(m, 1, cy)
		d, bo := bits.Sub64(0, 1, cy)
		fmt.Println(cy, s, co, d, bo)
	}
	h, l := bits.Mul64(m, m)
	n := -1
	fmt.Println(h, l, bits.RotateLeft64(1, n), bits.RotateLeft8(1, 9), bits.Len64(255), bits.LeadingZeros32(0), bits.TrailingZeros(8), bits.OnesCount16(7), bits.ReverseBytes32(0x01020304))
}
