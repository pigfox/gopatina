// Command zero_then_fields declares struct variables with no value and
// then sets their fields. Rust's clippy refuses a field set just after
// T::default() (field_reassign_with_default), so the fields set at once
// go into the initializer, and where a value reads the variable itself,
// every field's zero is spelled out instead.
package main

import "fmt"

type Point struct {
	X, Y int
	Name string
}

type Pair struct{ A, B int }

type Bag struct {
	Items []int
	N     int
}

type Counter struct{ N int }

func (c *Counter) Inc() { c.N++ }

func main() {
	// Two fields set at once: the initializer gives them.
	var p Point
	p.X = 3
	p.Name = "p"
	fmt.Println(p.X, p.Y, p.Name)

	// Every field set: no ..Default::default().
	var q Pair
	q.A = 1
	q.B = 2
	fmt.Println(q.A + q.B)

	// Changed again later: let mut.
	var r Point
	r.Y = 4
	r.X++
	fmt.Println(r.X, r.Y)

	// A value that reads s itself: the zeros spelled out.
	var s Point
	s.X = s.Y + 7
	fmt.Println(s.X, s.Name == "")

	// Not set at once: unchanged.
	var t Point
	fmt.Println(t.X, t.Name)
	t.Name = "later"
	fmt.Println(t.Name)

	// Assigned whole next: unchanged.
	var u Pair
	u = Pair{A: 5}
	fmt.Println(u.A)

	// Lent by & after: let mut.
	var w Point
	w.X = 1
	pw := &w
	pw.Y = 2
	fmt.Println(w.X, w.Y)

	// A method that changes it after: let mut.
	var c Counter
	c.N = 1
	c.Inc()
	fmt.Println(c.N)

	// An element changed after: let mut.
	var bag Bag
	bag.Items = []int{1, 2}
	n := 1
	pn := &n
	*pn = 3
	bag.Items[0] = 9 + n
	fmt.Println(bag.Items, bag.N)
}
