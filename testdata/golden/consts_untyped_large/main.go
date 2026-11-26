// Untyped constants beyond their default type (found by gopatina-parity in
// the Go tour's numeric-constants): Go gives each use the type it is used
// as, and folds it, so no Rust constant of the default type is written.
package main

import "fmt"

const (
	Big    = 1 << 100    // beyond int
	Small  = Big >> 99   // 2: fits
	Wide   = 'a' + 1<<40 // an untyped rune beyond int32
	Huge   = 1e400       // beyond float64
	Letter = 'z'         // fits
)

func needInt(x int) int { return x*10 + 1 }

func needFloat(x float64) float64 { return x * 0.1 }

func main() {
	fmt.Println(needInt(Small), needFloat(Small), needFloat(Big))
	fmt.Println(Wide>>40, Wide-1<<40 == 'a', Letter)
	fmt.Println(Huge/1e300, Huge/1e399 > 9)
}
