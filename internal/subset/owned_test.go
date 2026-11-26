package subset

import (
	"go/ast"
	"go/types"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/load"
)

// ownedOf loads src and returns its pointer plan and *Node.
func ownedOf(t *testing.T, src string) (*PtrPlan, types.Type) {
	t.Helper()
	pkg, err := load.Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	return Ptrs(pkg), types.NewPointer(pkg.Types.Scope().Lookup("Node").Type())
}

// tree is binary-trees' shape: built from fresh pointers, then only read
// through, tested for nil and lent.
const tree = `package main

import "fmt"

type Node struct {
	Left, Right *Node
	V           int
	Name        string
}

func bottomUp(d int) *Node {
	if d <= 0 {
		return &Node{V: 1}
	}
	if d == 1 {
		return &Node{bottomUp(0), (bottomUp(0)), 1, "one"}
	}
	n := &Node{Left: bottomUp(d - 1), Right: bottomUp(d - 1)}
	return n
}

func (n *Node) check() int {
	if n.Left == nil || nil == n.Right {
		return n.V
	}
	return 1 + n.Left.check() + count(n.Right)
}

func count(n *Node) int { return n.check() }

var top = count(bottomUp(1))

func main() {
	long := bottomUp(4)
	var spare *Node = new(Node)
	spare = bottomUp(1)
	spare = nil
	bottomUp(2)
	fmt.Println(long.check(), bottomUp(3).check(), count(bottomUp(1)), (long).V, spare == nil)
	fmt.Println(long == spare, long.Left != long.Right, long == bottomUp(0)) // by address: std::ptr::eq
}
`

// TestOwnedTree checks that binary-trees' shape is owned.
func TestOwnedTree(t *testing.T) {
	p, node := ownedOf(t, tree)
	if !p.Owned(node) {
		pos, why, _ := p.SharedReason(node)
		t.Fatalf("*Node is shared (%v: %s), want owned", pos, why)
	}
	if _, _, ok := p.SharedReason(node); ok {
		t.Error("SharedReason of an owned type: ok")
	}
	if p.Owned(types.Typ[types.Int]) || p.Owned(types.NewPointer(types.Typ[types.Bool])) {
		t.Error("Owned of a non-heap type: true")
	}
	if _, _, ok := p.SharedReason(types.Typ[types.Int]); ok {
		t.Error("SharedReason of a non-pointer: ok")
	}
}

// TestOwnedWrites checks that changing numbers and strings in place,
// through a variable, a field or a parameter, keeps a type owned (RFC
// 0017, step 3), and so does a call that changes one operand while it
// reads another variable.
func TestOwnedWrites(t *testing.T) {
	src := strings.Replace(tree, "func main() {\n", `func set(n *Node, v int) { n.V = v }

func (n *Node) rename(s string) { n.Name = s }

func main() {
	w := bottomUp(2)
	w.V = 2
	w.V++
	w.Name += "x"
	w.Left.V--
	(w.Left).V = 1
	w.Left = bottomUp(0)
	x := bottomUp(1)
	w.Right = x
	var y = bottomUp(1)
	w.Left = y
	s := []*Node{bottomUp(1), w}
	s = append(s, bottomUp(2))
	s[0].V++
	set(w, 3)
	set(w.Left, w.Right.V)
	w.rename("w")
	v := w.V
	set(w, v)
`, 1)
	p, node := ownedOf(t, src)
	if !p.Owned(node) {
		pos, why, _ := p.SharedReason(node)
		t.Fatalf("*Node is shared (%v: %s), want owned", pos, why)
	}
	elem := node.(*types.Pointer).Elem()
	if !p.Chained(elem) {
		t.Error("*Node moved into a field: not Chained")
	}
	if p, node := ownedOf(t, tree); p.Chained(node.(*types.Pointer).Elem()) {
		t.Error("binary-trees' Node, built only by new nested pointers: Chained")
	}
}

// TestOwnerStruct checks that struct values holding owned pointers and
// data own them (RFC 0017): declared, built by a literal, given a new
// pointer in a field.
func TestOwnerStruct(t *testing.T) {
	src := strings.Replace(tree, "func main() {\n", `type Pair struct {
	A, B *Node
	N    int
	Kids []*Node
}

func main() {
	var p Pair
	p.A = bottomUp(1)
	q := Pair{A: bottomUp(2), N: 1}
	r := &Pair{}
	r.B = bottomUp(0)
	_, _ = q.A.V, r.B.V
`, 1)
	if p, node := ownedOf(t, src); !p.Owned(node) {
		_, why, _ := p.SharedReason(node)
		t.Fatalf("*Node shared (%s), want owned", why)
	}
}

