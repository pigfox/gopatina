// Found by the generated differential tests (seed 1, program 26733, the
// gate on e63429a, 2026-11-22), reduced by hand: with optimization, gc (Go
// 1.21.0 to 1.27.1, amd64) prints 2147483720 72, while the specification
// and gc with -gcflags=all=-N -l give 2147483720 4294967368. gc's amd64
// rule (SUBQconst (SUBQconst x [d]) [c]) negates c in 32 bits, so it folds
// C + 2^31 + 2^31 into C + 0. The same rule is behind gc_inline_carry_1 to
// _3 (docs/upstream/gc-inline-carry.md). GoPatina must match the reference
// build.
package main

import "fmt"

var C = 72

func main() {
	x := -2147483648
	k := 478
	y := -86
	var t []int
	for i := 0; i < 16; i++ {
		switch k % 3 {
		case 0:
			t = append(t, len(t))
		case 1, -1:
			y |= len(t)
		}
	}
	if y < x {
		x = y
	}
	x = -x // 2147483648
	C += x
	a := C
	C += x
	fmt.Println(a, C)
}
