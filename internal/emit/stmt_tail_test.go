package emit

import (
	"go/ast"
	"go/token"
	"go/types"
	"testing"

	"github.com/pigfox/gopatina/internal/borrow"
	"github.com/pigfox/gopatina/internal/load"
	"github.com/pigfox/gopatina/internal/subset"
)

func TestFoldTailLetReturnBranches(t *testing.T) {
	// errs.Returns non-zero
	g := &gen{errs: &subset.ErrPlan{Returns: map[*ast.ReturnStmt]subset.ReturnKind{}}}
	r := &ast.ReturnStmt{Results: []ast.Expr{ast.NewIdent("x")}}
	g.errs.Returns[r] = subset.ReturnOk
	if _, _, ok := g.foldTailLetReturn(&ast.AssignStmt{}, r); ok {
		t.Fatal("expected no fold for lowered return")
	}

	// wrong result arity / non-ident / blank / missing use
	info := &types.Info{Uses: map[*ast.Ident]types.Object{}, Defs: map[*ast.Ident]types.Object{}, Types: map[ast.Expr]types.TypeAndValue{}}
	g = &gen{errs: &subset.ErrPlan{Returns: map[*ast.ReturnStmt]subset.ReturnKind{}, Calls: map[ast.Stmt]*subset.ErrCall{}}, info: info}
	if _, _, ok := g.foldTailLetReturn(&ast.AssignStmt{}, &ast.ReturnStmt{Results: nil}); ok {
		t.Fatal("expected no fold for no results")
	}
	if _, _, ok := g.foldTailLetReturn(&ast.AssignStmt{}, &ast.ReturnStmt{Results: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "1"}}}); ok {
		t.Fatal("expected no fold for non-ident return")
	}
	if _, _, ok := g.foldTailLetReturn(&ast.AssignStmt{}, &ast.ReturnStmt{Results: []ast.Expr{ast.NewIdent("_")}}); ok {
		t.Fatal("expected no fold for blank return")
	}
	if _, _, ok := g.foldTailLetReturn(&ast.AssignStmt{}, &ast.ReturnStmt{Results: []ast.Expr{ast.NewIdent("x")}}); ok {
		t.Fatal("expected no fold for unresolved return ident")
	}

	// wrong statement/assign form / mismatch defs
	pkg := types.NewPackage("p", "p")
	xobj := types.NewVar(token.NoPos, pkg, "x", types.Typ[types.Int])
	rx := ast.NewIdent("x")
	info.Uses[rx] = xobj
	rxRet := &ast.ReturnStmt{Results: []ast.Expr{rx}}

	if _, _, ok := g.foldTailLetReturn(&ast.EmptyStmt{}, rxRet); ok {
		t.Fatal("expected no fold for non-assign stmt")
	}
	if _, _, ok := g.foldTailLetReturn(&ast.AssignStmt{Tok: token.ASSIGN, Lhs: []ast.Expr{ast.NewIdent("x")}, Rhs: []ast.Expr{ast.NewIdent("y")}}, rxRet); ok {
		t.Fatal("expected no fold for assign token")
	}
	if _, _, ok := g.foldTailLetReturn(&ast.AssignStmt{Tok: token.DEFINE, Lhs: []ast.Expr{ast.NewIdent("x"), ast.NewIdent("y")}, Rhs: []ast.Expr{ast.NewIdent("z")}}, rxRet); ok {
		t.Fatal("expected no fold for multi-lhs")
	}
	if _, _, ok := g.foldTailLetReturn(&ast.AssignStmt{Tok: token.DEFINE, Lhs: []ast.Expr{&ast.IndexExpr{}}, Rhs: []ast.Expr{ast.NewIdent("y")}}, rxRet); ok {
		t.Fatal("expected no fold for non-ident lhs")
	}
	if _, _, ok := g.foldTailLetReturn(&ast.AssignStmt{Tok: token.DEFINE, Lhs: []ast.Expr{ast.NewIdent("_")}, Rhs: []ast.Expr{ast.NewIdent("y")}}, rxRet); ok {
		t.Fatal("expected no fold for blank lhs")
	}
	other := ast.NewIdent("x")
	info.Defs[other] = types.NewVar(token.NoPos, pkg, "x", types.Typ[types.Int])
	if _, _, ok := g.foldTailLetReturn(&ast.AssignStmt{Tok: token.DEFINE, Lhs: []ast.Expr{other}, Rhs: []ast.Expr{ast.NewIdent("y")}}, rxRet); ok {
		t.Fatal("expected no fold for mismatched defs")
	}

	// err-call branch: valid candidate but marked as an error call.
	g.pkg = &load.Package{Types: pkg}
	lx := ast.NewIdent("x")
	info.Defs[lx] = xobj
	as := &ast.AssignStmt{Tok: token.DEFINE, Lhs: []ast.Expr{lx}, Rhs: []ast.Expr{ast.NewIdent("y")}}
	g.errs.Calls[as] = &subset.ErrCall{}
	if _, _, ok := g.foldTailLetReturn(as, rxRet); ok {
		t.Fatal("expected no fold for err call")
	}
	delete(g.errs.Calls, as)

	// pointer-type branch: fold is refused for pointer-typed locals.
	ptrType := types.NewPointer(types.Typ[types.Int])
	pobj := types.NewVar(token.NoPos, pkg, "p", ptrType) // package scope: typeOf falls back to info.Types
	rp := ast.NewIdent("p")
	info.Uses[rp] = pobj
	lp := ast.NewIdent("p")
	info.Defs[lp] = pobj
	info.Types[lp] = types.TypeAndValue{Type: ptrType}
	if _, _, ok := g.foldTailLetReturn(&ast.AssignStmt{Tok: token.DEFINE, Lhs: []ast.Expr{lp}, Rhs: []ast.Expr{ast.NewIdent("y")}}, &ast.ReturnStmt{Results: []ast.Expr{rp}}); ok {
		t.Fatal("expected no fold for pointer-typed var")
	}
}

