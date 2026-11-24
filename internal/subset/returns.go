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
	p.retMut = map[*types.Func]bool{}
	for changed := true; changed; {
		// to a fixed point: return second(n) lends n back once second
		// does, and lends it exclusively only if second does
		found, mut := p.retBorrowsOf()
		changed = len(found) != len(p.retBorrow) || len(mut) != len(p.retMut)
		for fn, v := range found {
			changed = changed || p.retBorrow[fn] != v || p.retMut[fn] != mut[fn]
		}
		p.retBorrow, p.retMut = found, mut
	}
}

// retBorrowsOf finds the functions that return a borrow of an input, and
// those lending it back exclusively, the ones found so far taken as doing
// so.
func (p *PtrPlan) retBorrowsOf() (found map[*types.Func]*types.Var, exclusive map[*types.Func]bool) {
	found, exclusive = map[*types.Func]*types.Var{}, map[*types.Func]bool{}
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
				if ok, mut := p.returnsBorrowOf(fd, input); ok {
					found[fn] = input // the emitter names a lifetime where elision would not give it
					if mut {
						exclusive[fn] = true
					}
					break
				}
			}
		}
	}
	return found, exclusive
}

// returnsBorrowOf reports whether every return of fd gives nil or a place
// rooted at input, at least one of them the latter, and fd otherwise only
// reads or changes through input, or lends it to a call (ok), and whether
// it changes through it or lends it on (mut): an exclusive borrow returned.
func (p *PtrPlan) returnsBorrowOf(fd *ast.FuncDecl, input *types.Var) (ok, mut bool) {
	info := p.pkg.Info
	ok = true
	some := false
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
				read, changes := p.lendsBack(stack)
				ok = ok && read
				mut = mut || changes
			}
		}
		return ok
	})
	return ok && some, mut
}

// lendsBack reports whether the use of an input on top of stack keeps the
// input lent (ok): the place it starts (x.f, x[i], *x) is returned, given
// to a call that lends it back, compared with nil, read as a value,
// written, or lent to a call of the program (whose keeping is checked with
// the ownership); not its address taken, nor a pointer copied or kept,
// nor inside a function literal. changes reports a write through it or a
// lend to a call, which make the borrow returned exclusive.
func (p *PtrPlan) lendsBack(stack []ast.Node) (ok, changes bool) {
	info := p.pkg.Info
	if inFuncLit(stack) {
		return false, false
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
		return true, false // checked with the returns
	case *ast.AssignStmt:
		for _, l := range q.Lhs {
			if l == place {
				return place != stack[len(stack)-1], true // written through, not assigned
			}
		}
		return !pointer, false
	case *ast.IncDecStmt:
		return true, true
	case *ast.UnaryExpr:
		return q.Op != token.AND && !pointer, false
	case *ast.SelectorExpr:
		// A method called on it: lent to the program's method.
		call, isCall := stack[i-1].(*ast.CallExpr)
		return isCall && call.Fun == q && p.programFunc(q) != nil, true
	case *ast.CallExpr:
		if p.retBorrowCall(q) == place {
			return true, p.retMut[p.programFunc(q.Fun)]
		}
		if q.Fun != place && p.programFunc(q.Fun) != nil {
			return true, pointer // lent to the program's function
		}
		return !pointer && q.Fun != place, false
	case *ast.BinaryExpr:
		if q.Op == token.EQL || q.Op == token.NEQ {
			return !pointer || info.Types[q.X].IsNil() || info.Types[q.Y].IsNil(), false
		}
	}
	return !pointer, false
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

// RetMut reports whether fn changes through the input it lends back
// (retBorrows): the borrow it returns is exclusive.
func (p *PtrPlan) RetMut(fn *types.Func) bool { return p.retMut[fn] }

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
