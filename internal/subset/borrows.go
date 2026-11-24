package subset

import (
	"go/ast"
	"go/token"
	"go/types"
)

// Temporary borrows (RFC 0017, step 5, decision 1). A local variable of a
// heap pointer type that only ever holds a pointer some other place owns,
// and only reads through it, is a borrow, as Rust's let mut p =
// head.as_deref() is: walking a list, for p := head; p != nil; p = p.Next.
// It is assigned only a variable, a field or nil, never a new pointer;
// and it is only read through (p.f, a method that reads), tested for nil,
// lent to a function that reads it, or assigned to another such borrow.
// Assigning one lends its value instead of moving it; the borrow checker
// refuses a change to the owner while the borrow is in use.

// BorrowVar reports whether local variable v is a temporary borrow.
func (p *PtrPlan) BorrowVar(v *types.Var) bool {
	p.borrowsOnce()
	return p.borrows[v]
}

// BorrowSources returns the variables temporary borrow v borrows from:
// the roots of the values it is given, through other borrows.
func (p *PtrPlan) BorrowSources(v *types.Var) map[*types.Var]bool {
	p.borrowsOnce()
	return p.sources[v]
}

// borrowsOnce finds the temporary borrows the first time they are asked
// for: the greatest set of candidates whose uses and values keep to the
// rules, by removing those that break them until none does.
func (p *PtrPlan) borrowsOnce() {
	if p.borrows != nil {
		return
	}
	b := &borrowFinder{p: p, info: p.pkg.Info, uses: map[*types.Var][]use{}, values: map[*types.Var][]ast.Expr{}}
	b.index()
	cand := map[*types.Var]bool{}
	for v, vals := range b.values {
		lends := false // a borrow of something: a variable only ever nil owns its nil
		for _, val := range vals {
			lends = lends || val != nil && !b.info.Types[val].IsNil()
		}
		if p.HeapType(v.Type()) && !b.params[v] && !p.Cells[v] && lends {
			cand[v] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for v := range cand {
			if !b.keeps(v, cand) {
				delete(cand, v)
				changed = true
			}
		}
	}
	p.borrows = cand
	p.sources = map[*types.Var]map[*types.Var]bool{}
	for v := range cand {
		src := map[*types.Var]bool{}
		for _, val := range b.values[v] {
			if r := rootVar(b.info, val); r != nil && r != v {
				src[r] = true
			}
		}
		p.sources[v] = src
	}
	for changed := true; changed; {
		changed = false
		for v, src := range p.sources {
			for r := range src {
				for q := range p.sources[r] {
					if q != v && !src[q] {
						src[q] = true
						changed = true
					}
				}
			}
		}
	}
	// A borrow assigned from another borrow shares what that one borrows
	// (an Option<&T> is copied): only the owners count.
	for _, src := range p.sources {
		for r := range src {
			if cand[r] {
				delete(src, r)
			}
		}
	}
}

// use is one use of a variable, with the node above it.
type use struct {
	id  *ast.Ident
	par ast.Node
}

// borrowFinder indexes the program's local pointer variables.
type borrowFinder struct {
	p    *PtrPlan
	info *types.Info
	// uses lists each variable's uses, and values the values it is
	// declared or assigned with (nil for var p *T).
	uses   map[*types.Var][]use
	values map[*types.Var][]ast.Expr
	params map[*types.Var]bool
	// written are the variables' uses at the root of a place written
	// (v.f = x, v.f++), and changed the parameters their function changes
	// through.
	written map[*ast.Ident]bool
	changed map[*types.Var]bool
}

func (b *borrowFinder) index() {
	b.params, b.written = map[*types.Var]bool{}, map[*ast.Ident]bool{}
	for _, obj := range b.info.Defs {
		if fn, ok := obj.(*types.Func); ok {
			sig := fn.Signature()
			for _, tup := range []*types.Tuple{sig.Params(), sig.Results()} {
				for i := 0; i < tup.Len(); i++ {
					b.params[tup.At(i)] = true
				}
			}
			if r := sig.Recv(); r != nil {
				b.params[r] = true
			}
		}
	}
	for _, f := range b.p.pkg.Files {
		var stack []ast.Node
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if id, ok := n.(*ast.Ident); ok {
				if v, ok := b.info.Uses[id].(*types.Var); ok {
					b.uses[v] = append(b.uses[v], use{id: id, par: above(stack)})
				}
			}
			switch x := n.(type) {
			case *ast.IncDecStmt:
				b.markWritten(x.X)
			case *ast.AssignStmt:
				for _, l := range x.Lhs {
					b.markWritten(l)
				}
				if x.Tok == token.DEFINE || x.Tok == token.ASSIGN {
					for i, l := range x.Lhs {
						if id, ok := ast.Unparen(l).(*ast.Ident); ok && len(x.Lhs) == len(x.Rhs) {
							if v := b.localOf(id); v != nil {
								b.values[v] = append(b.values[v], x.Rhs[i])
							}
						}
					}
				}
			case *ast.ValueSpec:
				for i, id := range x.Names {
					if v := b.localOf(id); v != nil {
						var e ast.Expr
						if len(x.Values) == len(x.Names) {
							e = x.Values[i]
						}
						b.values[v] = append(b.values[v], e)
					}
				}
			}
			stack = append(stack, n)
			return true
		})
	}
}

// markWritten marks the variable at the root of l, a place written
// through a field: l = v.f, v.f.g.
func (b *borrowFinder) markWritten(l ast.Expr) {
	sel, ok := ast.Unparen(l).(*ast.SelectorExpr)
	for ok {
		if id, isID := ast.Unparen(sel.X).(*ast.Ident); isID {
			b.written[id] = true
			return
		}
		sel, ok = ast.Unparen(sel.X).(*ast.SelectorExpr)
	}
}

