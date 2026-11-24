package subset

import (
	"go/ast"
	"go/token"
	"strings"
	"testing"
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
		{"one of several names", "func mk() *Node {\n\tvar n, m Node\n\t_ = m.V\n\treturn &n\n}", 0, false},
		{"a parameter", "func mk(n Node) *Node {\n\treturn &n\n}", 0, false},
		{"shared for another reason", "func mk() *Node {\n\tn := Node{V: 1}\n\treturn &n\n}\n\nfunc same(a, b *Node) bool { return a == b }", 1, false},
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
