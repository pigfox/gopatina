package subset

import (
	"go/ast"
	"go/types"
	"sort"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/load"
)

// nonNilOf loads src and lists what the non-nil analysis (RFC 0017,
// step 2) finds: "result f" for a function returning a Box, "f.v" for a
// local variable, parameter or receiver v of function f that is never
// nil.
func nonNilOf(t *testing.T, src string) string {
	t.Helper()
	pkg, err := load.Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	p := Ptrs(pkg)
	var out []string
	for id, obj := range pkg.Info.Defs {
		switch o := obj.(type) {
		case *types.Func:
			if p.NonNilResult(o) {
				out = append(out, "result "+o.Name())
			}
		case *types.Var:
			if p.NonNilVar(o) {
				fn := ""
				for _, f := range pkg.Files {
					for _, d := range f.Decls {
						if fd, ok := d.(*ast.FuncDecl); ok && fd.Pos() <= id.Pos() && id.End() <= fd.End() {
							fn = fd.Name.Name
						}
					}
				}
				out = append(out, fn+"."+o.Name())
			}
		}
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

func TestNonNil(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"binary trees", `package main

import "fmt"

type Node struct{ Left, Right *Node }

func bottomUp(d int) *Node {
	if d <= 0 {
		return &Node{}
	}
	n := &Node{Left: bottomUp(d - 1), Right: bottomUp(d - 1)}
	return n
}

func (n *Node) check() int {
	if n.Left == nil {
		return 1
	}
	return 1 + n.Left.check() + n.Right.check()
}

func main() {
	long := bottomUp(4)
	fmt.Println(long.check(), bottomUp(2).check())
}
`, "bottomUp.n check.n main.long result bottomUp"},
		{"rules", `package main

import "fmt"

type Node struct {
	Next *Node
	V    int
}

type Box struct{ V int }

func fresh() *Node { return &Node{V: 1} }         // a new pointer
func viaCall() *Node { return fresh() }           // a call of one
func viaNew() *Node { return new(Node) }          // new(T)
func viaLocal() *Node { n := fresh(); return n }  // a local holding one
func viaVar() *Node { var n = fresh(); return n } // var with a value
func mayNil(b bool) *Node { // returns nil: no
	if b {
		return nil
	}
	return fresh()
}
func two() (*Node, int) { return fresh(), 1 }  // two results: no
func named() (r *Node) { r = fresh(); return } // a named result: no
func number() int { return 1 }
func viaLit() *Node { // a function literal's return is its own
	f := func() *Node { return nil }
	_ = f
	return fresh()
}

func readFirst(n *Node) int           { return n.V }
func readCall(n *Node) int            { return n.Next.V + number() } // the call is first: no
func recvFirst(n *Node) int           { return n.Next.get() }        // the call's receiver reads n
func argFirst(n *Node) int            { return pass(n.V) }           // the call's argument reads n
func noRead(n *Node) int              { return pass(1) + n.V }       // a call reading nothing first: no
func indexFirst(n *Node, s []int) int { return s[n.V] }              // the index reads n first
func otherIndex(n *Node, s []int) int { return s[0] + n.V }          // an index first: no
func divFirst(n *Node, d int) int     { return d/d + n.V }           // a division first: no
func constDiv(n *Node) int            { return 6/n.V + n.V }         // reads n before dividing
func starFirst(n *Node, b *Box) int   { return (*b).V + n.V }        // a dereference of b first: b, not n
func assertFirst(n *Node, x any) int  { return x.(int) + n.V }       // an assertion first: no
func sliceFirst(n *Node, s []int) int { return len(s[1:]) + n.V }    // a slice first: no
func unary(n *Node) int               { return -n.V }
func andRight(n *Node, b bool) bool   { return b && n.V > 0 } // the right of && may not run: no
func lit(n *Node) Box                 { return Box{V: n.V} }
func litPlain(n *Node) Box            { return Box{n.V} }
func conv(n *Node) int64              { return int64(n.V) }   // a conversion is no call
func lenOf(n *Node, s []int) int      { return len(s) + n.V } // nor is len
func paren(n *Node) int               { return (n).V }
func nested(n *Node) int              { return pass(pass(n.V)) } // the inner call is first
func second(m int, n *Node) int       { return n.V + m }
func cond(n *Node) int {
	if n.V > 0 {
		return 1
	}
	return 0
}
func condInit(n *Node) int {
	if x := 1; n.V > x { // an init first: no
		return 1
	}
	return 0
}
func exprFirst(n *Node) {
	use(n.V)
}
func assign(n *Node) int {
	x := n.V
	return x
}
func assignField(n *Node, b *Box) {
	b.V = n.V // a field assigned: no
}
func loop(n *Node) int { // a loop first: no
	for i := 0; i < 1; i++ {
	}
	return n.V
}
func empty(n *Node) {} // nothing read: no
func compared(n *Node) int { // compared with nil: no
	if n == nil {
		return 0
	}
	return n.V
}
func printed(n *Node) int { // the call reads n first
	fmt.Println(n.V)
	return n.V
}
func passOn(n *Node) int            { return readFirst(n) } // passes it on: a call reading nothing first, no
func (n *Node) get() int            { return n.V }
func (n *Node) late() int           { return n.V }
func (n *Node) take(x int) int      { return n.V + x }
func unsafeArgs(n *Node, m int) int { return n.V + m }
func quietArgs(n *Node, m int, b *Box) int { return n.V + m }
func setFirst(n *Node)                 { n.V = 1 }
func incFirst(n *Node)                 { n.V++ }
func setCall(n *Node)                  { n.V = number() }
func setPanics(n *Node, s []int)       { n.V = s[0] }
func pathArgs(n *Node, m int) int { return n.V + m } // a field of a field after it: no
func use(int)                       {}
func pass(x int) int                { return x }

func main() {
	a := fresh()
	b := fresh()
	b = nil // assigned: no
	c := fresh()
	if c == nil { // compared: no
		return
	}
	var d *Node // no value: no
	x := 1
	p := &x
	fmt.Println(readFirst(a), readCall(a), recvFirst(a), argFirst(a), noRead(a), indexFirst(a, nil), *p, b.V, d.V)
	fmt.Println(otherIndex(a, nil), divFirst(a, 1), constDiv(a), starFirst(a, &Box{}), assertFirst(a, 1), sliceFirst(a, nil))
	fmt.Println(unary(a), andRight(a, true), lit(a), litPlain(a), conv(a), lenOf(a, nil), cond(a), condInit(a), paren(a))
	exprFirst(a)
	fmt.Println(assign(a), loop(a), compared(a), printed(a), passOn(a))
	assignField(a, &Box{})
	empty(a)
	fmt.Println(a.get(), (a).Next.late(), a.Next.take(number()), unsafeArgs(a.Next, 3), unsafeArgs(a.Next, number()))
	fmt.Println(viaCall().V, viaNew().V, viaLocal().V, viaVar().V, mayNil(true).V, named().V, readFirst(fresh()), readFirst(a.Next))
	bx := Box{V: 1}
	e := mayNil(false) // not new: no
	fmt.Println(e.V, viaLit().V, (pass)(1), nested(a), second(1, a.Next), unsafeArgs(a.Next, x), quietArgs(a.Next, x, nil), quietArgs(a.Next, a.V, nil), quietArgs(a.Next, bx.V, nil), pathArgs(a.Next, a.Next.V), unsafeArgs(a.Next, pass(1)))
	setFirst(a)
	incFirst(a)
	setCall(a)
	setPanics(a, nil)
	n2, _ := two()
	fmt.Println(n2.V, readFirst(nil))
	g := number
	_ = g
}
`, "argFirst.n assign.n cond.n constDiv.n conv.n exprFirst.n get.n incFirst.n indexFirst.n late.n lenOf.n lit.n litPlain.n main.a nested.n paren.n printed.n quietArgs.n recvFirst.n result fresh result viaCall result viaLit result viaLocal result viaNew result viaVar second.n setFirst.n starFirst.b unary.n viaLocal.n viaVar.n"},
	}
	cases = append(cases, struct{ name, src, want string }{"a field given a value that reads it", `package main

type Node struct {
	Items []int
	V     int
}

type Pt struct{ V int }

func (p Pt) get() int             { return p.V }
func valueFirst(p *Pt) int        { return p.get() + 1 }       // the value method copies *p first
func valueLater(p *Pt) int        { return pass(1) + p.get() } // another call first: no
func pass(x int) int              { return x }
func mkPt() *Pt                   { return &Pt{V: 1} }
func appendFirst(n *Node)           { n.Items = append(n.Items, 1) } // reads n.Items first
func appendOther(n *Node, s []int)  { n.Items = append(s, 1) }       // the call reads nothing of n first: no
func two(n *Node, s []int) (x int)  { n.V, x = len(n.Items), 1; return } // two places: no

func main() {
	a := &Node{}
	appendFirst(a)
	appendOther(a, nil)
	_ = two(a, nil)
	pt := mkPt()
	_ = valueFirst(pt) + valueLater(pt)
	_ = a.V
}
`, "appendFirst.n main.a main.pt result mkPt valueFirst.p"})
	cases = append(cases, struct{ name, src, want string }{"a number behind a pointer", "package main\n\ntype Num int\n\nfunc (n *Num) Inc() { *n++ }\n\nfunc set(n *Num, v Num) { *n = v }\n\nfunc get(n *Num) Num { return *n }\n\nfunc newNum(v Num) *Num { return &v }\n\nfunc main() {\n\tn := newNum(4)\n\tn.Inc()\n\tset(n, 1)\n\t_ = get(n)\n}\n", "Inc.n get.n main.n result newNum set.n"})
	cases = append(cases, struct{ name, src, want string }{"a pointer lent back", `package main

type Node struct{ V int }

func mk() *Node { return &Node{V: 1} }

func same(n *Node) *Node { // lent back: read through first
	_ = n.V
	return n
}

func first(a, b *Node) *Node { // two inputs: lent back from a
	_ = a.V
	return a
}

func bump(n *Node) *Node { // changed through: returned, not lent back
	n.V++
	return n
}

func main() {
	n := mk()
	_ = same(n).V + first(n, mk()).V + bump(mk()).V
}
`, "first.a main.n result mk same.n"})
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := nonNilOf(t, c.src); got != c.want {
				t.Errorf("got  %s\nwant %s", got, c.want)
			}
		})
	}
}