// TestOwnedKept checks parameters their function keeps (RFC 0017, step
// 8): a tree's insert, given a child taken out of the field its result
// refills, and a parameter passed on to another keeper stay owned; a
// field not refilled cannot be handed to a keeper.
func TestOwnedKept(t *testing.T) {
	src := strings.Replace(tree, "func main() {\n", `func insert(t *Node, v int) *Node {
	if t == nil {
		return &Node{V: v}
	}
	t.Left = insert(t.Left, v)
	return t
}

func relay(t *Node) *Node { return insert(t, 1) }

func main() {
	var root *Node
	root = insert(root, 2)
	root = relay(root)
`, 1)
	if p, node := ownedOf(t, src); !p.Owned(node) {
		_, why, _ := p.SharedReason(node)
		t.Fatalf("*Node shared (%s), want owned", why)
	}
	walk := strings.Replace(src, "func main() {\n", "func walk(t *Node) int {\n\tn := 0\n\tfor p := t; p != nil; p = p.Left {\n\t\tn++\n\t}\n\treturn n\n}\n\nfunc main() {\n\t_ = walk(root.Left)\n", 1)
	walk = strings.Replace(walk, "_ = walk(root.Left)\n", "", 1)
	walk = strings.Replace(walk, "\troot = relay(root)\n", "\troot = relay(root)\n\t_ = walk(root.Left)\n", 1)
	p, node := ownedOf(t, walk)
	if !p.Owned(node) {
		_, why, _ := p.SharedReason(node)
		t.Errorf("a field lent to a parameter only a temporary borrow takes: *Node shared (%s), want owned", why)
	}
	src = strings.Replace(src, "func main() {\n", "func lend(t *Node) int { return insert(t.Left, 1).V }\n\nfunc main() {\n\t_ = lend(bottomUp(1))\n", 1)
	if p, node := ownedOf(t, src); p.Owned(node) {
		t.Error("a field not refilled handed to a keeper: *Node owned")
	}
}

// TestOwnerErrors checks that assigning a parameter a new pointer keeps
// the type owned and is refused, and that it is not refused where the
// type is shared anyway (its cells express it).
func TestOwnerErrors(t *testing.T) {
	src := strings.Replace(tree, "func main() {\n", "func swap(n *Node) int { n = bottomUp(0); return n.V }\n\nfunc main() {\n\t_ = swap(bottomUp(1))\n", 1)
	p, node := ownedOf(t, src)
	if !p.Owned(node) {
		t.Fatal("*Node shared, want owned")
	}
	if errs := p.ownerErrors(); len(errs) != 1 || !strings.Contains(errs[0].msg, "parameter n is assigned another pointer") {
		t.Errorf("ownerErrors = %v", errs)
	}
	shared := strings.Replace(src, "func main() {\n", "func main() {\n\tx := bottomUp(1)\n\tf := func() int { return x.V }\n\t_ = f()\n", 1)
	if p, _ := ownedOf(t, shared); len(p.ownerErrors()) != 0 {
		t.Errorf("ownerErrors of a shared type = %v", p.ownerErrors())
	}
}

