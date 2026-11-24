package subset

import (
	"go/ast"
	"go/token"
	"go/types"
)

// Escape boxes (RFC 0017). Go takes the address of a local struct value
// and hands it on:
//
//	n := Node{V: v}
//	n.Name = "n"
//	return &n
//
// When nothing uses n after the &, the pointer is the only name the value
// has from there, so Rust moves n into a new box at the &: Box::new(n).
// n itself stays a plain local, not a cell.

// escapeBoxes finds the &x of the program that move x into a new box: x is
// a local variable of a struct type whose pointer type is a heap type,
// declared in the function with a composite literal or no value; every
// other use of x is a field selection (read or written), none of them
// after the &; the & is the only one of x and hands the pointer on (a
// result, an assignment, a declaration, a literal's element or an
// append); and it is not inside a loop or function literal that x is
// declared outside of, where it would run again with x already moved.
// Whether the pointer type is owned is decided later, so EscapeBox asks.
func (p *PtrPlan) escapeBoxes() {
	p.escapes = map[*ast.UnaryExpr]*types.Var{}
	p.escapeInits = map[*ast.CompositeLit]bool{}
	info := p.pkg.Info
	for _, f := range p.pkg.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			addrs := map[*types.Var][]*ast.UnaryExpr{}
			inits := map[*types.Var]*ast.CompositeLit{}
			declared := map[*types.Var]bool{}
			bad := map[*types.Var]bool{}
			var fieldUses []struct {
				v   *types.Var
				pos token.Pos
			}
			var stack []ast.Node
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if n == nil {
					stack = stack[:len(stack)-1]
					return true
				}
				stack = append(stack, n)
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				if v, ok := info.Defs[id].(*types.Var); ok && p.escapable(v) {
					lit, ok := p.declInit(id, stack)
					if !ok {
						bad[v] = true
					}
					declared[v] = true
					if lit != nil {
						inits[v] = lit
					}
					return true
				}
				v, ok := info.Uses[id].(*types.Var)
				if !ok || !p.escapable(v) {
					return true
				}
				if inFuncLit(stack) {
					bad[v] = true
					return true
				}
				switch q := stack[len(stack)-2].(type) {
				case *ast.UnaryExpr:
					if q.Op == token.AND && handsOn(stack[:len(stack)-1]) && !loopedOutside(stack, v) {
						addrs[v] = append(addrs[v], q)
						return true
					}
				case *ast.SelectorExpr:
					s := info.Selections[q]
					if u, ok := stack[len(stack)-3].(*ast.UnaryExpr); !(ok && u.Op == token.AND) && s != nil && s.Kind() == types.FieldVal {
						fieldUses = append(fieldUses, struct {
							v   *types.Var
							pos token.Pos
						}{v, id.Pos()})
						return true
					}
				}
				bad[v] = true // a whole-value use, a method, &x.f, or & kept back
				return true
			})
			for _, u := range fieldUses {
				if as := addrs[u.v]; len(as) == 1 && u.pos > as[0].Pos() {
					bad[u.v] = true // used after its value moved into the box
				}
			}
			for v, as := range addrs {
				if bad[v] || !declared[v] || len(as) != 1 {
					continue
				}
				p.escapes[as[0]] = v
				if lit := inits[v]; lit != nil {
					p.escapeInits[lit] = true
				}
			}
		}
	}
}

// escapable reports whether v is a local variable of a struct type whose
// pointer type is a heap type.
func (p *PtrPlan) escapable(v *types.Var) bool {
	if v.IsField() || v.Parent() == v.Pkg().Scope() {
		return false
	}
	_, st := v.Type().Underlying().(*types.Struct)
	return st && p.HeapType(types.NewPointer(v.Type()))
}

// declInit returns the composite literal declaring id (nil for none),
// with ok false when id is declared another way: from another value, or
// as a parameter, a range variable or one of several names.
func (p *PtrPlan) declInit(id *ast.Ident, stack []ast.Node) (*ast.CompositeLit, bool) {
	switch d := stack[len(stack)-2].(type) {
	case *ast.AssignStmt:
		if len(d.Lhs) == 1 && len(d.Rhs) == 1 {
			lit, ok := ast.Unparen(d.Rhs[0]).(*ast.CompositeLit)
			return lit, ok
		}
	case *ast.ValueSpec:
		if len(d.Names) == 1 && len(d.Values) == 0 {
			return nil, true
		}
		if len(d.Names) == 1 && len(d.Values) == 1 {
			lit, ok := ast.Unparen(d.Values[0]).(*ast.CompositeLit)
			return lit, ok
		}
	}
	return nil, false
}

// handsOn reports whether the &x on top of stack hands the pointer on: it
// is a result, a value assigned or declared, an element of a composite
// literal, or appended.
func handsOn(stack []ast.Node) bool {
	child := stack[len(stack)-1]
	switch q := stack[len(stack)-2].(type) {
	case *ast.ReturnStmt, *ast.ValueSpec:
		return true
	case *ast.AssignStmt:
		return q.Tok == token.DEFINE || q.Tok == token.ASSIGN
	case *ast.KeyValueExpr:
		return q.Value == child
	case *ast.CompositeLit:
		return true
	case *ast.CallExpr:
		id, ok := ast.Unparen(q.Fun).(*ast.Ident)
		return ok && id.Name == "append" && q.Args[0] != child
	}
	return false
}

// loopedOutside reports whether the use on top of stack is inside a loop
// that v is declared outside of: the & would run again with v moved.
func loopedOutside(stack []ast.Node, v *types.Var) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		switch l := stack[i].(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			if v.Pos() < l.Pos() || v.Pos() >= l.End() {
				return true
			}
		}
	}
	return false
}

// inFuncLit reports whether the top of stack is inside a function literal.
func inFuncLit(stack []ast.Node) bool {
	for _, n := range stack {
		if _, ok := n.(*ast.FuncLit); ok {
			return true
		}
	}
	return false
}

// EscapeBox returns the variable that &x, e, moves into a new box (see
// escapeBoxes), or nil. It is nil unless the pointer type is owned: a
// shared one keeps x in a cell.
func (p *PtrPlan) EscapeBox(e ast.Expr) *types.Var {
	u, ok := ast.Unparen(e).(*ast.UnaryExpr)
	if !ok {
		return nil
	}
	v := p.escapes[u]
	if v == nil || !p.Owned(types.NewPointer(v.Type())) {
		return nil
	}
	return v
}
