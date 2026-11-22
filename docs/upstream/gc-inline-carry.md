# Draft upstream issue: cmd/compile drops 2^32 when folding two SUBQconst [-1<<31] (amd64)

**Status: filed** on the project lead's word, 2026-11-22, as
[golang/go#83292](https://github.com/golang/go/issues/83292). Replies are
watched by the coordinator and relayed.

The text below is what was filed, following Go's issue template.

---

### Title

cmd/compile: amd64 rule (SUBQconst (SUBQconst x [d]) [c]) overflows for c == -1<<31, dropping 2^32

### Go version

go version go1.27.1 linux/amd64

### Does this issue reproduce with the latest release?

Yes. Program 4, the smallest, reproduces with every release tested: Go
1.21.0, 1.22.0, 1.23.0, 1.24.0, 1.25.0, 1.26.0 and 1.27.1. Program 1
reproduces with Go 1.22.0, 1.23.0, 1.24.0, 1.25.0, 1.25.5, 1.25.10, 1.26.0,
1.27.0 and 1.27.1, and not with 1.21.0. Program 2 reproduces with 1.26.0,
1.27.0 and 1.27.1, and not with 1.25.10 or earlier. The rule is the same in
all of them; the releases differ in which programs reach it.

### What operating system and processor architecture are you using?

linux/amd64. Other architectures not tested.

### The cause

`src/cmd/compile/internal/ssa/_gen/AMD64.rules` has

```
(SUBQconst (SUBQconst x [d]) [c]) && is32Bit(int64(-c)-int64(d)) => (ADDQconst [-c-d] x)
```

`c` and `d` are `int32`, and `-c` is computed in `int32` before the
conversion. For `c == d == -1<<31` (each `SUBQconst` adds `2^31`), `-c`
overflows to `-1<<31`, the condition checks `is32Bit(0)`, and the rule
rewrites `x + 2^31 + 2^31` to `ADDQconst [0] x`: the sum loses `2^32`. The
correct sum, `2^32`, does not fit a 32-bit immediate, so the rule must not
fire. The neighbouring rule `(SUBQconst [c] x) && c != -(1<<31)` already
guards this value. A fix is to compute the condition in 64 bits:

```
(SUBQconst (SUBQconst x [d]) [c]) && is32Bit(-int64(c)-int64(d)) => (ADDQconst [-c-d] x)
```

(When the condition holds, the 32-bit `-c-d` wraps to the same value.)
With only the condition changed in `rewriteAMD64.go` (to
`is32Bit(-int64(c) - int64(d))`) and `cmd/compile` rebuilt, Go 1.27.1 prints
the right result for all four programs below.

### What did you do?

Four programs found by differential testing against another compiler. The
first three were reduced automatically (statements deleted while the
optimized and the unoptimized gc builds still disagreed); program 4 was
then reduced by hand, after the cause was found.

Program 4 (`go run .` prints `2147483720 72`; the specification, and
`go run -gcflags=all=-N -l .`, give `2147483720 4294967368`). The loop only
keeps the compiler from knowing `x` until late in compilation:

```go
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
```

`go build -gcflags=-S` shows the first addition as `SUBQ $-2147483648, CX`
and then stores the original `C` back: the second addition was folded
into the first as `+0`.

Program 1:

```go
package main

import "fmt"

func h169026(a, b int) int {
	return ((a + b) + b)
}
func p169026() {
	v0 := 4294967297
	v1 := 213
	v2 := 280
	v3 := -2147483648
	s := make([]int, 2)
	var t []int
	str := ""
	if !(len(t) != (-v0)) {
		for i0 := 0; i0 < 3; i0++ {
			v1 &= ((-v2) % (h169026(v0, v1) | 1))
		}
	}
	s[((v0/(v0|1))%2+2)%2] = ((-v2) + v1)
	v2 = min((v0 ^ min(v0, v1)), min(max(v3, v3), (v3^v1)))
	if (v0+v0) > v3 || (-v1) <= len(t) {
		str += fmt.Sprintf("%5d,", (v0 & v2))
		s[((v1>>47)%2+2)%2] = s[(max(v1, v2)%2+2)%2]
	}
	v2 = (h169026(s[(v0%2+2)%2], (-v2)) * s[(v3%2+2)%2])
	m1 := 28
	w1 := make([]int, m1)
	fmt.Println(w1[0], w1[m1-1], len(w1))
	sum := 0
	fmt.Println(v0, v1, v2, v3, sum, len(t), len(str))
}

func main() { p169026() }
```

Program 2:

```go
package main

import "fmt"

func h174986(a, b int) int {
	return ((b + a) + b)
}
func p174986() {
	v0 := -403
	v1 := 787
	v2 := 64
	v3 := -2147483648
	v4 := 4294967297
	s := make([]int, 8)
	var t []int
	str := ""
	v0 = h174986(v0, v1)
	switch v0 % 3 {
	case 0:
		switch v2 % 3 {
		case 0:
			v0 = min(((v4 << 5) ^ (v1 % (v4 | 1))), (len(t) / ((-v2) | 1)))
		}
	case 1, -1:
		v2 = (h174986(max(v1, v1), s[(v1%8+8)%8]) << 19)
	}
	switch v0 % 3 {
	case 1, -1:
	default:
		if (v2 + v2) > (v4 >> 15) {
			v3 = s[(min((-v2), h174986(v0, v0))%8+8)%8]
		}
	}
	v3 &= h174986((v2 / (v4 | 1)), (-v3))
	m1 := 20
	w1 := make([]int, m1)
	fmt.Println(w1[0], w1[m1-1], len(w1))
	sum := 0
	fmt.Println(v0, v1, v2, v3, v4, sum, len(t), len(str))
}

func main() { p174986() }
```

Program 3, of two packages (module `example.com/gccarry3`), where the
function is inlined across packages (found 2026-11-22):

```go
// main.go
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
```

```go
// lib/lib.go
package lib

import "fmt"

type P9143 struct{ A, B int }

const K9143 = 866

func F9143(x, y int) int { return ((((x + x) & 22) - y) - y) / (y | 1) }

type N9143 int32

func (p P9143) Sum() int { return p.A + p.B*K9143 }

func (p *P9143) Grow(x int) {
	p.A ^= x
	p.B += x
}

func (n N9143) String() string { return fmt.Sprintf("n%d", int(n)) }
```

Run each with `go run .` and with `go run -gcflags=all=-l .`.

### What did you see happen?

With the default build, program 1 prints `4294967297 213 4489 -2147483648 0 0 11`
as its last line, program 2 prints `1171 787 412614656 0 4294967297 0 0 0`,
and program 3 prints `-1859720839168 0 0 866 n466 [n-2147483648]`.

With `-gcflags=all=-l` (or `-gcflags=all=-N -l`), program 1 prints
`4294967297 213 -287762804343 -2147483648 0 0 11`, and program 2 prints
`1171 787 412614656 4294967296 4294967297 0 0 0`, and program 3 prints
`-1859720839168 0 -2 866 n466 [n-2147483648]`.

### What did you expect to see?

The second pair, which is what the specification gives:

- Program 1: the last assignment is `v2 = h169026(s[1], -v2) * s[0]`, with
  `v2 == -2147483648` and `s == [-67, -67]` at that point. So
  `h(-67, 2147483648) == -67 + 2*2147483648 == 4294967229`, and
  `4294967229 * -67 == -287762804343`.
- Program 2: `v3 &= h174986(v2/(v4|1), -v3)` with `v3 == -2147483648`,
  `v2 == 412614656`, `v4 == 4294967297`: `v2/(v4|1) == 0`,
  `h(0, 2147483648) == 4294967296`, and `-2147483648 & 4294967296 == 4294967296`.

- Program 3: `lib.F9143(0, -2147483648)` with
  `F(x, y) = (((x+x)&22) - y - y) / (y|1)`: `4294967296 / -2147483647 == -2`.
  Program 3 reproduces with Go 1.22.0 to 1.27.1, as program 1 does.

In all three, the inlined function is `h(a, b) = (a+b)+b` or `(b+a)+b` called with
`b = -v` where `v == -2147483648`, and the wrong result is what you get if
`2*b` lost its carry out of bit 31 (`2*2147483648 == 2^32`). Without
inlining the result is right, because only inlining lets the compiler see
the constant `2^31` twice; program 4 shows that no function call is
needed, and the rule above is where the carry is lost.