// TestOwnedShared checks each use that shares a pointer type.
func TestOwnedShared(t *testing.T) {
	for _, tc := range []struct {
		name, decl, body, why string
	}{
		{"copy", "", "a := bottomUp(1)\n\tb := a\n\t_ = b", "a copy of it is kept"},
		{"return a place kept", "func first(n *Node) *Node { return n.Left }", "k := bottomUp(1)\n\troot := &Node{Left: first(k)}\n\t_ = root", "a borrow a call returns is kept"},
		{"parameter assigned a variable", "func swap(n *Node, m *Node) int { n = m; return n.V }", "_ = swap(bottomUp(1), bottomUp(1))", "a copy of it is kept"},
		{"address of a field", "", "a := bottomUp(1)\n\tq := &a.V\n\t*q = 1", "written through"},
		{"package variable", "var root *Node", "root = bottomUp(1)", "a package-level variable"},
		{"package-level slice", "var all []*Node", "all = append(all, bottomUp(1))", "a slice, map, array, channel or struct value"},
		{"element of a call's result", "func some() []*Node { return nil }", "_ = some()[0].V", "a pointer is computed in a way that may share it"},
		{"struct value holding a struct of more than data", "type In struct{ X any }\n\ntype Pair struct {\n\tA  *Node\n\tIn In\n}", "var p Pair\n\t_ = p", "a field of a struct that is not boxed"},
		{"struct value holding more than data", "type Pair struct {\n\tA *Node\n\tX any\n}", "var p Pair\n\t_ = p", "a field of a struct that is not boxed"},
		{"value outside its box", "", "var v Node\n\t_ = v", "held outside its box"},
		{"address of a dereference", "", "a := bottomUp(1)\n\tq := &*a\n\t_ = q", "computed in a way that may share it"},
		{"dereference", "", "a := bottomUp(1)\n\tv := *a\n\t_ = v", "outside its box"},
		{"closure", "", "a := bottomUp(1)\n\tf := func() int { return a.V }\n\t_ = f()", "a function literal captures it"},
		{"method value", "", "a := bottomUp(1)\n\tf := a.check\n\t_ = f", "a copy of it is kept"},
		{"value method", "func (n Node) get() int { return n.V }", "_ = bottomUp(1).get()", "outside its box"},
		{"deferred", "func use(n *Node) {}", "a := bottomUp(1)\n\tdefer use(a)", "a copy of it is kept"},
		{"fresh compared", "", "_ = bottomUp(1) == nil", "a new pointer is stored"},
		{"function value", "", "f := bottomUp\n\t_ = f(1).V", "computed in a way that may share it"},
		{"lent to a non-program function", "", "fmt.Sprint(bottomUp(1).V, bottomUp(1))", "a new pointer is stored"},
		{"field of a call's result", "", "bottomUp(1).Left = bottomUp(0)", "written through"},
		{"appended, not to a field", "type Box struct{ Kids []*Node }\n\nfunc mkBox() *Box { return &Box{} }", "b := mkBox()\n\tfmt.Println(len(append(b.Kids, bottomUp(1))))", "a new pointer is stored"},
		{"lent to a variadic", "func many(ns ...*Node) {}", "many(bottomUp(1))", "a new pointer is stored"},
		{"field of another type", "type Other struct{ Items []int }\n\nfunc mk() *Other { return &Other{} }", "_ = mk()", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := strings.Replace(tree, "func main() {\n", tc.decl+"\n\nfunc main() {\n\t"+tc.body+"\n", 1)
			p, node := ownedOf(t, src)
			if tc.why == "" {
				return // checked in TestOwnedFields
			}
			if p.Owned(node) {
				t.Fatalf("*Node owned, want shared: %s", tc.why)
			}
			_, why, ok := p.SharedReason(node)
			if !ok || !strings.Contains(why, tc.why) {
				t.Errorf("SharedReason = %q, %v; want %q", why, ok, tc.why)
			}
		})
	}
}

// TestOwnedFields checks the candidates: the element must be a struct of
// the program whose fields are numbers, strings and heap pointers, and a
// shared type shares the types its fields point to.
func TestOwnedFields(t *testing.T) {
	src := `package main

type Node struct {
	Items map[string]int
	Kid   *Kid
}

type Kid struct{ V int }

func mk() *Node { return &Node{Kid: &Kid{V: 1}} }

func main() { _ = mk().Kid.V }
`
	plain := strings.Replace(src, "Items map[string]int", "Items []int\n\tAt    struct{ X, Y int }", 1)
	plain = strings.Replace(plain, "func main() { _ = mk().Kid.V }", "func main() {\n\tn := mk()\n\tn.Items = append(n.Items, 1)\n\tn.Items[0] = 2\n\tn.At.X++\n\t_ = n.Kid.V\n}", 1)
	if p, node := ownedOf(t, plain); !p.Owned(node) {
		_, why, _ := p.SharedReason(node)
		t.Errorf("plain-data fields changed in place: Node shared (%s), want owned", why)
	}
	p, node := ownedOf(t, src)
	if _, why, _ := p.SharedReason(node); !strings.Contains(why, "its field Items is of type map[string]int") {
		t.Errorf("Node: %q", why)
	}
	kid := types.NewPointer(p.pkg.Types.Scope().Lookup("Kid").Type())
	if _, why, _ := p.SharedReason(kid); !strings.Contains(why, "a field of a shared type holds it") {
		t.Errorf("Kid: %q", why)
	}
	src = `package main

type Node struct{ V int }

func mk() *map[int]int { m := map[int]int{}; return &m }

func main() { _ = len(*mk()) }
`
	p, _ = ownedOf(t, src)
	mapPtr := types.NewPointer(types.NewMap(types.Typ[types.Int], types.Typ[types.Int]))
	if _, why, _ := p.SharedReason(mapPtr); !strings.Contains(why, "not a struct type of the program") {
		t.Errorf("*map[int]int: %q", why)
	}
}

