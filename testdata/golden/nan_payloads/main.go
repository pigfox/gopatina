// Command nan_payloads prints the bits of NaN results, which gc computes
// on the hardware (it never folds an operation whose result is NaN) and
// rustc may fold to its own NaN: one NaN operand, signaling or quiet,
// against a constant; two NaNs into - and /, whose operands gc never
// swaps; an invalid operation, which gives the target's default NaN;
// conversions between float64 and float32, and Sqrt. Values that are
// not NaN show their bits are those of the plain operation. The same
// from values the compiler can see (constants) and from values it
// cannot (an argument), on every target.
package main

import (
	"fmt"
	"math"
)

func bits(what string, f float64) { fmt.Printf("%s %x\n", what, math.Float64bits(f)) }

//go:noinline
func ops(q, s, n, x float64) {
	bits("q+1", q+1)
	bits("1+s", 1+s)
	bits("s-1", s-1)
	bits("q*2", q*2)
	bits("s/3", s/3)
	bits("q-s", q-s)
	bits("s-q", s-q)
	bits("n/q", n/q)
	bits("q/s", q/s)
	bits("x-x", x-x)
	bits("0*x", 0*x)
	bits("x/x", x/x)
	bits("32 q", float64(float32(q)))
	bits("32 s", float64(float32(s)))
	bits("32 n", float64(float32(n)))
	bits("sqrt q", math.Sqrt(q))
	bits("sqrt s", math.Sqrt(s))
	bits("sqrt -1", math.Sqrt(n-1-n))
	bits("abs s", math.Abs(s))
	bits("-q", -q)
	// Not NaN: the plain IEEE results.
	bits("0.1+0.2", n*0+0.1+0.2)
	bits("1/3", (n-n+1)/3)
	bits("32 1/3", float64(float32((n-n+1)/3)))
	bits("sqrt 2", math.Sqrt(n-n+2))
}

func main() {
	q := math.Float64frombits(0x7FF8000000000AAA)
	s := math.Float64frombits(0xFFF0000000000BBB)
	n := math.Float64frombits(0x7FF800AAA0000001)
	ops(q, s, n, math.Inf(1))
	fmt.Println("from an argument, with n a number:")
	ops(q, s, 5, math.Inf(-1))
	fmt.Println("from constants the compiler sees:")
	bits("c inf-inf", math.Inf(1)-math.Inf(1))
	bits("c 0*inf", 0*math.Inf(1))
	bits("c s*2", math.Float64frombits(0x7FF0000000000CCC)*2)
	bits("c 32", float64(float32(math.Float64frombits(0x7FF800AAA0000000))))
	bits("c sqrt -2", math.Sqrt(-2))
	bits("c 0.1*3", 0.1*3)
}
