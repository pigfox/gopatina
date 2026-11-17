// Command borrows exercises borrows the checker infers, including a call
// that changes a variable a later argument reads.
package main

import "fmt"

type Counter struct {
	N    int
	Name string
}

func (c *Counter) Add(k int) int {
	c.N += k
	return c.N
}

func (c *Counter) Next() int {
	c.N++
	return c.N
}

func addTo(c *Counter, k int) {
	c.N += k
}

func show(c Counter, label string) {
	fmt.Println(label, c.Name, c.N)
}

func fill(xs []int, v int) {
	for i := range xs {
		xs[i] = v
	}
}

func main() {
	c := Counter{Name: "hits"}
	addTo(&c, c.N+5)
	c.Add(c.N)
	fmt.Println(c.Next(), c.Next())
	fmt.Println(c.N)
	show(c, c.Name)

	p := &c
	p.Add(1)
	addTo(p, 2)
	fmt.Println(p.N)
	fmt.Println(c.N)

	xs := make([]int, 3)
	fill(xs, len(xs))
	fmt.Println(xs)

	n := 1
	q := &n
	*q += 10
	*q = *q * 2
	fmt.Println(n)
}
