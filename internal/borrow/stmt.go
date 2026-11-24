package borrow

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/pigfox/gopatina/internal/subset"
)

// block walks every statement in b, in order, for the flat summaries.
func (w *walker) block(b *ast.BlockStmt) {
	for _, s := range b.List {
		w.stmt(s)
	}
}

// stmt walks s and everything nested in it.
func (w *walker) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.BlockStmt:
		w.block(s)
	case *ast.IfStmt:
		w.simple(s.Init)
		w.read(s.Cond)
		w.block(s.Body)
		if s.Else != nil {
			w.stmt(s.Else)
		}
	case *ast.ForStmt:
		w.simple(s.Init)
		if s.Cond != nil {
			w.read(s.Cond)
		}
		w.block(s.Body)
		w.simple(s.Post)
	case *ast.RangeStmt:
		w.rangeHead(s)
		w.block(s.Body)
	case *ast.SwitchStmt:
		w.simple(s.Init)
		if s.Tag != nil {
			w.read(s.Tag)
		}
		for _, c := range s.Body.List {
			c := c.(*ast.CaseClause)
			for _, e := range c.List {
				w.read(e)
			}
			for _, st := range c.Body {
				w.stmt(st)
			}
		}
	case *ast.TypeSwitchStmt:
		w.read(typeSwitchX(s))
		for _, c := range s.Body.List {
			c := c.(*ast.CaseClause)
			if v, ok := w.info().Implicits[c].(*types.Var); ok {
				w.add(event{kind: evDef, v: v, pos: c.Pos(), init: true})
			}
			for _, st := range c.Body {
				w.stmt(st)
			}
		}
	default:
		w.simple(s)
	}
}

// typeSwitchX is the expression a type switch switches on (RFC 0012).
func typeSwitchX(s *ast.TypeSwitchStmt) ast.Expr {
	if a, ok := s.Assign.(*ast.AssignStmt); ok {
		return a.Rhs[0].(*ast.TypeAssertExpr).X
	}
	return s.Assign.(*ast.ExprStmt).X.(*ast.TypeAssertExpr).X
}

// rangeHead walks the range expression and defines the loop variables.
func (w *walker) rangeHead(s *ast.RangeStmt) {
	w.read(s.X)
	for _, e := range []ast.Expr{s.Key, s.Value} {
		if id, ok := e.(*ast.Ident); ok {
			if v := w.localVar(id); v != nil {
				w.add(event{kind: evDef, v: v, pos: id.Pos(), init: true})
			}
		}
	}
}

// simple walks a statement with no nested statements. s may be nil.
func (w *walker) simple(s ast.Stmt) {
	w.stmtCall = nil
	switch s := s.(type) {
	case *ast.ExprStmt:
		w.stmtCall = ast.Unparen(s.X)
	case *ast.AssignStmt:
		if len(s.Rhs) == 1 {
			w.stmtCall = ast.Unparen(s.Rhs[0])
		}
	case *ast.ReturnStmt:
		if len(s.Results) == 1 {
			w.stmtCall = ast.Unparen(s.Results[0])
		}
	}
	switch s := s.(type) {
	case *ast.ExprStmt:
		w.read(s.X)
	case *ast.IncDecStmt:
		w.target(s.X, true)
	case *ast.AssignStmt:
		w.assign(s)
	case *ast.DeclStmt:
		w.decl(s.Decl.(*ast.GenDecl))
	case *ast.ReturnStmt:
		for _, r := range s.Results {
			w.consume(r, "a return", nil)
		}
	}
}

func (w *walker) decl(d *ast.GenDecl) {
	if d.Tok != token.VAR {
		return
	}
	for _, spec := range d.Specs {
		vs := spec.(*ast.ValueSpec)
		for i, name := range vs.Names {
			if i < len(vs.Values) && len(vs.Values) == len(vs.Names) {
				w.define(name, vs.Values[i])
			}
		}
		if len(vs.Values) == 1 && len(vs.Names) > 1 {
			w.read(vs.Values[0])
		}
		for _, name := range vs.Names {
			if v := w.localVar(name); v != nil {
				w.add(event{kind: evDef, v: v, pos: name.Pos(), init: true})
			}
		}
	}
}

