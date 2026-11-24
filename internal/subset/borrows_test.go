package subset

import (
	"go/types"
	"sort"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/load"
)

// TestBorrows checks which locals are temporary borrows (RFC 0017, step
// 5), and what each borrows from: its owners, not other borrows.
func TestBorrows(t *testing.T) {
	src := `package main

import "fmt"

type List struct {
	V    int
	Next *List
}

var global *List

func mk() *List { return &List{V: 1, Next: &List{V: 2}} }

func read(l *List) int { return l.V }

func set(l *List) { l.V = 3 }

func setVia(l *List) { set(l) }

func main() {
	head := mk()
	p := head        // a borrow
	var q = p        // a borrow of a borrow: head's
	r := head.Next   // a borrow of a field
	var s *List      // only ever nil: owns its nil, no borrow
	w := head        // written through: not a borrow
	w.V = 2
	x := mk()        // a new pointer: an owner
	y := x           // lent to a reader: a borrow
	z := x           // lent to a changer, through another: not a borrow
	setVia(z)
	global = mk()
	fmt.Println(p.V, q.V, r.V, s == nil, read(y))
}
`
	pkg, err := load.Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	p := Ptrs(pkg)
	var got []string
	for id, obj := range pkg.Info.Defs {
		v, ok := obj.(*types.Var)
		if !ok || !p.BorrowVar(v) {
			continue
		}
		var from []string
		for s := range p.BorrowSources(v) {
			from = append(from, s.Name())
		}
		sort.Strings(from)
		got = append(got, id.Name+"<"+strings.Join(from, ",")+">")
	}
	sort.Strings(got)
	if want := "p<head> q<head> r<head> y<x>"; strings.Join(got, " ") != want {
		t.Errorf("borrows = %s, want %s", strings.Join(got, " "), want)
	}
	for _, obj := range pkg.Info.Defs {
		if v, ok := obj.(*types.Var); ok && p.RangeElem(v) {
			t.Errorf("RangeElem(%s): there is no range", v.Name())
		}
	}
}
