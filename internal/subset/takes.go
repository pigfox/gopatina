package subset

import (
	"go/ast"
	"go/token"
	"go/types"
)

// Takes (RFC 0017, step 6). A pointer moved out of a field leaves the
// field naming nothing it owns, which one owner per object allows only
// when the very next statement fills the field again: l := t.Left;
// t.Left = nil is Rust's let l = t.left.take(), and with another value
// in place of nil, std::mem::replace. Go's meaning is kept: nothing runs
// between the two statements, so nothing sees the field in between.

// Take returns the statement that refills the field e is moved out of,
// when e is a take: a field of an owned pointer type, assigned to a local
// variable by a statement whose next statement in the same block assigns
// the same field.
func (p *PtrPlan) Take(e ast.Expr) (*ast.AssignStmt, bool) {
	p.takesOnce()
	s, ok := p.takes[ast.Unparen(e)]
	return s, ok
}

// takesOnce finds the takes the first time they are asked for.
func (p *PtrPlan) takesOnce() {
	if p.takes != nil {
		return
	}
	p.takes = map[ast.Expr]*ast.AssignStmt{}
	info := p.pkg.Info
	for _, f := range p.pkg.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			b, ok := n.(*ast.BlockStmt)
			if !ok {
				return true
			}
			for _, st := range b.List {
				// t.f = f(..., t.f, ...): the field taken out for the call,
				// which refills it with its result (RFC 0017, step 8).
				as, ok := st.(*ast.AssignStmt)
				if !ok || as.Tok != token.ASSIGN || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
					continue
				}
				c, ok := ast.Unparen(as.Rhs[0]).(*ast.CallExpr)
				if !ok {
					continue
				}
				for _, a := range c.Args {
					sel, ok := ast.Unparen(a).(*ast.SelectorExpr)
					if ok && types.ExprString(sel) == types.ExprString(ast.Unparen(as.Lhs[0])) && p.HeapType(info.TypeOf(sel)) && rootVar(info, sel) != nil {
						p.takes[sel] = as
					}
				}
			}
			for i := 0; i+1 < len(b.List); i++ {
				e := takenValue(info, b.List[i])
				next, ok := b.List[i+1].(*ast.AssignStmt)
				if e == nil || !ok || next.Tok != token.ASSIGN || len(next.Lhs) != 1 || len(next.Rhs) != 1 {
					continue
				}
				if types.ExprString(ast.Unparen(next.Lhs[0])) == types.ExprString(e) && p.HeapType(info.TypeOf(e)) {
					p.takes[e] = next
				}
			}
			return true
		})
	}
}

// takenValue returns the field statement s assigns to one local variable
// or field, v := t.f, v = t.f, var v = t.f or x.g = t.f, or nil.
func takenValue(info *types.Info, s ast.Stmt) ast.Expr {
	var e ast.Expr
	switch s := s.(type) {
	case *ast.AssignStmt:
		if (s.Tok != token.DEFINE && s.Tok != token.ASSIGN) || len(s.Lhs) != 1 || len(s.Rhs) != 1 {
			return nil
		}
		switch l := ast.Unparen(s.Lhs[0]).(type) {
		case *ast.Ident:
		case *ast.SelectorExpr:
			if rootVar(info, l) == nil {
				return nil
			}
			// n.Next = s.Top; s.Top = n: taken into a field (RFC 0017,
			// step 8), Rust's n.next = s.top.take().
		default:
			return nil
		}
		e = s.Rhs[0]
	case *ast.DeclStmt:
		g, ok := s.Decl.(*ast.GenDecl)
		if !ok || g.Tok != token.VAR || len(g.Specs) != 1 {
			return nil
		}
		vs := g.Specs[0].(*ast.ValueSpec)
		if len(vs.Names) != 1 || len(vs.Values) != 1 {
			return nil
		}
		e = vs.Values[0]
	default:
		return nil
	}
	sel, ok := ast.Unparen(e).(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	if s := info.Selections[sel]; s == nil || s.Kind() != types.FieldVal || rootVar(info, sel) == nil {
		return nil
	}
	return sel
}
