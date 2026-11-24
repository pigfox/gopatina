// Command owned_value_ptrs keeps numbers and strings behind pointers with
// one owner each. Nothing can reach itself through such a pointer, so
// Rust holds the value itself, T, or Option<T> where it may be nil, not a
// Box (RFC 0017).
package main

import "fmt"

// Score is a named number with methods.
type Score int

// Get reads a copy.
func (s Score) Get() int { return int(s) * 10 }

// Next changes its copy and returns it.
func (s Score) Next() Score {
	s++
	return s
}

// Add changes the score through the pointer.
func (s *Score) Add(v int) { *s += Score(v) }

// Title is a named string.
type Title string

// Loud reads a copy.
func (t Title) Loud() string { return string(t) + "!" }

// Shout changes its copy and returns it.
func (t Title) Shout() Title {
	t += "!!"
	return t
}

// maybeTitle returns nil for an empty string.
func maybeTitle(s string) *Title {
	if s == "" {
		return nil
	}
	t := Title(s)
	return &t
}

// newInt is never nil.
func newInt(v int) *int { return &v }

// Limits holds optional numbers.
type Limits struct {
	Max  *int
	Name *string
}

func newScore(v int) *Score {
	s := Score(v)
	return &s
}

func newTitle(s string) *Title {
	t := Title(s)
	return &t
}

// maybeInt returns nil for a negative value.
func maybeInt(v int) *int {
	if v < 0 {
		return nil
	}
	return &v
}

// orZero reads a pointer that may be nil.
func orZero(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func main() {
	s := newScore(4)
	s.Add(3)
	fmt.Println(*s, s.Get(), s.Next().Get(), *s)

	t := newTitle("go")
	u := *t
	u += "pher"
	fmt.Println(t.Loud(), u, *t, t.Shout(), maybeTitle("m").Shout())
	mt := maybeTitle("v")
	fmt.Println(newTitle("n").Shout(), mt.Shout(), *mt)
	w := newInt(5)
	fmt.Println(orZero(w), *w)

	l := Limits{Max: maybeInt(7)}
	fmt.Println(orZero(l.Max), orZero(maybeInt(-1)), l.Name == nil)
	if l.Max != nil {
		*l.Max += 1
	}
	name := "lim"
	l.Name = &name
	fmt.Println(*l.Max, *l.Name, orZero(l.Max))
	m := maybeInt(2)
	v := *m
	*m = v * 5
	fmt.Println(*m, v)
}
