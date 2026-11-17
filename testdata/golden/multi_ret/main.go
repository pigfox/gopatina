package main

import (
	"fmt"
	"strings"
)

func split3(s string) (a, b, c string) {
	a = strings.TrimSpace(s)
	b = strings.Repeat(a, 2)
	c = a + b
	return
}

func pair(x, y string) (string, string) { return y, x }

func named(n int) (res int, ok bool) {
	if n < 0 {
		return
	}
	res = n * n
	ok = true
	return res, ok
}

func main() {
	a, b, c := split3("  hi ")
	fmt.Println(a, b, c)
	x, y := pair(a, b)
	x, y = pair(x, y)
	x, y = y, x
	fmt.Println(x, y)
	for i := -1; i < 3; i++ {
		r, ok := named(i)
		fmt.Println(r, ok)
	}
	p, q, r := "p", "q", "r"
	p, q, r = q, r, p
	fmt.Println(p, q, r)
}
