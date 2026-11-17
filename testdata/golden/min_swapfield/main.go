package main

import "fmt"

type Pair struct{ a, b string }

func (p *Pair) Swap() {
	p.a, p.b = p.b, p.a
}

func main() {
	p := Pair{"l", "r"}
	p.Swap()
	fmt.Println(p.a, p.b)
}
