package main

import "fmt"

type Pair struct {
	a, b string
	n    int
}

func (p *Pair) Swap() {
	p.a, p.b = p.b, p.a
	p.n++
}

func main() {
	p := Pair{"left", "right", 0}
	for i := 0; i < 3; i++ {
		p.Swap()
		fmt.Println(p.a, p.b, p.n)
	}
	q := Pair{a: "x"}
	p.a, q.a = q.a, p.a
	fmt.Println(p.a, q.a, p == q, p != q)
	s := []string{"a", "b", "c"}
	s[0], s[2] = s[2], s[0]
	i := 0
	i, s[i] = 2, "changed"
	fmt.Println(s, i)
}