// define walks the value assigned to a new or existing variable name.
func (w *walker) define(name *ast.Ident, value ast.Expr) {
	v := w.localVar(name)
	if u, ok := ast.Unparen(value).(*ast.UnaryExpr); ok && u.Op == token.AND && v != nil && subset.Ptrs(w.a.pkg).EscapeBox(u) == nil {
		if _, isPtr := v.Type().(*types.Pointer); isPtr && KindOf(v.Type()) == PointerKind {
			ptr := w.a.pointers[v]
			if ptr == nil {
				ptr = &Pointer{pos: u.Pos()}
				if ev, ok := w.place(ast.Unparen(u.X)); ok {
					ptr.Target = ev.v
					ptr.path = ev.path
				}
				w.a.pointers[v] = ptr
			}
			w.addressOf(u, v, false)
			return
		}
	}
	if plan := subset.Ptrs(w.a.pkg); v != nil && plan.Owned(v.Type()) && plan.BorrowVar(v) {
		// A temporary borrow (RFC 0017, step 5) reads the pointer it is
		// given; its owner keeps it.
		w.read(value)
		return
	}
	w.consume(value, name.Name, nil)
}

func (w *walker) assign(s *ast.AssignStmt) {
	if s.Tok != token.DEFINE && s.Tok != token.ASSIGN {
		// x op= y
		w.subExprs(s.Lhs[0])
		w.read(s.Rhs[0])
		w.target(s.Lhs[0], false)
		return
	}
	// The operands of index expressions on the left are evaluated first.
	for _, l := range s.Lhs {
		w.subExprs(l)
	}
	if len(s.Lhs) == len(s.Rhs) {
		for i, r := range s.Rhs {
			if w.isSelfAppend(s.Lhs[i], r) {
				w.builtin(r.(*ast.CallExpr), "append")
				continue
			}
			if id, ok := s.Lhs[i].(*ast.Ident); ok {
				w.define(id, r)
				continue
			}
			w.consume(r, exprString(s.Lhs[i]), nil)
		}
	} else {
		w.read(s.Rhs[0])
	}
	for i, l := range s.Lhs {
		if len(s.Lhs) == len(s.Rhs) && w.isSelfAppend(l, s.Rhs[i]) {
			if ev, ok := w.place(l); ok {
				ev.kind = evMut
				ev.grow = true
				w.add(ev)
			}
			continue
		}
		if id, ok := l.(*ast.Ident); ok {
			if v := w.localVar(id); v != nil {
				init := s.Tok == token.DEFINE && w.info().Defs[id] != nil
				w.add(event{kind: evDef, v: v, pos: id.Pos(), whole: true, path: id.Name, init: init})
			}
			continue
		}
		w.target(l, false)
	}
}

// isSelfAppend reports whether r is append(l, ...).
func (w *walker) isSelfAppend(l, r ast.Expr) bool {
	call, ok := ast.Unparen(r).(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	if _, ok := w.info().Uses[id].(*types.Builtin); !ok || id.Name != "append" {
		return false
	}
	return sameExpr(l, call.Args[0])
}

// sameExpr reports whether two expressions are the same simple place.
func sameExpr(a, b ast.Expr) bool {
	return types.ExprString(ast.Unparen(a)) == types.ExprString(ast.Unparen(b))
}

// target records a change to the place l (an assignment target or the
// operand of ++ and --). subs reports whether to walk its index operands.
func (w *walker) target(l ast.Expr, subs bool) {
	if subs {
		w.subExprs(l)
	}
	if ev, ok := w.place(l); ok {
		ev.kind = evMut
		ev.replacesSlice = !ev.elem && isSliceType(w.info().TypeOf(l))
		w.add(ev)
	}
}
