// Command consume_copy reads a value in one operand and hands it by value
// to a call in a later one, as in fmt.Println(k, pass(k)). Go passes a
// copy, so both operands see the same value; patina mode copies it too
// (once rejected as "another operand changes k").
package main

import "fmt"

type Pair struct {
	A string
	N int
}

func pass(s string) string {
	s += "!"
	return s
}

func bump(p Pair) Pair {
	p.N++
	p.A += "+"
	return p
}

func main() {
	k := "x"
	fmt.Println(k, pass(k), k)
	p := Pair{A: "a", N: 1}
	fmt.Println(p.A, bump(p).A, p.N, bump(bump(p)).N)
	s := k + pass(k) + k
	fmt.Println(s, len(s), fmt.Sprint(k, pass(pass(k))))
}
