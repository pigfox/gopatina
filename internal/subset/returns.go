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
	p.walked = map[*types.Var]bool{}
	for _, f := range p.pkg.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
				if in := p.retBorrow[p.pkg.Info.Defs[fd.Name].(*types.Func)]; in != nil {
					for v := range p.walkers(fd, in) {
						p.walked[v] = true
					}
				}
			}
		}
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

// assigned reports whether body assigns v or takes its address.
func assigned(info *types.Info, body *ast.BlockStmt, v types.Object) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, l := range x.Lhs {
				if id, ok := ast.Unparen(l).(*ast.Ident); ok && info.Uses[id] == v {
					found = true
				}
			}
		case *ast.UnaryExpr:
			if id, ok := ast.Unparen(x.X).(*ast.Ident); ok && x.Op == token.AND && info.Uses[id] == v {
				found = true
			}
		}
		return !found
	})
	return found
}

// returnsBorrowOf reports whether every return of fd gives nil or a place
// rooted at input, at least one of them the latter, and fd otherwise only
// reads or changes through input, or lends it to a call (ok), and whether
// it changes through it or lends it on (mut): an exclusive borrow returned.
func (p *PtrPlan) returnsBorrowOf(fd *ast.FuncDecl, input *types.Var) (ok, mut bool) {
	info := p.pkg.Info
	ok = true
	some := false
	walk := p.walkers(fd, input)
	var stack []ast.Node
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		switch x := n.(type) {
		case *ast.FuncLit:
			ok = false // a closure's returns are not fd's: not followed
			stack = stack[:len(stack)-1]
			return false
		case *ast.ReturnStmt:
			var r ast.Expr
			if len(x.Results) == 0 {
				// A bare return gives the named result: nil if fd never
				// assigns it, as return nil does; else as return r.
				named := fd.Type.Results.List[0].Names[0]
				if !assigned(info, fd.Body, info.Defs[named]) {
					return true
				}
				r = named
			} else {
				r = ast.Unparen(x.Results[0])
			}
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
			root := placeRoot(info, r)
			ok = ok && (root == input || walk[root])
			some = true
		case *ast.Ident:
			if info.Uses[x] == input && walkerValue(info, stack, walk) {
				return true // p := head: a walker starts at the input
			}
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

// Walkers (RFC 0017). A function may walk from its input to the place it
// returns, a borrow of the input all the way:
//
//	for p := head; p != nil; p = p.Next {
//		if p.V == v {
//			return p
//		}
//	}
//
// p is a walker: a local pointer variable of fd only ever given the input,
// a field of itself (p = p.Next) or nil, and only read through, compared
// and returned. Returning it lends the input back. The walkers of the
// functions lending back are temporary borrows (borrows.go).

// walkers returns the walkers of fd from input.
func (p *PtrPlan) walkers(fd *ast.FuncDecl, input *types.Var) map[*types.Var]bool {
	info := p.pkg.Info
	cand := map[*types.Var]bool{}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if v, ok := info.Defs[id].(*types.Var); ok && isPointer(v.Type()) && types.Identical(v.Type(), input.Type()) {
				cand[v] = true
			}
		}
		return true
	})
	var stack []ast.Node
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		switch x := n.(type) {
		case *ast.FuncLit:
			ast.Inspect(x, func(m ast.Node) bool {
				if id, ok := m.(*ast.Ident); ok {
					delete(cand, localVarOf(info, id))
				}
				return true
			})
			return false
		case *ast.AssignStmt:
			for i, l := range x.Lhs {
				v := localVarOf(info, ast.Unparen(l))
				if !cand[v] {
					continue
				}
				if len(x.Lhs) != len(x.Rhs) || (x.Tok != token.DEFINE && x.Tok != token.ASSIGN) {
					delete(cand, v)
					continue
				}
				r := ast.Unparen(x.Rhs[i])
				if !info.Types[r].IsNil() && placeRoot(info, r) != input && placeRoot(info, r) != v {
					delete(cand, v)
				}
			}
		case *ast.ValueSpec:
			for _, name := range x.Names {
				if v, _ := info.Defs[name].(*types.Var); cand[v] && len(x.Values) != 0 {
					delete(cand, v) // var p = head: only := and = are walkers' steps
				}
			}
		case *ast.Ident:
			v, _ := info.Uses[x].(*types.Var)
			if cand[v] && !walkerUse(info, stack) {
				delete(cand, v)
			}
		}
		return true
	})
	return cand
}

// localVarOf returns the variable e names, or nil.
func localVarOf(info *types.Info, e ast.Expr) *types.Var {
	id, ok := e.(*ast.Ident)
	if !ok {
		return nil
	}
	obj := info.Defs[id]
	if obj == nil {
		obj = info.Uses[id]
	}
	v, _ := obj.(*types.Var)
	return v
}

// walkerUse reports whether the use of a walker on top of stack keeps it a
// walker: a field read (p.V, p.Next as a value), a comparison, a return,
// or the place it is assigned.
func walkerUse(info *types.Info, stack []ast.Node) bool {
	child := stack[len(stack)-1]
	i := len(stack) - 2
	for ; i > 0; i-- {
		if _, ok := stack[i].(*ast.ParenExpr); !ok {
			break
		}
		child = stack[i]
	}
	switch q := stack[i].(type) {
	case *ast.SelectorExpr:
		s := info.Selections[q]
		if s == nil || s.Kind() != types.FieldVal {
			return false
		}
		for j := i - 1; ; j-- { // not written through: p.V = x, p.V++, &p.V
			switch w := stack[j].(type) {
			case *ast.SelectorExpr, *ast.IndexExpr, *ast.ParenExpr:
				continue
			case *ast.AssignStmt:
				for _, l := range w.Lhs {
					if l == stack[j+1] {
						return false
					}
				}
			case *ast.IncDecStmt:
				return false
			case *ast.UnaryExpr:
				return w.Op != token.AND
			}
			return true // the body's block at least ends the walk up
		}
	case *ast.BinaryExpr:
		return q.Op == token.EQL || q.Op == token.NEQ
	case *ast.ReturnStmt:
		return true
	case *ast.AssignStmt:
		for _, l := range q.Lhs {
			if l == child {
				return true // assigned: its values are checked
			}
		}
	}
	return false
}

// walkerValue reports whether the input on top of stack is the value a
// walker is given (p := head).
func walkerValue(info *types.Info, stack []ast.Node, walk map[*types.Var]bool) bool {
	as, ok := stack[len(stack)-2].(*ast.AssignStmt)
	if !ok || len(as.Lhs) != len(as.Rhs) {
		return false
	}
	for i, r := range as.Rhs {
		if r == stack[len(stack)-1] && walk[localVarOf(info, ast.Unparen(as.Lhs[i]))] {
			return true
		}
	}
	return false
}

// lendsBack reports whether the use of an input on top of stack keeps the
// input lent (ok): the place it starts (x.f, x[i], *x) is returned, given
// to a call that lends it back, compared with nil, read as a value,
// written, or lent to a call of the program (whose keeping is checked with
// the ownership); not its address taken, nor a pointer copied or kept
// (returnsBorrowOf gives up at a function literal before any use in one).
// changes reports a write through it or a lend to a call, which make the
// borrow returned exclusive.
func (p *PtrPlan) lendsBack(stack []ast.Node) (ok, changes bool) {
	info := p.pkg.Info
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