// TestValueData checks which types copy without sharing: numbers,
// booleans, strings, and structs of them, but no slice.
func TestValueData(t *testing.T) {
	str := types.NewStruct([]*types.Var{types.NewField(0, nil, "S", types.Typ[types.String], false)}, nil)
	slice := types.NewSlice(types.Typ[types.Int])
	for _, tc := range []struct {
		t    types.Type
		want bool
	}{
		{types.Typ[types.Int], true},
		{str, true},
		{slice, false},
		{types.NewStruct([]*types.Var{types.NewField(0, nil, "S", slice, false)}, nil), false},
		{types.NewMap(types.Typ[types.Int], types.Typ[types.Int]), false},
	} {
		if got := valueData(tc.t); got != tc.want {
			t.Errorf("valueData(%s) = %v, want %v", tc.t, got, tc.want)
		}
	}
}

// TestIfaceConversions checks pointers converted to interfaces (RFC 0012
// with RFC 0017): the interface owns the object, so a new pointer or a
// variable not used after moves in, and the type stays owned; a variable
// still used, or a field, is a second owner, refused; a variable known to
// be nil has no object, so it may be converted and still used.
func TestIfaceConversions(t *testing.T) {
	const src = `package main

import "fmt"

type Node struct {
	Kid *Kid
	V   int
}

type Kid struct{ V int }

func main() {
	var a any = &Node{V: 1}
	n := &Node{Kid: &Kid{V: 1}}
	var b any = n
	var none *Node
	var c any = none
	fmt.Println(a != nil, b != nil, c != nil, none == nil)
}
`
	p, node := ownedOf(t, src)
	if !p.Owned(node) {
		_, why, _ := p.SharedReason(node)
		t.Fatalf("*Node shared: %s", why)
	}
	if errs := p.ownerErrors(); len(errs) > 0 {
		t.Errorf("errors: %v", errs)
	}
	bad := strings.Replace(src, "none == nil)", "none == nil, n.V, b == nil)\n\tvar d any = n.Kid\n\t_ = d", 1)
	p, _ = ownedOf(t, bad)
	var msgs []string
	for _, e := range p.ownerErrors() {
		msgs = append(msgs, e.msg)
	}
	got := strings.Join(msgs, "\n")
	for _, want := range []string{"n is converted to an interface while it is still used", "n.Kid is converted to an interface, so its object has two owners"} {
		if !strings.Contains(got, want) {
			t.Errorf("errors %q, want %q", got, want)
		}
	}
	// none assigned is not known to be nil.
	for _, change := range []string{"none = &Node{}"} {
		changed := strings.Replace(src, "\tvar c any = none\n", "\t"+change+"\n\tvar c any = none\n", 1)
		if p, _ := ownedOf(t, changed); len(p.ownerErrors()) == 0 {
			t.Errorf("%s: no error for none converted while still used", change)
		}
	}
}

// TestAssertRefs checks that q, ok := x.(*T) borrows the pointer the
// interface holds: a local borrow of x's variable (RFC 0017).
func TestAssertRefs(t *testing.T) {
	const src = `package main

import "fmt"

type Node struct{ V int }

func (n *Node) Inc() { n.V++ }

type Incer interface{ Inc() }

func main() {
	var x Incer = &Node{V: 1}
	if q, ok := x.(*Node); ok {
		q.Inc()
		fmt.Println(q.V)
	}
	fmt.Println(x != nil)
}
`
	p, _ := ownedOf(t, src)
	var q *types.Var
	for v := range p.refAsserts {
		q = v
	}
	if q == nil || p.RefAssert(q) == nil || p.AssertRef(p.RefAssert(q)) != q || !p.LocalRef(q) {
		t.Fatalf("q is not a borrow of x: %v", q)
	}
	if p.AssertRef(&ast.TypeAssertExpr{}) != nil {
		t.Error("AssertRef of another assertion")
	}
}

