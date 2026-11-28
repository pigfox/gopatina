// Command closures_math passes math functions as function values (RFC
// 0018): each is the runtime function a call of it uses, which gives Go's
// result bit for bit. Go Tour's function-values.
package main

import (
	"fmt"
	"math"
)

func compute(fn func(float64, float64) float64) float64 {
	return fn(3, 4)
}

func apply(f func(float64) float64, x float64) float64 {
	return f(x)
}

func main() {
	hypot := func(x, y float64) float64 {
		return math.Sqrt(x*x + y*y)
	}
	fmt.Println(hypot(5, 12))
	fmt.Println(compute(hypot))
	fmt.Println(compute(math.Pow))
	fmt.Println(apply(math.Sqrt, 2), apply(math.Floor, -2.5), apply(math.Ceil, 2.25), apply(math.Trunc, -7.75))
	fmt.Println(apply(math.Log, 1), apply(math.Exp, 0), apply(math.Sin, 0), apply(math.Cos, 0))
}
