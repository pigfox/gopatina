package subset

import (
	"go/ast"
	"sort"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/load"
)

// TestTakes checks which fields moved out are takes (RFC 0017, step 6):
// refilled by the next statement, in every form of statement that moves
// one out.
func TestTakes(t *testing.T) {
	src := `package main

type Tree struct {
	V           int
	Left, Right *Tree
}

func main() {
	t := &Tree{Left: &Tree{}, Right: &Tree{}}
	a := t.Left // a take
	t.Left = nil
	var b = t.Right // a take
	t.Right = a
	c := t.Right // not refilled next: no take
	_ = c
	const k = 1
	var d, e = t.Left, t.Right // two at once: no take
	t.Left = nil
	f := t.V // not a pointer
	t.V = f
	g := (t.Left) // a take, in parentheses
	t.Left = b
	t.V += k
	h := t.Left
	t.Left, t.Right = nil, nil // two at once: no take
	_, _, _, _ = b, d, e, h
	t.Right.Left = g
	i := t.Right.Left // through a field
	t.Right.Left = nil
	_ = i
}
`
	pkg, err := load.Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	p := Ptrs(pkg)
	var got []string
	ast.Inspect(pkg.Files[0], func(n ast.Node) bool {
		if e, ok := n.(ast.Expr); ok {
			if _, ok := p.Take(e); ok {
				got = append(got, pkg.Fset.Position(e.Pos()).String()[len("/p/main.go:"):])
			}
		}
		return true
	})
	sort.Strings(got)
	if want := "10:7 12:10 21:7 21:8 28:7"; strings.Join(got, " ") != want {
		t.Errorf("takes at %s, want %s", strings.Join(got, " "), want)
	}
}
