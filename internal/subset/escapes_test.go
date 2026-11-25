package subset

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/load"
)

// TestEscapeBoxes checks which &x move a local value into a new box: x is
// a local struct value, used before through its fields only, and its one
// & hands the pointer on, outside any loop or function literal x is
// declared outside of.
func TestEscapeBoxes(t *testing.T) {
	for _, tc := range []struct {
		name, decl string
		boxes      int  // the &x that are escape boxes
		owned      bool // whether *Node stays owned
	}{
		{"returned", "func mk() *Node {\n\tn := Node{V: 1}\n\tn.Name = \"n\"\n\treturn &n\n}", 1, true},
		{"no value, then fields", "func mk(l *Node) *Node {\n\tvar n Node\n\tn.Left = l\n\treturn &n\n}", 1, true},
		{"declared with var and a value", "func mk() *Node {\n\tvar n = Node{V: 1}\n\treturn &n\n}", 1, true},
		{"a field moved into the literal", "func mk() *Node {\n\tn := Node{Left: bottomUp(1)}\n\treturn &n\n}", 1, true},
		{"assigned", "func mk() *Node {\n\tn := Node{V: 1}\n\tp := &n\n\treturn p\n}", 1, true},
		{"a literal's field", "type Bag struct{ First *Node }\n\nfunc mk() *Bag {\n\tn := Node{V: 1}\n\treturn &Bag{First: &n}\n}", 1, true},
		{"a slice literal's element", "func mk() []*Node {\n\tn := Node{V: 1}\n\treturn []*Node{&n}\n}", 1, true},
		{"appended in the loop that declares it", "func mk(vs []int) []*Node {\n\tvar ns []*Node\n\tfor _, v := range vs {\n\t\tn := Node{V: v}\n\t\tns = append(ns, &n)\n\t}\n\treturn ns\n}", 1, true},
		{"used after", "func mk() *Node {\n\tn := Node{V: 1}\n\tp := &n\n\tn.V = 2\n\treturn p\n}", 0, false},
		{"in a loop it is declared outside of", "func mk(vs []int) []*Node {\n\tvar ns []*Node\n\tn := Node{V: 1}\n\tfor range vs {\n\t\tns = append(ns, &n)\n\t}\n\treturn ns\n}", 0, false},
		{"in a function literal", "func mk() *Node {\n\tn := Node{V: 1}\n\tf := func() *Node { return &n }\n\treturn f()\n}", 0, false},
		{"taken twice", "func mk(b bool) *Node {\n\tn := Node{V: 1}\n\tif b {\n\t\treturn &n\n\t}\n\treturn &n\n}", 0, false},
		{"copied whole", "func mk() *Node {\n\tn := Node{V: 1}\n\tm := n\n\t_ = m\n\treturn &n\n}", 0, false},
		{"lent to a call", "func use(n *Node) int { return n.V }\n\nfunc mk() int {\n\tn := Node{V: 1}\n\treturn use(&n)\n}", 0, false},
		{"a field's address", "func mk() *int {\n\tn := Node{V: 1}\n\treturn &n.V\n}", 0, false},
		{"declared from another value", "func mk() *Node {\n\tn := *bottomUp(1)\n\treturn &n\n}", 0, false},
		{"one of several names", "func mk() *Node {\n\tvar n, m Node\n\t_ = m.V\n\treturn &n\n}", 1, false},
		{"one of several values", "func mk() *Node {\n\tn, m := Node{V: 1}, Node{V: 2}\n\t_ = m.V\n\treturn &n\n}", 1, false},
		{"one of a call's results, by var", "func two() (Node, Node) { return Node{}, Node{} }\n\nfunc mk() *Node {\n\tvar n, m Node = two()\n\t_ = m.V\n\treturn &n\n}", 0, false},
		{"one of a call's results", "func two() (Node, Node) { return Node{}, Node{} }\n\nfunc mk() *Node {\n\tn, m := two()\n\t_ = m.V\n\treturn &n\n}", 0, false},
		{"a parameter", "func mk(n Node) *Node {\n\treturn &n\n}", 1, true},
		{"shared for another reason", "func mk() *Node {\n\tn := Node{V: 1}\n\treturn &n\n}\n\nvar kept *Node\n\nfunc keep(a *Node) { kept = a }", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := strings.Replace(tree, "func main() {\n", tc.decl+"\n\nfunc main() {\n", 1)
			p, node := ownedOf(t, src)
			if len(p.escapes) != tc.boxes {
				t.Errorf("%d escape boxes, want %d", len(p.escapes), tc.boxes)
			}
			if p.Owned(node) != tc.owned {
				_, why, _ := p.SharedReason(node)
				t.Errorf("owned = %v, want %v (%s)", p.Owned(node), tc.owned, why)
			}
			for u, v := range p.escapes {
				got := p.EscapeBox(u)
				if tc.owned && got != v || !tc.owned && got != nil {
					t.Errorf("EscapeBox(&%s) = %v", v.Name(), got)
				}
				if tc.owned && p.Cells[v] {
					t.Errorf("%s is still a cell", v.Name())
				}
			}
		})
	}
	p, _ := ownedOf(t, tree)
	if p.EscapeBox(&ast.Ident{Name: "x"}) != nil || p.EscapeBox(&ast.UnaryExpr{Op: token.AND, X: &ast.Ident{Name: "x"}}) != nil {
		t.Error("EscapeBox of an expression that is not an escape box is not nil")
	}
}

