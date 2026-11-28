// Command closures_param passes functions to parameters of function type
// (RFC 0018, step 2): a function of the program, a closure, or a function
// literal written in the call. Each parameter is a generic impl Fn in
// Rust, or impl FnMut where a closure passed to it changes what it uses.
package main

import "fmt"

func apply(f func(int) int, x int) int {
	return f(x)
}

func twice(f func(int) int, x int) int {
	return f(f(x))
}

func count(n int, keep func(int) bool) int {
	c := 0
	for i := 0; i < n; i++ {
		if keep(i) {
			c++
		}
	}
	return c
}

func describe(x float64, name func(float64) string) {
	fmt.Println(x, "is", name(x))
}

func each(n int, visit func(int)) {
	for i := 1; i <= n; i++ {
		visit(i)
	}
}

func square(x int) int { return x * x }

func even(i int) bool { return i%2 == 0 }

type scale struct {
	factor int
}

func (s scale) map3(f func(int) int) int {
	return f(s.factor) + f(2*s.factor) + f(3*s.factor)
}

func sizeName(x float64) string {
	if x < 10 {
		return "small"
	}
	return "big"
}

func main() {
	fmt.Println(apply(square, 7), twice(square, 3))
	double := func(x int) int { return 2 * x }
	fmt.Println(apply(double, 21), twice(double, 5))

	total := 0
	add := func(x int) int {
		total += x
		return total
	}
	each(4, func(i int) { fmt.Println("visit", i) })
	r := apply(add, 10)
	r = apply(add, 5)
	fmt.Println("running", r, total)

	limit := 3
	small := count(10, func(i int) bool { return i < limit })
	fmt.Println(count(10, even), small)

	describe(2.5, sizeName)
	describe(40, sizeName)
	fmt.Println(scale{4}.map3(square), scale{1}.map3(double))

	sum := 0
	each(5, func(i int) { sum += i })
	fmt.Println("sum", sum)
	fmt.Println(offset(3))
}

func offset(base int) int {
	return apply(func(x int) int { return x + base }, 100)
}
