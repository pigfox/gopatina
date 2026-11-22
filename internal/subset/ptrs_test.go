package subset

import (
	"go/types"
	"testing"

	"github.com/pigfox/gopatina/internal/load"
)

// TestPtrPlan checks which pointer types RFC 0013's analysis makes heap
// types, and which variables it puts in cells.
func TestPtrPlan(t *testing.T) {
	src := `package main

type Node struct{ Next *Node }

type B struct{ n int }

func keep(b *B) *B { return b }

func borrow(n *int) { *n++ }

func main() {
	var b B
	keep(&b)
	x := 1
	borrow(&x)
	var q **int
	_ = q
	r := &x
	pp := &r
	**pp = 2
}
`
	pkg, err := load.Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	p := Ptrs(pkg)
	if Ptrs(pkg) != p {
		t.Error("Ptrs is not made once")
	}
	if !p.Heap() {
		t.Error("Heap: false")
	}
	scope := pkg.Types.Scope()
	node := scope.Lookup("Node").Type()
	bType := scope.Lookup("B").Type()
	for _, tc := range []struct {
		t    types.Type
		heap bool
	}{
		{types.NewPointer(node), true},                                   // a field
		{types.NewPointer(bType), true},                                  // a result
		{types.Typ[types.Int], false},                                    // not a pointer
		{types.NewPointer(types.NewPointer(types.Typ[types.Int])), true}, // starts nil
		{types.NewPointer(types.Typ[types.Int]), true},                   // **pp: *int is held as a heap pointer
	} {
		if got := p.HeapType(tc.t); got != tc.heap {
			t.Errorf("HeapType(%s) = %v, want %v", tc.t, got, tc.heap)
		}
	}
	cells := map[string]bool{}
	for v := range p.Cells {
		cells[v.Name()] = true
	}
	if !cells["b"] || !cells["x"] || !cells["r"] {
		t.Errorf("cells %v, want b, x and r", cells)
	}
}
