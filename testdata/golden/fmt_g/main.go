// Command fmt_g prints floats with %g, which for a float is Go's %v: the
// shortest digits that read back as the number, with an exponent below
// 1e-4 and from 1e+06 up, a width and the - flag.
package main

import (
	"fmt"
	"math"
)

func main() {
	xs := []float64{
		0, math.Copysign(0, -1), 1, -2.5, 0.1, 1.0 / 3, 2.0 / 3, 100000, 999999, 1e6, 123456789,
		1e20, 1e21, 0.0001, 0.00001, 1.5e-300, 5e-324, math.MaxFloat64,
		math.Inf(1), math.Inf(-1), math.NaN(), math.Sqrt(2), math.Pi,
	}
	for _, x := range xs {
		fmt.Printf("[%g] [%12g] [%-12g] [%v]\n", x, x, x, x)
	}
	var f float32 = 16777216.5
	var g float32 = 0.1
	fmt.Printf("%g %g %10g|\n", f, g, -g)
	s := fmt.Sprintf("%g / %g = %g", 7.0, 2.0, 7.0/2)
	fmt.Println(s, len(s))
	err := fmt.Errorf("limit %g exceeded", 1e9)
	fmt.Println(err)
	fmt.Printf("Now you have %g problems.\n", math.Sqrt(7))
}
