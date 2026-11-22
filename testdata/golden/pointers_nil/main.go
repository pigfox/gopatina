package main

import "fmt"

type T struct{ n int }

func (t *T) Get() int { return t.n }

func (t *T) Safe() int {
	if t == nil {
		return -1
	}
	return t.n
}

func main() {
	var p *T
	fmt.Println(p.Safe())
	fmt.Println("before")
	fmt.Println(p.Get())
}
