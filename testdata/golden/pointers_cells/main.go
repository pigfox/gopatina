// Variables whose address escapes live in cells (RFC 0013), and a field or
// slice inside a cell is lent to a call inside the cell's one borrow.
package main

import "fmt"

type In struct{ n int }

func (i *In) Inc() { i.n++ }

type T struct {
	in    In
	items []int
	name  string
	n     int
}

func (t T) Total() int { return sum(t.items) + t.n }

func (t T) Zero() { t.items[0] = 0 }

func sum(s []int) int {
	total := 0
	for _, x := range s {
		total += x
	}
	return total
}

func set(s []int, v int) { s[len(s)-1] = v }

func inc(x *int) { *x++ }

func keep(n int) *T { return &T{items: []int{1, 2, 3}, n: n} }

func addr(x float64) *float64 { return &x }

func pair() (*string, *string) {
	a, b := "a", "b"
	return &a, &b
}

type Counter struct{ hits int }

func hold(c *Counter) *Counter { return c }

func (c *Counter) Hit() int {
	c.hits++
	return c.hits
}

func main() {
	p := keep(4)
	p.in.Inc()
	p.in.Inc()
	inc(&p.n)
	set(p.items, 30)
	fmt.Println(p.in.n, p.n, p.items, sum(p.items), p.Total(), len(p.items))
	p.Zero()
	p.items = append(p.items, 7, 8)
	p.name += "ab"
	p.name = p.name + p.name
	p.n *= 3
	p.n -= p.n / 4
	fmt.Println(p.items, p.name, p.n)
	x := addr(5)
	y := x
	*y += *x
	fmt.Println(*x, *y, x == y, addr(1) == addr(1))
	a, b := pair()
	*a, *b = *b, *a
	fmt.Println(*a, *b)
	var c Counter
	keepC := hold(&c)
	c.Hit()
	keepC.Hit()
	fmt.Println(c.hits, keepC.hits, c.Hit()+keepC.Hit(), c.hits)
	cells := []int{c.hits, keepC.Hit(), c.hits}
	fmt.Println(cells)
	var q *T
	if q == nil {
		q = keep(1)
	}
	q.items[1] += 40
	q.in.n--
	fmt.Println(q.items[1], q.in.n, q != p)
}
