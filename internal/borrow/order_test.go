package borrow

import (
	"go/ast"
	"testing"

	"github.com/pigfox/gopatina/internal/load"
)

// TestChangedBeforeRead checks which statements read a variable, or a
// place under one, before a call that changes it.
func TestChangedBeforeRead(t *testing.T) {
	const src = `package main

type In struct{ V int }

func (i *In) Bump() int { i.V++; return i.V }

type Out struct{ In In }

func addTo(p *int, v int) int { *p += v; return *p }

func first(s []int) int { s[0]++; return s[0] }

func peek(p *int) int { return *p }

type Box struct{ Xs []int }

func get() Box { return Box{Xs: []int{1}} }

func main() {
	var o Out
	x := 1
	s := [][]int{{1}}
	p := &x
	ps := &s
	_ = (*ps)[0][0] + first((*ps)[0])
	_ = o.In.V + addTo(&o.In.V, 1)
	_ = o.In.V + o.In.Bump()
	_ = s[0][0] + first(s[0])
	_ = *p + addTo(p, 1)
	_ = x + first(get().Xs)
	_ = x + peek(&x)
	_ = addTo(&x, 1) + x
	_ = peek(&x) + addTo(&x, 1)
}
`
	pkg, err := load.Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	res, _ := Check(pkg)
	var stmts []ast.Stmt
	for _, d := range pkg.Files[0].Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == "main" {
			stmts = fd.Body.List[5:]
		}
	}
	want := []bool{true, true, true, true, true, false, false, false, false}
	for i, s := range stmts {
		if got := ChangedBeforeRead(pkg.Info, res.Funcs, s); got != want[i] {
			t.Errorf("statement %d: ChangedBeforeRead = %v, want %v", i, got, want[i])
		}
	}
}
