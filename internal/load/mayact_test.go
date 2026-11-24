package load

import (
	"go/ast"
	"testing"
)

// TestMayAct checks which package-level initializers can do something a
// program observes, so that Go's initialization of every package-level
// variable is kept for them even when nothing uses the variable.
func TestMayAct(t *testing.T) {
	src := `package main

func f() int { return 1 }

var s = []int{1, 2}
var m map[string]int
var ch = make(chan int)
var d = 3

var (
	constant = 1 << 3
	literal  = []int{1, 2, 3}
	length   = len("abc") + cap(s)
	conv     = float64(d)
	closure  = func() int { return f() }
	call     = f()
	index    = s[1]
	slice    = s[1:]
	assert   = any(d).(int)
	deref    = *&d
	divide   = 10 / d
	modulo   = 10 % d
	constDiv = 10 / 2
	receive  = func() bool { return true }() && len(ch) == 0
	builtin  = append(s, 3)
	lookup   = m["x"]
)

func main() {}
`
	pkg, err := Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"s": false, "m": false, "d": false, "ch": true,
		"constant": false, "literal": false, "length": false, "conv": false, "closure": false,
		"call": true, "index": true, "slice": true, "assert": true, "deref": true,
		"divide": true, "modulo": true, "constDiv": false, "receive": true, "builtin": true,
		"lookup": true,
	}
	seen := 0
	for _, d := range pkg.Files[0].Decls {
		ast.Inspect(d, func(n ast.Node) bool {
			if s, ok := n.(*ast.ValueSpec); ok {
				name := s.Names[0].Name
				if got := mayAct(pkg.Info, s); got != want[name] {
					t.Errorf("mayAct(%s) = %v, want %v", name, got, want[name])
				}
				seen++
			}
			return true
		})
	}
	if seen != len(want) {
		t.Errorf("checked %d specs, want %d", seen, len(want))
	}
	recv := `package main

var ch = make(chan int, 1)
var got = <-ch

func main() {}
`
	pkg, err = Source("/p", "main.go", recv, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	spec := pkg.Files[0].Decls[1].(*ast.GenDecl).Specs[0].(*ast.ValueSpec)
	if !mayAct(pkg.Info, spec) {
		t.Error("mayAct(<-ch) = false")
	}
}
