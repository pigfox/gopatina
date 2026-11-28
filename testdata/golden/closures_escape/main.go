// Command closures_escape returns closures from functions (RFC 0018, step
// 3): each owns the numbers it captures, moved in, so every call of the
// function that makes it starts a state of its own. A variable holding one
// is only called, or passed to a parameter of function type.
package main

import "fmt"

func intSeq() func() int {
	i := 0
	return func() int {
		i++
		return i
	}
}

func adder() func(int) int {
	sum := 0
	return func(x int) int {
		sum += x
		return sum
	}
}

func fibonacci() func() int {
	f, g := 1, 0
	return func() int {
		f, g = g, f+g
		return f
	}
}

func step(up bool, by int) func(int) int {
	if up {
		return func(x int) int { return x + by }
	}
	return func(x int) int { return x - by }
}

type threshold struct {
	limit float64
}

func (t threshold) above() func(float64) bool {
	limit := t.limit
	return func(x float64) bool { return x > limit }
}

// applyTwice computes the inner call first: with closures that change what
// they capture passed to it, f(f(x)) would borrow f twice at once in Rust.
func applyTwice(f func(int) int, x int) int {
	y := f(x)
	return f(y)
}

func main() {
	nextInt := intSeq()
	fmt.Println(nextInt(), nextInt(), nextInt())
	newInts := intSeq()
	fmt.Println(newInts())

	pos, neg := adder(), adder()
	for i := 0; i < 5; i++ {
		fmt.Println(pos(i), neg(-2*i))
	}

	fib := fibonacci()
	for i := 0; i < 10; i++ {
		fmt.Print(fib(), " ")
	}
	fmt.Println()

	up, down := step(true, 3), step(false, 3)
	fmt.Println(up(10), down(10), applyTwice(up, 1))

	hot := threshold{30.5}.above()
	fmt.Println(hot(12), hot(31), hot(30.5))

	counter := adder()
	fmt.Println(applyTwice(counter, 5), counter(0))
}
