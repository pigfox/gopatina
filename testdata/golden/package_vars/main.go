// Package-level variables (RFC 0010): Go's initialization order (d, b, c,
// a below), read-only tables, changed counters of several kinds, a changed
// string, and gc's order of calls and reads in one expression (calls
// first).
package main

import "fmt"

var (
	a = c + b // initialized third: it needs b and c
	b = f("b")
	c = f("c")
	d = 3 // initialized first: b and c need it through f
)

var count int

var names = []string{"x", "y"}

var primes = []int{2, 3, 5, 7, 11}

type Stat struct {
	N   int32
	Sum float64
	On  bool
}

var stat Stat

// Empty is never changed and has no initializer: its zero value.
var empty []int

// Tick is a changed struct without fields.
type Tick struct{}

var tick Tick

var _ = note("blank")

// Name is a read-only string, copied out.
var name = "gopatina"

// Limit is never changed and has no initializer.
var limit int

func twice(x int) int { return 2 * x }

var (
	small int8 = 120
	flag  bool
	ratio float32 = 0.5
	log           = "start"
)

func f(s string) int {
	count++
	fmt.Println("init", s, count, d)
	return d * count
}

func init() { fmt.Println("init()", a, b, c) }

func next() int {
	count++
	return count
}

func sum(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

func note(s string) string {
	log += "," + s
	return s
}

func grow(xs []int, n int) int {
	xs[0] += n
	return xs[0] + count
}

func fresh() []int { return []int{1, 2} }

func greet(s string, n int) string { return s + fmt.Sprint(n) }

func tally() int { return count + next() }

func record(x float64) {
	stat.N++
	stat.Sum += x
	stat.On = stat.N > 1
}

func main() {
	fmt.Println(a, b, c, d, next(), next(), names[1], len(names))
	x := count + next() // gc calls next first
	fmt.Println(x, count)
	count += next()
	fmt.Println(count, next()+count)
	for _, p := range primes {
		record(float64(p))
	}
	fmt.Println(stat.N, stat.Sum, stat.On, sum(primes), primes, primes[2])
	small += 10
	flag = !flag
	ratio *= 3
	fmt.Println(small, flag, ratio)
	log = log + "!" + note("a")
	fmt.Println(log, len(log))
	stat = Stat{N: 1}
	fmt.Println(stat.N, stat.Sum)
	// Calls first, also as arguments: a string, a slice changed in place, a
	// copied value.
	fmt.Println(greet(note("b"), count), grow(fresh(), count), count+next(), tally())
	record(float64(count))
	ok := count > 0 && next() > 0
	vs := []int{0, 0}
	vs[0] = count + next()
	count += count
	log += log
	tick = Tick{}
	_ = tick
	n := name
	m := len([]byte(log)) + count
	fmt.Println(twice(next()), count, limit, m)
	fmt.Println(ok, vs, count, len(log), len(empty), n, count&(count+1), stat.N)
}
