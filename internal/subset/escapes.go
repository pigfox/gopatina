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
// a local variable or value parameter of a struct type whose pointer type
// is a heap type, declared with a composite literal or no value; every
// other use of x is a field selection (read or written), none of them
// after the &; the & is the only one of x and hands the pointer on (a
// result, an assignment, a declaration, a literal's element or an
// append); and it is not inside a loop or function literal that x is
// declared outside of, where it would run again with x already moved.
//
// It also finds the variables whose every address is lent to a call of
// the program: &x as an argument, or x.M() and (&x).M() for a method with
// a pointer receiver. Each stays a plain local, lent as &x or &mut x, if
// no callee keeps the parameter it is lent to (lentVar).
//
// Whether the pointer type is owned is decided later, so EscapeBox and
// LentAddr ask.
func (p *PtrPlan) escapeBoxes() {
	p.escapes = map[*ast.UnaryExpr]*types.Var{}
	p.escapeInits = map[*ast.CompositeLit]bool{}
	p.lends = map[*types.Var][]*types.Var{}
	p.lentOK = map[*types.Var]bool{}
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
			bad := map[*types.Var]bool{}     // not an escape box
			lendBad := map[*types.Var]bool{} // an address not lent
			lends := map[*types.Var][]*types.Var{}
			for _, fl := range fd.Type.Params.List {
				for _, name := range fl.Names {
					if v, ok := info.Defs[name].(*types.Var); ok && p.escapable(v) {
						declared[v] = true // a value parameter: the caller's copy
					}
				}
			}
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
					bad[v], lendBad[v] = true, true
					return true
				}
				switch q := stack[len(stack)-2].(type) {
				case *ast.UnaryExpr:
					if q.Op != token.AND {
						break // -x, ^x, !x of a number or a boolean: a read
					}
					if par := p.lentTo(stack[:len(stack)-1]); par != nil && p.refs[par] != nil && plainData(v.Type()) {
						lends[v] = append(lends[v], par) // p := &x, a local borrow
						bad[v] = true
						return true
					}
					if handsOn(stack[:len(stack)-1]) && !loopedOutside(stack, v) {
						addrs[v] = append(addrs[v], q)
						lendBad[v] = true
						return true
					}
					if par := p.lentTo(stack[:len(stack)-1]); par != nil {
						lends[v] = append(lends[v], par)
						bad[v] = true
						return true
					}
					lendBad[v] = true
				case *ast.SelectorExpr:
					s := info.Selections[q]
					if u, ok := stack[len(stack)-3].(*ast.UnaryExpr); !(ok && u.Op == token.AND) && s != nil && s.Kind() == types.FieldVal {
						fieldUses = append(fieldUses, struct {
							v   *types.Var
							pos token.Pos
						}{v, id.Pos()})
						return true
					}
					if par := p.lentTo(stack); par != nil {
						lends[v] = append(lends[v], par) // x.M(), M with a pointer receiver
						bad[v] = true
						return true
					}
					if s == nil || s.Kind() != types.MethodVal {
						lendBad[v] = true // &x.f
					}
				}
				if valueData(v.Type()) {
					// A value read or written whole before its &: a plain
					// local until then.
					fieldUses = append(fieldUses, struct {
						v   *types.Var
						pos token.Pos
					}{v, id.Pos()})
					return true
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
			for v, ps := range lends {
				if !lendBad[v] && declared[v] {
					p.lends[v] = ps
				}
			}
		}
	}
}