// above returns the node above the top of stack, skipping parentheses.
func above(stack []ast.Node) ast.Node {
	for i := len(stack) - 1; ; i-- { // a name always has a parent: its file at least
		if _, ok := stack[i].(*ast.ParenExpr); !ok {
			return stack[i]
		}
	}
}

// localOf returns the local variable id defines or names, or nil.
func (b *borrowFinder) localOf(id *ast.Ident) *types.Var {
	obj := b.info.Defs[id]
	if obj == nil {
		obj = b.info.Uses[id]
	}
	v, ok := obj.(*types.Var)
	if !ok || v.IsField() || v.Pkg() == nil || v.Parent() == v.Pkg().Scope() {
		return nil
	}
	return v
}

// keeps reports whether v, a candidate, keeps to the rules, the other
// candidates being borrows.
func (b *borrowFinder) keeps(v *types.Var, cand map[*types.Var]bool) bool {
	for _, val := range b.values[v] {
		if !b.lendable(val, cand) {
			return false
		}
	}
	for _, u := range b.uses[v] {
		if !b.reads(v, u, cand) {
			return false
		}
	}
	return true
}

// lendable reports whether e, a value assigned to a borrow, is owned
// elsewhere: nil, a variable (another borrow, or an owner), or a field
// read through one.
func (b *borrowFinder) lendable(e ast.Expr, cand map[*types.Var]bool) bool {
	if e == nil || b.info.Types[e].IsNil() {
		return true
	}
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		v := b.localOf(x)
		return v != nil
	case *ast.SelectorExpr:
		if _, taken := b.p.Take(x); taken {
			return false // taken out of the field: the variable owns it
		}
		s := b.info.Selections[x]
		return s != nil && s.Kind() == types.FieldVal && rootVar(b.info, x) != nil
	}
	return false // a new pointer, or a call's: it needs an owner
}

// reads reports whether use u of borrow v only reads: v.f read, a method
// called, v compared with nil, v lent to a call, or v assigned to another
// borrow.
func (b *borrowFinder) reads(v *types.Var, u use, cand map[*types.Var]bool) bool {
	switch p := u.par.(type) {
	case *ast.SelectorExpr:
		if b.written[u.id] {
			return false // changed through: an exclusive borrow, not yet
		}
		s := b.info.Selections[p]
		if s.Kind() == types.MethodVal {
			fn := s.Obj().(*types.Func)
			return !b.changes(fn, fn.Signature().Recv())
		}
		return true
	case *ast.BinaryExpr:
		return (p.Op == token.EQL || p.Op == token.NEQ) && (b.info.Types[p.X].IsNil() || b.info.Types[p.Y].IsNil())
	case *ast.CallExpr:
		fn := b.callee(p)
		for i, a := range p.Args {
			if ast.Unparen(a) == ast.Expr(u.id) {
				return fn != nil && i < fn.Signature().Params().Len() && !b.changes(fn, fn.Signature().Params().At(i))
			}
		}
	case *ast.AssignStmt:
		for i, r := range p.Rhs {
			if ast.Unparen(r) == ast.Expr(u.id) && len(p.Lhs) == len(p.Rhs) {
				id, ok := ast.Unparen(p.Lhs[i]).(*ast.Ident)
				return ok && cand[b.localOf(id)]
			}
		}
		for _, l := range p.Lhs {
			if ast.Unparen(l) == ast.Expr(u.id) {
				return true // assigned: its values are checked in lendable
			}
		}
	case *ast.ValueSpec:
		for i, r := range p.Values {
			if ast.Unparen(r) == ast.Expr(u.id) {
				return cand[b.localOf(p.Names[i])]
			}
		}
	}
	return false
}

// callee returns the program function c calls statically, or nil.
func (b *borrowFinder) callee(c *ast.CallExpr) *types.Func {
	var fn *types.Func
	switch f := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		fn, _ = b.info.Uses[f].(*types.Func)
	case *ast.SelectorExpr:
		fn, _ = b.info.Uses[f.Sel].(*types.Func)
	}
	if fn == nil || fn.Pkg() != b.p.pkg.Types {
		return nil
	}
	return fn
}

// changes reports whether fn changes through its parameter or receiver
// v: a field under it written, or it lent to a callee that changes it,
// found once for every function.
func (b *borrowFinder) changes(fn *types.Func, v *types.Var) bool {
	if b.changed == nil {
		b.changed = b.changedParams()
	}
	return b.changed[v]
}

// changedParams returns the parameters and receivers that their function
// changes through, to a fixed point over the calls.
func (b *borrowFinder) changedParams() map[*types.Var]bool {
	mut := map[*types.Var]bool{}
	for id := range b.written {
		if v, ok := b.info.Uses[id].(*types.Var); ok && b.params[v] {
			mut[v] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, f := range b.p.pkg.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				fn := b.callee(c)
				if fn == nil {
					return true
				}
				sig := fn.Signature()
				var ops []ast.Expr
				var ps []*types.Var
				if sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr); ok && sig.Recv() != nil {
					ops, ps = append(ops, sel.X), append(ps, sig.Recv())
				}
				for i, a := range c.Args {
					if i < sig.Params().Len() {
						ops, ps = append(ops, a), append(ps, sig.Params().At(i))
					}
				}
				for i, op := range ops {
					if v := rootVar(b.info, op); v != nil && b.params[v] && mut[ps[i]] && !mut[v] {
						mut[v] = true
						changed = true
					}
				}
				return true
			})
		}
	}
	return mut
}
