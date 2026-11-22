# Draft upstream issue: cmd/compile miscompiles int arithmetic after inlining (amd64)

**Status: draft, not filed.** Filing it is outward-facing and waits for the
project lead's decision (relayed by the coordinator, 2026-11-22). Nothing
here has been sent anywhere.

The text below is what would be filed at https://github.com/golang/go/issues,
following Go's issue template.

---

### Title

cmd/compile: wrong int result with inlining on amd64 (missing 2^32 carry)

### Go version

go version go1.27.1 linux/amd64

### Does this issue reproduce with the latest release?

Yes. Program 1 reproduces with Go 1.22.0, 1.23.0, 1.24.0, 1.25.0, 1.25.5,
1.25.10, 1.26.0, 1.27.0 and 1.27.1, and not with 1.21.0. Program 2
reproduces with 1.26.0, 1.27.0 and 1.27.1, and not with 1.25.10 or earlier.

### What operating system and processor architecture are you using?

linux/amd64. Other architectures not tested.

### What did you do?

Two programs found by differential testing against another compiler, each
reduced automatically (statements deleted while the optimized and the
unoptimized gc builds still disagreed). Further hand reduction made both
disagreements disappear, so they are given at this size.

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

Run each with `go run .` and with `go run -gcflags=all=-l .`.

### What did you see happen?

With the default build, program 1 prints `4294967297 213 4489 -2147483648 0 0 11`
as its last line, and program 2 prints `1171 787 412614656 0 4294967297 0 0 0`.

With `-gcflags=all=-l` (or `-gcflags=all=-N -l`), program 1 prints
`4294967297 213 -287762804343 -2147483648 0 0 11`, and program 2 prints
`1171 787 412614656 4294967296 4294967297 0 0 0`.

### What did you expect to see?

The second pair, which is what the specification gives:

- Program 1: the last assignment is `v2 = h169026(s[1], -v2) * s[0]`, with
  `v2 == -2147483648` and `s == [-67, -67]` at that point. So
  `h(-67, 2147483648) == -67 + 2*2147483648 == 4294967229`, and
  `4294967229 * -67 == -287762804343`.
- Program 2: `v3 &= h174986(v2/(v4|1), -v3)` with `v3 == -2147483648`,
  `v2 == 412614656`, `v4 == 4294967297`: `v2/(v4|1) == 0`,
  `h(0, 2147483648) == 4294967296`, and `-2147483648 & 4294967296 == 4294967296`.

In both, the inlined function is `h(a, b) = (a+b)+b` or `(b+a)+b` called with
`b = -v` where `v == -2147483648`, and the wrong result is what you get if
`2*b` lost its carry out of bit 31 (`2*2147483648 == 2^32`). Without
inlining the result is right.