// lentTo returns the parameter that the address on top of stack is lent
// to: &x as an argument of a function of the program, or x (or &x) as the
// receiver of a method of the program with a pointer receiver. It is nil
// for anything else.
func (p *PtrPlan) lentTo(stack []ast.Node) *types.Var {
	info := p.pkg.Info
	child := stack[len(stack)-1]
	i := len(stack) - 2
	for ; i > 0; i-- {
		if _, ok := stack[i].(*ast.ParenExpr); !ok {
			break
		}
		child = stack[i]
	}
	switch q := stack[i].(type) {
	case *ast.AssignStmt:
		// p := &x: lent to p, a local borrow if p keeps to localRef.
		if q.Tok == token.DEFINE && len(q.Lhs) == 1 && len(q.Rhs) == 1 && q.Rhs[0] == child {
			if id, ok := q.Lhs[0].(*ast.Ident); ok {
				v, _ := info.Defs[id].(*types.Var)
				return v
			}
		}
	case *ast.CallExpr:
		fn := p.programFunc(q.Fun)
		if fn == nil || fn.Signature().Variadic() {
			return nil
		}
		if sel, ok := ast.Unparen(q.Fun).(*ast.SelectorExpr); ok {
			if s := info.Selections[sel]; s != nil && s.Kind() == types.MethodExpr {
				return nil // (*T).m(&x): a method expression, which the subset refuses
			}
		}
		for j, a := range q.Args {
			if a == child {
				return fn.Signature().Params().At(j)
			}
		}
	case *ast.SelectorExpr:
		s := info.Selections[q]
		call, ok := stack[i-1].(*ast.CallExpr)
		if s == nil || s.Kind() != types.MethodVal || !ok || call.Fun != q {
			return nil // a method value keeps its receiver
		}
		fn := s.Obj().(*types.Func)
		if r := fn.Signature().Recv(); fn.Pkg() == p.pkg.Types && isPointer(r.Type()) {
			return r
		}
	}
	return nil
}

// localRefs finds the local borrows: a pointer variable declared p := &x
// (x a local) and never assigned again, whose every use reads or writes
// through it (*p, p.f), calls a method with a pointer receiver on it, or
// lends it to a function of the program, outside any function literal.
// Rust holds it as &x or &mut x. The parameters it is lent to are kept in
// refParams; ownership checks that no callee keeps them.
func (p *PtrPlan) localRefs() {
	p.refs = map[*types.Var][]*types.Var{}
	p.refSources = map[*types.Var]*types.Var{}
	p.refCalls = map[*types.Var]*ast.CallExpr{}
	info := p.pkg.Info
	cand := map[*types.Var]bool{}
	bad := map[*types.Var]bool{}
	for _, f := range p.pkg.Files {
		var stack []ast.Node
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			if v, ok := info.Defs[id].(*types.Var); ok && isPointer(v.Type()) && !v.IsField() {
				if as, ok := stack[len(stack)-2].(*ast.AssignStmt); ok && as.Tok == token.DEFINE && len(as.Lhs) == 1 && len(as.Rhs) == 1 {
					if c, ok := ast.Unparen(as.Rhs[0]).(*ast.CallExpr); ok && p.retBorrowCall(c) != nil {
						// p := f(x), f lending x back: a borrow of x's root.
						if xv := placeRoot(info, p.retBorrowCall(c)); xv != nil && !xv.IsField() && xv.Parent() != xv.Pkg().Scope() {
							cand[v] = true
							p.refSources[v] = xv
							p.refCalls[v] = c
							return true
						}
					}
					if u, ok := ast.Unparen(as.Rhs[0]).(*ast.UnaryExpr); ok && u.Op == token.AND {
						if x, ok := ast.Unparen(u.X).(*ast.Ident); ok {
							if xv, ok := info.Uses[x].(*types.Var); ok && !xv.IsField() && xv.Parent() != xv.Pkg().Scope() {
								cand[v] = true
								p.refSources[v] = xv
								return true
							}
						}
					}
				}
				return true
			}
			v, ok := info.Uses[id].(*types.Var)
			if !ok || !isPointer(v.Type()) {
				return true
			}
			if inFuncLit(stack) {
				bad[v] = true
				return true
			}
			i := len(stack) - 2
			for ; i > 0; i-- {
				if _, ok := stack[i].(*ast.ParenExpr); !ok {
					break
				}
			}
			switch q := stack[i].(type) {
			case *ast.StarExpr:
				if u, ok := stack[i-1].(*ast.UnaryExpr); ok && u.Op == token.AND {
					bad[v] = true // &*p
				}
				return true
			case *ast.SelectorExpr:
				s := info.Selections[q]
				if s != nil && s.Kind() == types.FieldVal {
					if u, ok := stack[i-1].(*ast.UnaryExpr); ok && u.Op == token.AND {
						bad[v] = true // &p.f
					}
					return true
				}
			}
			if par := p.lentTo(stack); par != nil {
				if _, isAssign := stack[i].(*ast.AssignStmt); !isAssign {
					p.refs[v] = append(p.refs[v], par)
					return true
				}
			}
			if b, ok := stack[i].(*ast.BinaryExpr); ok && p.refCalls[v] != nil && (b.Op == token.EQL || b.Op == token.NEQ) && (info.Types[b.X].IsNil() || info.Types[b.Y].IsNil()) {
				return true // p := f(x) compared with nil: a borrow that may be none
			}
			bad[v] = true // assigned, compared, returned, stored or copied
			return true
		})
	}
	for v := range cand {
		if bad[v] {
			delete(p.refs, v)
			continue
		}
		if p.refs[v] == nil {
			p.refs[v] = []*types.Var{}
		}
	}
	for v := range p.refs {
		if !cand[v] {
			delete(p.refs, v)
		}
	}
}

