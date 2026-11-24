package subset

import (
	"go/ast"
	"go/token"
	"go/types"
)

// Returned borrows (RFC 0017, decision 4). A function that returns one of
// its pointer inputs, or a place through it, lends it back: Rust's
//
//	fn second(n: Option<&Node>) -> Option<&Node> { n.unwrap().next.as_deref() }
//	fn self_(&mut self) -> &mut Counter { self }
//
// with the result's lifetime the input's, which Rust's elision gives where
// the input is the receiver or the only borrowed input. The caller's
// result is a borrow of its argument, not a new owner.

// retBorrows finds the functions whose one result is a pointer that every
// return gives as nil, an input of pointer type, or a place through it
// (fields, elements, *p), the same input every time, never assigned or
// changed through in the function: the input whose borrow is returned.
func (p *PtrPlan) retBorrows() {
	p.retBorrow = map[*types.Func]*types.Var{}
	for changed := true; changed; {
		// to a fixed point: return second(n) lends n back once second does
		changed = false
		for fn, v := range p.retBorrowsOf() {
			if p.retBorrow[fn] == nil {
				p.retBorrow[fn] = v
				changed = true
			}
		}
	}
}

// retBorrowsOf finds the functions that return a borrow of an input, the
// ones found so far taken as doing so.
func (p *PtrPlan) retBorrowsOf() map[*types.Func]*types.Var {
	found := map[*types.Func]*types.Var{}
	info := p.pkg.Info
	for _, f := range p.pkg.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			fn, _ := info.Defs[fd.Name].(*types.Func)
			sig := fn.Signature()
			if sig.Results().Len() != 1 || !isPointer(sig.Results().At(0).Type()) {
				continue
			}
			var inputs []*types.Var // the receiver first
			if r := sig.Recv(); r != nil && isPointer(r.Type()) {
				inputs = append(inputs, r)
			}
			for i := 0; i < sig.Params().Len(); i++ {
				if v := sig.Params().At(i); isPointer(v.Type()) {
					inputs = append(inputs, v)
				}
			}
			for _, input := range inputs {
				if p.returnsBorrowOf(fd, input) {
					found[fn] = input // the emitter names a lifetime where elision would not give it
					break
				}
			}
		}
	}
	return found
}

// returnsBorrowOf reports whether every return of fd gives nil or a place
// rooted at input, at least one of them the latter, and fd only reads
// through input otherwise: input lends nothing else, so the result is a
// shared borrow of it.
func (p *PtrPlan) returnsBorrowOf(fd *ast.FuncDecl, input *types.Var) bool {
	info := p.pkg.Info
	ok, some := true, false
	var stack []ast.Node
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		switch x := n.(type) {
		case *ast.ReturnStmt:
			r := ast.Unparen(x.Results[0])
			if info.Types[r].IsNil() {
				return true
			}
			for {
				c, isCall := r.(*ast.CallExpr)
				if !isCall || p.retBorrowCall(c) == nil {
					break
				}
				r = ast.Unparen(p.retBorrowCall(c)) // return second(n): n lent back
			}
			ok = ok && placeRoot(info, r) == input
			some = true
		case *ast.Ident:
			if info.Uses[x] == input {
				ok = ok && p.readOnly(stack)
			}
		}
		return ok
	})
	return ok && some
}

// readOnly reports whether the use of an input on top of stack only reads
// through it, or lends it back: the place it starts (x.f, x[i], *x) is
// returned, given to a call that lends it back, compared with nil, or a
// value read; not written, its address taken, a method called on it, or a
// pointer kept, nor inside a function literal.
func (p *PtrPlan) readOnly(stack []ast.Node) bool {
	info := p.pkg.Info
	if inFuncLit(stack) {
		return false
	}
	i := len(stack) - 1
	place := stack[i].(ast.Expr)
	for i--; i >= 0; i-- {
		switch q := stack[i].(type) {
		case *ast.SelectorExpr:
			if q.X == place && info.Selections[q] != nil && info.Selections[q].Kind() == types.FieldVal {
				place = q
				continue
			}
		case *ast.IndexExpr:
			if q.X == place {
				place = q
				continue
			}
		case *ast.StarExpr, *ast.ParenExpr:
			place = q.(ast.Expr)
			continue
		}
		break
	}
	pointer := isPointer(info.TypeOf(place))
	switch q := stack[i].(type) {
	case *ast.ReturnStmt:
		return true // checked with the returns
	case *ast.AssignStmt:
		for _, l := range q.Lhs {
			if l == place {
				return false // written
			}
		}
		return !pointer
	case *ast.IncDecStmt:
		return false
	case *ast.UnaryExpr:
		return q.Op != token.AND && !pointer
	case *ast.SelectorExpr:
		return false // a method called on it
	case *ast.CallExpr:
		return p.retBorrowCall(q) == place || !pointer && q.Fun != place
	case *ast.BinaryExpr:
		if q.Op == token.EQL || q.Op == token.NEQ {
			return !pointer || info.Types[q.X].IsNil() || info.Types[q.Y].IsNil()
		}
	}
	return !pointer
}

// placeRoot returns the variable that place e starts from, through field
// selections, elements and *p, or nil.
func placeRoot(info *types.Info, e ast.Expr) *types.Var {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.Ident:
			v, _ := info.Uses[x].(*types.Var)
			return v
		case *ast.SelectorExpr: // a field: a pointer-typed place
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		default:
			return nil
		}
	}
}

// PlaceRoot is placeRoot, for the borrow checker.
func PlaceRoot(info *types.Info, e ast.Expr) *types.Var { return placeRoot(info, e) }

// RetBorrow returns the input whose borrow fn returns (retBorrows), when
// the result's pointer type is owned, or nil.
func (p *PtrPlan) RetBorrow(fn *types.Func) *types.Var {
	v := p.retBorrow[fn]
	if v == nil || !p.Owned(fn.Signature().Results().At(0).Type()) {
		return nil
	}
	return v
}

// retBorrowCall returns the argument of call c that c's result borrows
// (RetBorrow), or nil: the receiver of a method, or the argument of the
// borrowed input.
func (p *PtrPlan) retBorrowCall(c *ast.CallExpr) ast.Expr {
	info := p.pkg.Info
	fn := p.programFunc(c.Fun)
	if fn == nil {
		return nil
	}
	v := p.retBorrow[fn]
	if v == nil {
		return nil
	}
	if v == fn.Signature().Recv() {
		if sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr); ok && info.Selections[sel] != nil && info.Selections[sel].Kind() == types.MethodVal {
			return sel.X
		}
		return nil // a method expression: T.m(x)
	}
	i := 0
	for fn.Signature().Params().At(i) != v {
		i++ // the input is one of the parameters
	}
	return c.Args[i]
}
