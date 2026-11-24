// Command owned_read_order reads through owned pointers (RFC 0016) beside
// calls that print. Go (gc) makes an expression's calls before it reads
// the variables, so the last line prints "arg" and only then panics on the
// nil pointer; GoPatina makes the call first too. A read inside a call's
// arguments comes before the calls after it, and a read through p before a
// call that reads through p itself panics on the same nil in either order,
// so those need no reordering.
package main

import "fmt"

type N struct {
	V    int
	Next *N
}

func say(s string) int {
	fmt.Println(s)
	return 1
}

func mk(v int) *N { return &N{V: v, Next: &N{V: v + 1}} }

func get(n *N) int { return n.V }

// both lends its parameter on.
func both(n *N) int { return get(n) + get(n) }

// Acc's add takes enough to break a call over lines.
type Acc struct{ V int }

func (c Acc) add(a, b, d int) int { return c.V + a + b + d }

func keep(x int) int { return x }

func main() {
	a := mk(1)
	fmt.Println(a.V+say("first"), get(a.Next)+say("second"))
	x := a.V + get(a.Next)
	y := get(a) + a.Next.V + say("third")
	fmt.Println(x, y, a.V+get(a))
	and := a.V > 0 && say("and") > a.Next.V
	either := get(a) > 5 || say("or") > a.Next.V
	big, bigger := 1000000000000000, 2000000000000000
	acc := Acc{V: a.V}
	total := keep(acc.add(big*big+bigger*bigger+big, bigger*bigger*big+big*big, bigger*big+bigger))
	fmt.Println(and, either, both(a), total)
	// A new pointer made first, by the order above, then lent.
	fmt.Println(a.Next.V, get(mk(3)))
	var z *N
	fmt.Println(z.V + say("arg"))
}
