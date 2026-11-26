package subset

import (
	"go/ast"
	"go/types"
	"sort"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/load"
)

// TestRetBorrows checks which functions return a borrow of an input: a
// receiver, or the one pointer input, returned itself or as a place
// through it on every path, or nil, and never assigned, taken the address
// of or used in a function literal.
func TestRetBorrows(t *testing.T) {
	const src = `package main

import "fmt"

type Node struct {
	V    int
	Left *Node
	Kids []*Node
}

func mk() *Node { return &Node{V: 1, Left: &Node{}} }

func (n *Node) self() *Node      { return n }
func second(n *Node) *Node        { return n.Left }
func kid(n *Node) *Node           { return n.Kids[0] }
func deref(pp **Node) *Node       { return *pp }
func maybe(n *Node, b bool) *Node {
	if b {
		return nil
	}
	return n.Left
}
func pick(a, b *Node) *Node { return a } // two inputs: lent back from a, its lifetime named
func lit(n *Node) *Node { // used in a function literal: no
	f := func() int { return n.V }
	_ = f
	return n
}
func moved(n *Node) *Node { // assigned: no
	n = n.Left
	return n
}
func addr(n *Node) *Node { // its address taken: no
	q := &n
	_ = q
	return n
}
func other(n *Node) *Node { return mk() } // another pointer: no
func none(n *Node) *Node  { return nil }  // nil only: no
func count() int          { return 1 }
func value(n Node) *Node  { return n.Left } // no pointer input: no
func inc(n *Node) *Node   { n.V++; return n }        // changed through: lent back exclusively
func lend(n *Node) *Node  { touch(n); return n }     // lent to a call: exclusively
func meth(n *Node) *Node  { n.touch(); return n }    // a method on it: exclusively
func chain(n *Node) *Node { _ = second(n); return n } // lent back by second
func touch(n *Node)       {}
func shown(n *Node) *Node { _ = fmt.Sprint(n); return n } // handed to fmt: no
type Holder struct{ N *Node }
func store(n *Node) *Node { h := &Holder{N: n}; _ = h; return n } // stored: no
func alias(n *Node) *Node { m := n; _ = m; return n }             // copied: no
func same(n, m *Node) *Node { _ = n == m; return n }               // compared with another: no
func reads(n *Node) *Node { v := *n; _ = v.V + -n.V; _ = n == nil; _ = n.V > 0; return n }
func (n *Node) touch()    {}
func (n Node) val() *Node { return n.Left } // a value receiver: no

func main() {
	n := mk()
	pp := &n
	m := (*Node).self(n)
	_ = second(n).V + kid(n).V + deref(pp).V + maybe(n, false).V + m.V
	_ = pick(n, n) == nil
	_, _, _, _, _ = lit(n), moved(n), addr(n), other(n), none(n)
	_, _, _ = count(), value(*n), n.val()
	_, _, _, _ = inc(n), lend(n), meth(n), chain(n)
	_, _, _, _ = store(n), alias(n), same(n, n), reads(n)
	_ = shown(n)
}
`
	pkg, err := load.Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	p := Ptrs(pkg)
	var got []string
	for fn, v := range p.retBorrow {
		got = append(got, fn.Name()+"."+v.Name())
	}
	sort.Strings(got)
	if want := "chain.n deref.pp inc.n kid.n lend.n maybe.n meth.n pick.a reads.n second.n self.n"; strings.Join(got, " ") != want {
		t.Errorf("retBorrow = %s\nwant       %s", strings.Join(got, " "), want)
	}
	var mut []string
	for fn := range p.retBorrow {
		if p.RetMut(fn) {
			mut = append(mut, fn.Name())
		}
	}
	sort.Strings(mut)
	if want := "inc lend meth"; strings.Join(mut, " ") != want {
		t.Errorf("RetMut = %s, want %s", strings.Join(mut, " "), want)
	}
	// *Node is shared here (compared with another pointer), so no
	// function's result is an owned borrow.
	for fn := range p.retBorrow {
		if p.RetBorrow(fn) != nil {
			t.Errorf("RetBorrow(%s) of a shared type", fn.Name())
		}
	}
	var main *ast.FuncDecl
	for _, d := range pkg.Files[0].Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == "main" {
			main = fd
		}
	}
	ast.Inspect(main, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr); ok && sel.Sel.Name == "self" {
			if p.retBorrowCall(c) != nil {
				t.Error("retBorrowCall of a method expression")
			}
		}
		return true
	})
	if PlaceRoot(pkg.Info, &ast.BasicLit{}) != nil {
		t.Error("PlaceRoot of a literal")
	}
	_ = types.Typ
}