// LocalRef reports whether v is a local borrow (localRefs) whose pointer
// type is owned and whose source is lent, not a cell: Rust holds it as &x
// or &mut x.
func (p *PtrPlan) LocalRef(v *types.Var) bool {
	_, ok := p.refs[v]
	src := p.refSources[v]
	if !ok || !p.Owned(v.Type()) {
		return false
	}
	if p.lentOK[src] {
		return true // p := &x
	}
	// p := f(x), x an owned pointer: a borrow of it, unless only read,
	// where a shared returned borrow is a temporary borrow (step 5).
	return src != nil && p.Owned(src.Type()) && !p.Cells[src] && !p.BorrowVar(v)
}

// RefCall returns the call that local borrow v is declared from, p :=
// f(x) with f lending x back (returns.go), or nil for p := &x.
func (p *PtrPlan) RefCall(v *types.Var) *ast.CallExpr { return p.refCalls[v] }

// RefSource returns the local that local borrow v borrows.
func (p *PtrPlan) RefSource(v *types.Var) *types.Var { return p.refSources[v] }

// programFunc returns the function of the program that fun names, or nil.
func (p *PtrPlan) programFunc(fun ast.Expr) *types.Func {
	var id *ast.Ident
	switch f := ast.Unparen(fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	default:
		return nil
	}
	fn, ok := p.pkg.Info.Uses[id].(*types.Func)
	if !ok || fn.Pkg() != p.pkg.Types {
		return nil
	}
	return fn
}

// LentAddr reports whether e, &x or the receiver x of a method with a
// pointer receiver, lends a plain local (see escapeBoxes) that is not a
// cell: Rust lends &x or &mut x.
func (p *PtrPlan) LentAddr(e ast.Expr) bool {
	e = ast.Unparen(e)
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = ast.Unparen(u.X)
	}
	id, ok := e.(*ast.Ident)
	if !ok {
		return false
	}
	v, _ := p.pkg.Info.Uses[id].(*types.Var)
	return v != nil && p.lentOK[v] && p.Owned(types.NewPointer(v.Type()))
}

// escapable reports whether v is a local variable of a struct type whose
// pointer type is a heap type.
func (p *PtrPlan) escapable(v *types.Var) bool {
	if v.IsField() || v.Parent() == v.Pkg().Scope() {
		return false
	}
	_, st := v.Type().Underlying().(*types.Struct)
	return (st || valueData(v.Type())) && p.HeapType(types.NewPointer(v.Type()))
}

// declInit returns the composite literal declaring id (nil for none),
// with ok false when id is declared another way: from another value, or
// as a parameter, a range variable or one of several names.
func (p *PtrPlan) declInit(id *ast.Ident, stack []ast.Node) (*ast.CompositeLit, bool) {
	if v, _ := p.pkg.Info.Defs[id].(*types.Var); valueData(v.Type()) {
		// A number, a string or a struct of them, declared from any one
		// value: a copy of it shares nothing.
		switch d := stack[len(stack)-2].(type) {
		case *ast.AssignStmt:
			if len(d.Lhs) == 1 && len(d.Rhs) == 1 {
				lit, _ := ast.Unparen(d.Rhs[0]).(*ast.CompositeLit)
				return lit, true
			}
		case *ast.ValueSpec:
			if len(d.Names) == 1 && len(d.Values) <= 1 {
				var lit *ast.CompositeLit
				if len(d.Values) == 1 {
					lit, _ = ast.Unparen(d.Values[0]).(*ast.CompositeLit)
				}
				return lit, true
			}
		}
		return nil, false
	}
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
