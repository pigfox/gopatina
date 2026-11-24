// Command owned_plain_values copies values out of owned pointers and calls
// their value methods. The types hold numbers and strings only, so Go's
// copy shares nothing, and Rust copies a Copy type (*p) and clones the
// others, while the pointers keep one owner (RFC 0017).
package main

import "fmt"

// Rect is Copy in Rust: numbers only.
type Rect struct{ W, H int }

// Area is a value method.
func (r Rect) Area() int { return r.W * r.H }

// Grown returns a copy one larger each way.
func (r Rect) Grown() Rect {
	r.W++
	r.H++
	return r
}

// Label holds a string: Clone, not Copy.
type Label struct {
	Text string
	N    int
}

// Show is a value method.
func (l Label) Show() string { return fmt.Sprint(l.Text, "#", l.N) }

// Renamed changes its copy and returns it.
func (l Label) Renamed(s string) Label {
	l.Text = s
	return l
}

func newRect(w, h int) *Rect { return &Rect{W: w, H: h} }

func newLabel(s string) *Label { return &Label{Text: s, N: 1} }

// maybe returns nil for a negative width.
func maybe(w int) *Rect {
	if w < 0 {
		return nil
	}
	return &Rect{W: w, H: 1}
}

// maybeLabel returns nil for an empty text.
func maybeLabel(s string) *Label {
	if s == "" {
		return nil
	}
	return &Label{Text: s}
}

// area reads a value method through a borrowed pointer.
func area(r *Rect) int { return r.Area() + r.Grown().Area() }

func main() {
	p := newRect(2, 3)
	r := *p
	r.W = 10
	fmt.Println(p.Area(), r.Area(), p.W, r.W, area(p))
	*p = Rect{W: 4, H: 4}
	fmt.Println(p.Area(), p.Grown().W)

	l := newLabel("a")
	c := *l
	c.Text = "b"
	l.N = 5
	fmt.Println(l.Show(), c.Show(), l.Renamed("x").Show(), l.Text)

	m := maybe(3)
	fmt.Println(m.Area(), (*m).H)
	mv := *m
	if m != nil {
		*m = Rect{W: 9, H: 9}
	}
	fmt.Println(mv.W, m.Area())
	*m = Rect{W: 1, H: 2}
	ml := maybeLabel("m")
	fmt.Println(m.Area(), ml.Renamed("n").Show(), ml.Text)
}
