// Command closures_local uses closures called where they are made (RFC
// 0018, step 1): each borrows the numbers and bools it uses from its
// enclosing function, and changes them in place when it assigns them.
package main

import "fmt"

// runs counts the calls of counter; a closure does not use it (RFC 0018,
// step 1 keeps package-level variables out of closures).
var runs int

func counter() int {
	runs++
	count := 0
	step := 3
	add := func(n int) {
		n *= step
		count += n
	}
	for i := 1; i <= 4; i++ {
		add(i)
	}
	add(10)
	return count // after the last call: 3*(1+2+3+4) + 30
}

func classify(limit float64) {
	label := func(x float64) string {
		switch {
		case x < 0:
			return "negative"
		case x < limit:
			return "small"
		}
		return "large"
	}
	for _, x := range []float64{-1.5, 0, 2.25, 9, 100} {
		fmt.Println(x, label(x))
	}
}

func flags() {
	seen := false
	hits := 0
	mark := func(b bool) bool {
		if b {
			hits++
			seen = true
		}
		return seen
	}
	fmt.Println(mark(false), mark(true), mark(false))
	fmt.Println(hits, seen)
}

func perIteration() {
	total := 0
	for i := 0; i < 3; i++ {
		square := func() int { return i * i }
		total += square()
	}
	fmt.Println("squares", total)
}

func average(a, b, c float64) float64 {
	mean := func(x, y, z float64) float64 {
		return (x + y + z) / 3
	}
	return mean(a, b, c)
}

func main() {
	fmt.Println("counter", counter(), counter(), runs)
	classify(5)
	flags()
	perIteration()
	fmt.Println("average", average(1, 2, 4))
	var u uint8 = 250
	wrap := func(d uint8) uint8 { return u + d }
	fmt.Println(wrap(3), wrap(10))
	none := func() {}
	none()
}
