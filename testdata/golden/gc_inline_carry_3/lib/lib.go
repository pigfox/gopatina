package lib

import "fmt"

type P9143 struct{ A, B int }

const K9143 = 866

func F9143(x, y int) int { return ((((x + x) & 22) - y) - y) / (y | 1) }

type N9143 int32

func (p P9143) Sum() int { return p.A + p.B*K9143 }

func (p *P9143) Grow(x int) {
	p.A ^= x
	p.B += x
}

func (n N9143) String() string { return fmt.Sprintf("n%d", int(n)) }
