package borrow

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/pigfox/gopatina/internal/diag"
)

// Calls before reads. gc makes the calls of an expression before it reads
// the variables (the Go spec leaves that order unspecified; GoPatina
// matches gc, its reference), where Rust evaluates left to right. A read
// of a variable, or of a place under one, before a call that changes it
// would see the old value in Rust. The emitter makes such calls first,
// into locals; a loop condition, evaluated again each time round, and a
// switch tag cannot be written that way yet.

// ChangedBeforeRead reports whether n reads a local variable, or a place
// under one, before (in Rust's left-to-right order) a call of the program
// that changes it: one lending it as &mut, as a receiver or parameter in
// mode RefMut (&x, a slice it changes, an owned pointer, x.M()). A read in
// a call's receiver or arguments is made with that call, in Go too, so it
// does not count.
func ChangedBeforeRead(info *types.Info, funcs map[*types.Func]*Func, n ast.Node) bool {
	local := func(id *ast.Ident) *types.Var {
		v, _ := info.Uses[id].(*types.Var)
		if v == nil || v.IsField() || v.Pkg() == nil || v.Parent() == v.Pkg().Scope() {
			return nil
		}
		return v
	}
	root := func(e ast.Expr) *types.Var {
		for {
			switch x := ast.Unparen(e).(type) {
			case *ast.Ident:
				return local(x)
			case *ast.SelectorExpr: // a field
				e = x.X
			case *ast.IndexExpr:
				e = x.X
			case *ast.StarExpr:
				e = x.X
			case *ast.UnaryExpr: // &x
				e = x.X
			default:
				return nil
			}
		}
	}
	var calls []*ast.CallExpr
	ast.Inspect(n, func(m ast.Node) bool {
		if c, ok := m.(*ast.CallExpr); ok && funcs[calleeFunc(info, c)] != nil {
			calls = append(calls, c)
		}
		return true
	})
	inCall := func(m ast.Node) bool {
		for _, c := range calls {
			if c.Pos() <= m.Pos() && m.End() <= c.End() {
				return true
			}
		}
		return false
	}
	reads := map[*types.Var][]token.Pos{}
	ast.Inspect(n, func(m ast.Node) bool {
		if id, ok := m.(*ast.Ident); ok && !inCall(id) {
			if v := local(id); v != nil {
				reads[v] = append(reads[v], id.End())
			}
		}
		return true
	})
	for _, c := range calls {
		fn := funcs[calleeFunc(info, c)]
		lent := map[*types.Var]bool{}
		if sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr); ok && fn.Recv != nil && fn.Recv.Mode == RefMut {
			lent[root(sel.X)] = true
		}
		for i, a := range c.Args {
			if i < len(fn.Params) && fn.Params[i].Mode == RefMut {
				lent[root(a)] = true
			}
		}
		for v := range lent {
			for _, end := range reads[v] {
				if v != nil && end <= c.Pos() {
					return true
				}
			}
		}
	}
	return false
}

// checkOrder reports a loop condition, and a switch case, that reads a
// variable before a call that changes it (ChangedBeforeRead): neither has
// a place before it, run once, to make the call in. A switch tag does:
// the emitter makes its calls before the switch.
func (a *analyzer) checkOrder(decls []*ast.FuncDecl) {
	refuse := func(e ast.Expr, what, fix string) {
		d := diag.Errorf(a.pkg.Fset.Position(e.Pos()), "%s that reads a variable before a call that changes it is not supported in patina mode yet", what)
		a.report(e.Pos(), d.With("Go (gc) makes the call first; "+fix, "see docs/compat.md for what patina mode supports"))
	}
	for _, fd := range decls {
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.ForStmt:
				if n.Cond != nil && ChangedBeforeRead(a.pkg.Info, a.funcs, n.Cond) {
					refuse(n.Cond, "a loop condition", "compute the call into a variable before the loop and at the end of its body, or read the variable after the call")
				}
			case *ast.CaseClause:
				for _, e := range n.List {
					if ChangedBeforeRead(a.pkg.Info, a.funcs, e) {
						refuse(e, "a switch case", "compute the call into a variable before the switch, or read the variable after the call")
					}
				}
			}
			return true
		})
	}
}
