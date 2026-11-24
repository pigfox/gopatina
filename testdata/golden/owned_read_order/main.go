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

func main() {
	a := mk(1)
	fmt.Println(a.V+say("first"), get(a.Next)+say("second"))
	x := a.V + get(a.Next)
	y := get(a) + a.Next.V + say("third")
	fmt.Println(x, y, a.V+get(a))
	both := a.V > 0 && say("and") > a.Next.V
	fmt.Println(both)
	var z *N
	fmt.Println(z.V + say("arg"))
}
