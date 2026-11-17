// Command corners exercises less common corners of the subset, so that
// each has a golden translation and a differential test.
//
// It is not meant to be read as an example.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Prefix is printed before each section.
const Prefix = ">> "

// Empty has no fields.
type Empty struct{}

type Point struct{ X, Y int }

func (p *Point) Scale(k int) {
	p.X *= k
	p.Y *= k
}

func (p Point) Sum() int { return p.X + p.Y }

func makePoint() Point { return Point{2, 3} }

type Tag struct{ Name string }

func same(a, b Tag) bool { return a == b }

func scale(p *Point, k int) { p.Scale(k) }

func squares(n int) []int {
	out := make([]int, n)
	for i := range n {
		out[i] = i * i
	}
	return out
}

func report(w io.Writer, b []byte, n int) {
	fmt.Fprintf(w, "%s=%d\n", b, n)
}

func find(xs []int, want int) (int, error) {
	for i, x := range xs {
		if x == want {
			return i, nil
		}
	}
	return 0, errors.New("not found")
}

func lookup(xs []int, want int) (int, error) {
	i, err := find(xs, want)
	if err != nil {
		fmt.Println("lookup failed:", err)
		return 0, err
	}
	return i, nil
}

func main() {
	fmt.Println(Prefix+"points", Empty{} == Empty{})
	p := Point{1, 1}
	(&p).Scale(2)
	pp := &p
	(*pp).Scale(3)
	(*pp).X = 10
	q := &p
	fmt.Println(q.X, q.Y, makePoint().Sum(), +p.X)
	scale(&Point{1, 2}, 5)
	box := &Point{X: 4}
	*box = Point{7, 8}
	fmt.Println(box.X, squares(4)[2], same(Tag{"a"}, Tag{"a"}))
	_ = &p

	fmt.Println(Prefix + "strings")
	s := "quote\" back\\ cr\r nul\x00 bell\x07"
	fmt.Println(len(s))
	word := "héllo"
	bs := []byte(word)
	raw := []byte("héllo wörld")
	fmt.Println(len(bs), len(raw), len(word) < 10, (len(word) == 6) == (len(raw) == 13))
	fmt.Printf("%s and %d\n", raw, len(raw))
	report(os.Stdout, raw, 1)
	fmt.Fprintf(os.Stderr, "%s\n", raw)
	names := make([]string, 2)
	names[1] = strings.Repeat("ab", 2)
	fmt.Println(names, len(names[0]))
	long := "this line is long enough that the Rust statement holding it cannot fit in one hundred columns"
	fmt.Println(len(long))

	fmt.Println(Prefix + "assignments")
	a, b := 17, 5
	a /= b
	b %= a
	c := 0xF0
	c &^= 0x30
	x, _ := find([]int{1, 2}, 2)
	var _ = makePoint()
	x, _ = find([]int{3}, 3)
	a, d := 1, 2
	fmt.Println(a, b, c, x, d)
	rows := [][]int{{1}}
	rows = append(rows, nil)
	fmt.Println(len(rows), len(rows[1]))
	grid := [][]int{{1, 2}, {3, 4}}
	grid[1][len(grid[1])-1] = 9
	fmt.Println(grid[1][1])
	t := Point{}
	t.X, t.Y = t.Y+1, t.X+2
	fmt.Println(t.X, t.Y)
	label := ""
	label = strings.Repeat("-", 3)
	fmt.Println(label)

	fmt.Println(Prefix + "control")
	for i := 0; i < 4; i++ {
		switch i {
		case 1:
			continue
		case 2:
		default:
			fmt.Print(i, " ")
		}
	}
	fmt.Println()
	if n := len(names); n > 5 {
		fmt.Println("many")
	} else if m := n * 2; m > 3 {
		fmt.Println("doubled", m)
	}
	switch k := len(s); {
	case k > 100:
		fmt.Println("huge")
	default:
		fmt.Println("small", k)
	}
	switch y := 2.5; y {
	case 1.5, 2.5:
		fmt.Println("float match")
	}
	switch {
	default:
		fmt.Println("only default")
	}
	for range names {
		fmt.Print(".")
	}
	for mut := range 2 {
		mut++
		fmt.Print(mut)
	}
	tags := []Tag{{"x"}, {"y"}}
	for _, tag := range tags {
		tag.Name += "!"
		fmt.Print(tag.Name)
	}
	for i, tag := range tags {
		tags[i].Name = tag.Name + "?"
	}
	for i := range tags {
		tags = append(tags, Tag{"z"})
		i++
		fmt.Print(i)
	}
	for i, v := range squares(3) {
		fmt.Print(i, v)
	}
	for v := range squares(2) {
		fmt.Print(v)
	}
	fmt.Println(len(tags), tags[0].Name)

	fmt.Println(Prefix + "errors")
	if i, err := lookup([]int{5}, 5); err != nil {
		fmt.Println("no")
	} else {
		fmt.Println("found at", i)
	}
	if _, err := lookup([]int{5}, 6); err != nil {
		fmt.Println("missing:", err)
	} else if len(names) > 1 {
		fmt.Println("never")
	}
	code := 0
	if len(names) > 1 {
		code = 3
	}
	os.Exit(code)
}