// TestRetBorrowUses checks the uses of a returned borrow that keep its
// type owned: passed on by another function lending the same input,
// compared with nil, read through; and a return inside a function
// literal, which lends nothing back.
func TestRetBorrowUses(t *testing.T) {
	const src = `package main

type Node struct {
	V    int
	Left *Node
}

func mk() *Node { return &Node{V: 1, Left: &Node{}} }

func second(k int, n *Node) *Node { return n.Left }

func third(n *Node) *Node { return second(0, n) }

func bump(n *Node) *Node {
	n.V++
	return n
}

func main() {
	n := mk()
	t := third(n)
	_ = t.V + n.V
	_ = second(1, n) == nil
	b := bump(n)
	if b != nil {
		b.V++
	}
	_ = n.V
}
`
	pkg, err := load.Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	p := Ptrs(pkg)
	node := types.NewPointer(pkg.Types.Scope().Lookup("Node").Type())
	if !p.Owned(node) {
		_, why, _ := p.SharedReason(node)
		t.Fatalf("*Node shared (%s), want owned", why)
	}
	second := pkg.Types.Scope().Lookup("second").(*types.Func)
	if v := p.RetBorrow(second); v == nil || v.Name() != "n" {
		t.Errorf("RetBorrow(second) = %v, want n", v)
	}
	for v := range p.refs {
		if v.Name() == "b" && (!p.LocalRef(v) || p.RefCall(v) == nil || p.RefSource(v).Name() != "n") {
			t.Errorf("b := bump(n): LocalRef = %v, RefSource = %v", p.LocalRef(v), p.RefSource(v))
		}
	}
	lit := strings.Replace(src, "func main() {\n", "func main() {\n\tf := func(n *Node) *Node { return n.Left }\n\t_ = f\n", 1)
	pkg, err = load.Source("/p", "main.go", lit, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	if p := Ptrs(pkg); p.Owned(types.NewPointer(pkg.Types.Scope().Lookup("Node").Type())) {
		t.Error("a function literal returning a field of its pointer: *Node owned")
	}
}

// TestWalkers checks the functions that walk from their input to the
// place they return (returns.go): the walker is only ever given the
// input, a field of itself or nil, and only read, compared and returned.
func TestWalkers(t *testing.T) {
	const src = `package main

import "fmt"

type Node struct {
	V    int
	Next *Node
	Kids []*Node
}

func mk() *Node { return &Node{V: 1, Next: &Node{V: 2}} }

func find(head *Node, v int) *Node {
	for p := head; p != nil; p = p.Next {
		if (p).V == v && p.Kids == nil {
			return p
		}
	}
	return nil
}

func last(head *Node) *Node {
	p := head
	for p.Next != nil {
		p = p.Next
	}
	return p
}

func lit(head *Node) *Node {
	p := head
	f := func() int { return p.V }
	_ = f
	return p
}

func two(head *Node) *Node {
	p, q := head, head
	_ = q
	return p
}

func call(head *Node) *Node {
	p, q := split(head)
	_ = q
	return p
}

func split(n *Node) (*Node, *Node) { return n, n }

func other(head *Node) *Node {
	p := head
	p = mk()
	return p
}

func deref(head **Node) *Node {
	p := *head
	return p
}

func byVar(head *Node) *Node {
	var p = head
	return p
}

func written(head *Node) *Node {
	p := head
	p.V = 3
	return p
}

func deep(head *Node) *Node {
	p := head
	p.Next.V = 3
	return p
}

func bumped(head *Node) *Node {
	p := head
	p.V++
	return p
}

func addr(head *Node) *Node {
	p := head
	q := &p.V
	_ = q
	return p
}

func neg(head *Node) *Node {
	p := head
	_ = -p.V
	return p
}

func method(head *Node) *Node {
	p := head
	p.touch()
	return p
}

func (n *Node) touch() {}

func lent(head *Node) *Node {
	p := head
	fmt.Println(p)
	return p
}

func main() {
	n := mk()
	fmt.Println(find(n, 2).V, last(n).V, lit(n).V, two(n).V, call(n).V, other(n).V)
	fmt.Println(byVar(n).V, written(n).V, deep(n).V, bumped(n).V, addr(n).V, neg(n).V, method(n).V, lent(n).V)
	d := mk()
	fmt.Println(deref(&d).V)
}
`
	pkg, err := load.Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	p := Ptrs(pkg)
	want := map[string]bool{"find": true, "last": true, "neg": true}
	for _, name := range []string{"find", "last", "lit", "two", "call", "other", "deref", "byVar", "written", "deep", "bumped", "addr", "neg", "method", "lent"} {
		fn := pkg.Types.Scope().Lookup(name).(*types.Func)
		if got := p.retBorrow[fn] != nil; got != want[name] {
			t.Errorf("%s lends its input back = %v, want %v", name, got, want[name])
		}
	}
	walked := 0
	for v := range p.walked {
		walked++
		if v.Name() != "p" {
			t.Errorf("walker %s", v.Name())
		}
	}
	if walked != 3 {
		t.Errorf("%d walkers, want 3", walked)
	}
}

// TestBareReturns checks returns that lend a borrow back with no result
// written (found by gopatina-parity in regexp and text/template, where a
// bare return crashed the analysis): a named result never assigned is nil,
// as return nil is; one assigned, or whose address is taken, is returned
// as itself; and a function literal's returns are not the function's.
func TestBareReturns(t *testing.T) {
	const src = `package main

import "fmt"

type Node struct {
	V    int
	Next *Node
}

func find(n *Node, v int) (r *Node) {
	if n.V == v {
		return n
	}
	return
}

func next(n *Node) (r *Node) {
	r = n.Next
	return
}

func held(n *Node) (r *Node) {
	q := &r
	_ = q
	return
}

func inner(n *Node) *Node {
	f := func() *Node { return nil }
	_ = f
	return n
}

func main() {
	l := &Node{V: 1, Next: &Node{V: 2}}
	fmt.Println(find(l, 1) != nil, next(l) != nil, held(l) == nil, inner(l) != nil)
}
`
	pkg, err := load.Source("/bare", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	p := Ptrs(pkg)
	fn := func(name string) *types.Func { return pkg.Types.Scope().Lookup(name).(*types.Func) }
	if p.RetBorrow(fn("find")) == nil {
		t.Error("find: a bare return of a named result never assigned is nil; find lends n back")
	}
	for _, name := range []string{"next", "held", "inner"} {
		if p.RetBorrow(fn(name)) != nil {
			t.Errorf("%s: lends its input back", name)
		}
	}
}