func TestHasLineCommentsBetween(t *testing.T) {
	g := &gen{usedComments: map[*ast.CommentGroup]bool{}}
	cg := &ast.CommentGroup{List: []*ast.Comment{{Slash: 10, Text: "// hi"}}}
	g.comments = []*ast.CommentGroup{cg}
	if !g.hasLineCommentsBetween(1, 20) {
		t.Fatal("expected comment between")
	}
	g.usedComments[cg] = true
	if g.hasLineCommentsBetween(1, 20) {
		t.Fatal("expected used comment ignored")
	}
	g.usedComments[cg] = false
	if g.hasLineCommentsBetween(11, 20) {
		t.Fatal("expected out-of-range start ignored")
	}
	if g.hasLineCommentsBetween(1, 9) {
		t.Fatal("expected out-of-range end ignored")
	}
}

func TestFoldTailLetReturnEmissionBranches(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "decl then return not folded",
			src: `package main
func main() { println(f()) }
func f() int { var x = 1; return x }
func println(v int) {}
`,
		},
		{
			name: "multi results no fold",
			src: `package main
func main() { a, b := f(); println(a); println(b) }
func f() (int, int) { x := 1; return x, 2 }
func println(v int) {}
`,
		},
		{
			name: "return different ident",
			src: `package main
func main() { println(f()) }
func f() int { x := 1; y := 2; _ = x; return y }
func println(v int) {}
`,
		},
		{
			name: "trailing comment on folded return",
			src: `package main
func main() { println(f()) }
func f() int {
	x := 1
	return x // tail
}
func println(v int) {}
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pkg, err := load.Source("/app", "main.go", tc.src, "go1.25")
			if err != nil {
				t.Fatal(err)
			}
			if ds := subset.Check(pkg); len(ds) > 0 {
				t.Fatalf("subset: %v", ds)
			}
			res, ds := borrow.Check(pkg)
			if len(ds) > 0 {
				t.Fatalf("borrow: %v", ds)
			}
			_ = Emit(pkg, res, Options{Target: goldenTarget, Version: "devel"})
		})
	}
}