// TestLentAddrs checks which locals stay plain when their address is
// taken: every & lent to a call of the program, as an argument or a
// pointer receiver, where no callee keeps it.
func TestLentAddrs(t *testing.T) {
	const pt = `package main

type Pt struct{ V int }

func mkPt() *Pt { return &Pt{V: 1} }

func (n *Pt) bump() { n.V++ }

func (n *Pt) self() *Pt { return n }

func (n *Pt) add(o *Pt) { n.V += o.V }

func peek(n *Pt) int { return n.V }

func keep(n *Pt) *Pt { return n }

type Hold struct{ P *Pt }

func store(n *Pt) *Hold { return &Hold{P: n} }

func many(ns ...*Pt) {}

func main() {
	_ = mkPt().V
	BODY
}
`
	for _, tc := range []struct {
		name, body  string
		lent, owned bool
	}{
		{"a pointer receiver", "var n Pt\n\tn.bump()\n\t(&n).bump()\n\t_ = n.V", true, true},
		{"an argument", "n := Pt{V: 1}\n\t_ = peek(&n) + peek((&n))\n\tn.V++", true, true},
		{"lent back by a method", "var n Pt\n\tm := n.self()\n\t_ = m.V + n.V", true, true},
		{"kept by a function", "var n Pt\n\tm := keep(&n)\n\t_ = m.V + n.V", false, false},
		{"lent to a variadic", "var n Pt\n\tmany(&n)\n\t_ = n.V", false, false},
		{"lent to a method's argument", "n := Pt{V: 1}\n\tm := Pt{V: 2}\n\tm.add(&n)\n\t_ = n.V + m.V", true, true},
		{"lent to a function literal", "var n Pt\n\tfunc(p *Pt) { p.V++ }(&n)\n\t_ = n.V", false, false},
		{"a field's address", "var n Pt\n\t_ = peek(&n)\n\tq := &n.V\n\t*q = 1", false, false},
		{"in a function literal", "var n Pt\n\tf := func() int { return peek(&n) }\n\t_ = f() + n.V", false, false},
		{"lent to a function value", "var n Pt\n\tf := peek\n\t_ = f(&n) + n.V", false, false},
		{"a local borrow", "var n Pt\n\t_ = peek(&n)\n\tq := &n\n\t_ = q.V", true, true},
		{"a local borrow lent back", "var n Pt\n\tq := &n\n\tm := keep(q)\n\t_ = m.V + n.V", true, true},
		{"a local borrow kept", "var n Pt\n\tq := &n\n\th := store(q)\n\t_ = n.V + h.P.V", false, false},
		{"a pointer computed", "var n Pt\n\t_ = peek(&n)\n\tq := &n\n\tr := q\n\t_ = r.V", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pkg, err := load.Source("/p", "main.go", strings.Replace(pt, "BODY", tc.body, 1), "go1.25")
			if err != nil {
				t.Fatal(err)
			}
			p := Ptrs(pkg)
			ptr := types.NewPointer(pkg.Types.Scope().Lookup("Pt").Type())
			if p.Owned(ptr) != tc.owned {
				_, why, _ := p.SharedReason(ptr)
				t.Fatalf("owned = %v, want %v (%s)", p.Owned(ptr), tc.owned, why)
			}
			if got := len(p.lentOK) > 0 && !p.Cells[firstKey(p.lentOK)]; got != tc.lent {
				t.Errorf("lent = %v, want %v", got, tc.lent)
			}
			for v := range p.refs {
				if p.RefCall(v) != nil {
					continue // p := f(x): a temporary borrow where only read
				}
				if p.LocalRef(v) != tc.lent || tc.lent && p.RefSource(v) == nil {
					t.Errorf("LocalRef(%s) = %v, RefSource = %v", v.Name(), p.LocalRef(v), p.RefSource(v))
				}
			}
		})
	}
	p, _ := ownedOf(t, tree)
	if p.LentAddr(&ast.BasicLit{}) || p.LentAddr(&ast.UnaryExpr{Op: token.AND, X: &ast.Ident{Name: "x"}}) {
		t.Error("LentAddr of an expression that lends nothing")
	}
}

func firstKey(m map[*types.Var]bool) *types.Var {
	for v := range m {
		return v
	}
	return nil
}

// TestEscapeValues checks escape boxes of numbers: used whole before the
// &, by any operator, and declared from any value.
func TestEscapeValues(t *testing.T) {
	const src = `package main

func neg(v int) *int {
	w := v
	w = -w
	return &w
}

func dec(v int) *int {
	var w = v + 1
	return &w
}

func main() { _ = *neg(1) + *dec(2) }
`
	pkg, err := load.Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	p := Ptrs(pkg)
	if !p.Owned(types.NewPointer(types.Typ[types.Int])) {
		_, why, _ := p.SharedReason(types.NewPointer(types.Typ[types.Int]))
		t.Fatalf("*int shared (%s), want owned", why)
	}
	if len(p.escapes) != 2 {
		t.Errorf("%d escape boxes, want 2", len(p.escapes))
	}
}