// TestAsRefs checks that var q *T; errors.As(err, &q) makes q a borrow
// of the pointer in err's chain, and that a nil pointer converted to an
// interface is marked (NilConv); &v given to another call is neither.
func TestAsRefs(t *testing.T) {
	const src = `package main

import (
	"errors"
	"fmt"
)

type Node struct{ V int }

func (n *Node) Error() string { return "node" }

func (n *Node) Inc() { n.V++ }

func main() {
	var err error = &Node{V: 1}
	var q *Node
	if errors.As(err, &q) {
		fmt.Println(q.V)
	}
	var none *Node
	var e2 error = none
	n := &Node{}
	n.Inc()
	fmt.Println(e2 != nil, none == nil, n.V)
}
`
	pkg, err := load.Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	p := Ptrs(pkg)
	byName := map[string]*types.Var{}
	for id, obj := range pkg.Info.Defs {
		if v, ok := obj.(*types.Var); ok {
			byName[id.Name] = v
		}
	}
	if q := byName["q"]; p.RefAs(q) == nil || !p.LocalRef(q) {
		t.Errorf("q is not a borrow of err: %v, %v", p.RefAs(q), p.LocalRef(q))
	}
	nils := 0
	for id := range pkg.Info.Uses {
		if p.NilConv(id) {
			nils++
			if id.Name != "none" {
				t.Errorf("NilConv(%s)", id.Name)
			}
		}
	}
	if nils != 1 {
		t.Errorf("%d nil conversions, want 1", nils)
	}
	// &r given to a function of the program is not an errors.As target.
	other := strings.Replace(src, "func (n *Node) Inc()", "func set(e error, p **Node) bool { return e == nil && p != nil }\n\nfunc (n *Node) Inc()", 1)
	other = strings.Replace(other, "\tn := &Node{}\n", "\tvar r *Node\n\t_ = set(nil, &r)\n\tn := &Node{}\n", 1)
	if pkg, err = load.Source("/q", "main.go", other, "go1.25"); err != nil {
		t.Fatal(err)
	}
	p = Ptrs(pkg)
	for id, obj := range pkg.Info.Defs {
		if v, ok := obj.(*types.Var); ok && id.Name == "r" && p.RefAs(v) != nil {
			t.Error("&r given to set is an errors.As target")
		}
	}
}

// TestWrittenThroughMap checks a write into a map a field holds,
// n.m["a"] = 1: not data the box owns, so the type is shared.
func TestWrittenThroughMap(t *testing.T) {
	const src = `package main

import "fmt"

type Node struct{ m map[string]int }

func mk() *Node { return &Node{m: map[string]int{}} }

func main() {
	n := mk()
	n.m["a"] = 1
	fmt.Println(n.m["a"])
}
`
	p, node := ownedOf(t, src)
	if _, why, ok := p.SharedReason(node); !ok || !strings.Contains(why, "its field m") {
		t.Errorf("*Node: %q, %v", why, ok)
	}
}

// TestSliceReadThrough checks that len(*p) and (*p)[i] of a
// slice of data only read the box's Vec: the slice pointer stays owned.
// (A copy of the slice out of the box, q := *p, is refused by the borrow
// checker: it would not share the elements as Go's does.)
func TestSliceReadThrough(t *testing.T) {
	const src = `package main

import "fmt"

type Ints []int

func mk() *Ints {
	ns := Ints{1, 2}
	return &ns
}

func main() {
	p := mk()
	(*p)[0] = 5
	fmt.Println(len(*p), (*p)[1], len(*mk()))
}
`
	pkg, err := load.Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	p := Ptrs(pkg)
	ints := types.NewPointer(pkg.Types.Scope().Lookup("Ints").Type())
	if !p.Owned(ints) {
		_, why, _ := p.SharedReason(ints)
		t.Errorf("*Ints shared: %s", why)
	}
}
