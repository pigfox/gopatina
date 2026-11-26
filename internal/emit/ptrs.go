package emit

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/pigfox/gopatina/internal/borrow"
	"github.com/pigfox/gopatina/internal/subset"
)

// Pointers that escape (RFC 0013) have one owner (RFC 0017): a heap
// pointer is an Option<Box<T>> (a Box<T> where never nil, the value itself
// for an unboxed one), lent as &T or &mut T. A pointer type with more than
// one owner is not supported (subset).

// heapPtr reports whether t is a heap pointer type.
func (g *gen) heapPtr(t types.Type) bool {
	return t != nil && g.ptrs != nil && g.ptrs.HeapType(t)
}

// ownPtr reports whether t is a heap pointer type that is never shared
// (RFC 0016): a box with one owner, read through as a plain borrow.
func (g *gen) ownPtr(t types.Type) bool {
	return g.heapPtr(t) && g.ptrs.Owned(t)
}

// ptrType renders the Rust type of heap pointer type t: Option<Box<T>>
// (RFC 0016), as Rust writes a pointer that may be nil.
func (g *gen) ptrType(t types.Type) string {
	return "Option<" + g.boxOf(t) + ">"
}

// ptrNil renders nil of heap pointer type t.
func (g *gen) ptrNil(t types.Type) node {
	return text("None")
}

// ptrNew renders a new heap pointer of type t holding v: Go's &T{...}.
// An owned one is a Box<T>, never nil (RFC 0017, step 2); value wraps it
// in Some where an Option<Box<T>> is wanted.
func (g *gen) ptrNew(t types.Type, v node) node {
	if g.unboxed(t) {
		return v // the value itself (unboxed)
	}
	return &call{head: "Box::new", open: "(", close: ")", args: []node{v}}
}

// boxContents renders *p for owned pointer p (plain data, RFC 0017): the
// value in its box, as a place. An Option is unwrapped, Go's nil
// dereference; a Box or a borrow is dereferenced.
func (g *gen) boxContents(p ast.Expr) node {
	unboxed := g.unboxed(g.typeOf(p)) && (g.ptrRep(p) == repOptBox || g.ptrRep(p) == repBox)
	if r := g.ptrRep(p); r == repOptRef || r == repOptRefMut && !g.usedAfter(p) {
		// An Option<&T>, Copy, or an Option<&mut T> at its last use: the
		// borrow itself, unwrapped (clippy's needless_option_as_deref).
		return atom{"*" + (&call{recv: g.rawPtr(p), head: ".unwrap", open: "(", close: ")"}).flat(), precUnary}
	}
	if g.ptrRep(p).optional() {
		m := ".as_deref"
		if g.writing {
			m = ".as_deref_mut"
		}
		if unboxed {
			m = map[bool]string{false: ".as_ref", true: ".as_mut"}[g.writing]
		}
		n := &call{recv: &call{recv: g.rawPtr(p), head: m, open: "(", close: ")"}, head: ".unwrap", open: "(", close: ")"}
		return atom{"*" + n.flat(), precUnary}
	}
	if unboxed && g.ptrRep(p) == repBox {
		return g.rawPtr(p) // the value itself
	}
	return atom{"*" + paren(g.rawPtr(p), precUnary), precUnary}
}

// unboxed reports whether t is an owned pointer to a number, a string or
// a slice of data, which Rust holds as the value itself, T or Option<T>,
// not in a Box: one owner, and nothing can reach itself through it
// (clippy's box_collection).
func (g *gen) unboxed(t types.Type) bool {
	_, st := types.Unalias(t).(*types.Pointer).Elem().Underlying().(*types.Struct)
	return !st
}

// boxOf renders what owned pointer type t holds when never nil: Box<T>,
// or T itself for an unboxed one.
func (g *gen) boxOf(t types.Type) string {
	elem := g.rustType(types.Unalias(t).(*types.Pointer).Elem())
	if g.unboxed(t) {
		return elem
	}
	return "Box<" + elem + ">"
}

// localRefRep returns how local borrow v (subset.PtrPlan.LocalRef) is held:
// &mut T where its function changes through it, &T otherwise.
func (g *gen) localRefRep(v *types.Var) ptrRep {
	if c := g.ptrs.RefCall(v); c != nil {
		return g.ptrRep(c) // p := f(x): held as f returns it
	}
	if g.ptrs.RefAssert(v) != nil || g.ptrs.RefAs(v) != nil {
		// q := x.(*T), or errors.As(err, &q): the pointer the interface
		// owns, which may be nil.
		if g.changesThrough(g.fn.Decl.Body, &ast.Ident{Name: v.Name()}) {
			return repOptRefMut
		}
		return repOptRef
	}
	if g.changesThrough(g.fn.Decl.Body, &ast.Ident{Name: v.Name()}) {
		return repRefMut
	}
	return repRef
}

// namedLifetime reports whether fn, returning a borrow of an input
// (subset.PtrPlan.RetBorrow), needs the lifetime named: Rust's elision
// ties a method's result to its receiver, and a function's to its only
// borrowed input, so a method lending back a parameter, or a function with
// more than one borrowed input, writes fn f<'a>(.. &'a T ..) -> &'a T.
func (g *gen) namedLifetime(fn *types.Func, params []string) bool {
	in := g.ptrs.RetBorrow(fn)
	if in == nil {
		return false
	}
	if r := fn.Signature().Recv(); r != nil && g.recv == r {
		return in != r // &self: elision gives the receiver's
	}
	refs := 0
	for _, p := range params {
		if strings.Contains(p, "&") {
			refs++
		}
	}
	return refs > 1
}

// lentVar returns the local that e, &x or x, lends (LentAddr).
func (g *gen) lentVar(e ast.Expr) *types.Var {
	e = ast.Unparen(e)
	if u, ok := e.(*ast.UnaryExpr); ok {
		e = ast.Unparen(u.X)
	}
	return g.localVar(e.(*ast.Ident))
}

// ptrRep is how an owned pointer (RFC 0016) is held in Rust: in an
// Option where it may be nil, owned or borrowed, and a borrow shared or
// exclusive.
type ptrRep int

const (
	repOptBox    ptrRep = iota // Option<Box<T>>: a field, or a variable that may be nil
	repBox                     // Box<T>: a new pointer, never nil (RFC 0017, step 2)
	repOptRef                  // Option<&T>: a parameter that may be nil
	repRef                     // &T: a parameter or receiver never nil
	repOptRefMut               // Option<&mut T>: one changed through (RFC 0017, step 3)
	repRefMut                  // &mut T
)

// mut reports whether r is an exclusive borrow, optional whether it is
// in an Option.
func (r ptrRep) mut() bool      { return r >= repOptRefMut }
func (r ptrRep) optional() bool { return r == repOptBox || r == repOptRef || r == repOptRefMut }

// ptrRep returns how owned pointer e is held.
func (g *gen) ptrRep(e ast.Expr) ptrRep {
	if b, ok := g.bound[types.ExprString(ast.Unparen(e))]; ok {
		return b.rep // bound by an if let Some
	}
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		v := g.localVar(x) // a variable: nil is never asked, a global never owned
		if g.ptrs.LocalRef(v) {
			return g.localRefRep(v) // p := &x: &x or &mut x
		}
		if g.res.Param(v) != nil {
			return g.paramRep(v) // lent: an owned pointer is never kept
		}
		if g.isBorrow(v) {
			if g.ptrs.RangeElem(v) {
				return repOptBox // &Option<Box<T>>, an element: read through as a field (step 7)
			}
			return repOptRef // a temporary borrow (RFC 0017, step 5)
		}
		if g.ptrs.NonNilVar(v) {
			return repBox
		}
	case *ast.CallExpr:
		if fn := g.programFunc(x); fn != nil && g.ptrs.RetBorrow(fn.Obj) != nil {
			return g.retRep(fn.Obj) // a borrow of its argument (decision 4)
		}
		if fn := g.programFunc(x); fn != nil && g.ptrs.NonNilResult(fn.Obj) {
			return repBox
		}
	case *ast.UnaryExpr:
		return repBox // &T{...}, the only owned pointer taken
	}
	return repOptBox
}

// paramRep returns how parameter v, of an owned pointer type, is held.
// retRep returns how fn's result is held when fn returns a borrow of an
// input (subset.PtrPlan.RetBorrow), which fn does not change through: the
// input's own form where every return gives the input itself, and
// otherwise an Option of a shared borrow (RFC 0017, decision 4).
func (g *gen) retRep(fn *types.Func) ptrRep {
	in := g.ptrs.RetBorrow(fn)
	r := g.paramRep(in)
	itself := true
	ast.Inspect(g.res.Funcs[fn].Decl.Body, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok {
			id, isID := ast.Unparen(ret.Results[0]).(*ast.Ident)
			itself = itself && isID && g.info.Uses[id] == in
		}
		_, lit := n.(*ast.FuncLit)
		return !lit
	})
	if itself {
		return r
	}
	if r.mut() {
		return repOptRefMut // lent exclusively: changed through, here or by a caller
	}
	return repOptRef
}

// borrowType renders the Rust type of a borrow of pointer type t held as
// r: &T, &mut T, Option<&T> or Option<&mut T>, with lifetime life ("" or
// "'a ").
func (g *gen) borrowType(t types.Type, r ptrRep, life string) string {
	inner := "&" + life + map[bool]string{false: "", true: "mut "}[r.mut()] + g.rustType(types.Unalias(t).(*types.Pointer).Elem())
	if r.optional() {
		return "Option<" + inner + ">"
	}
	return inner
}

func (g *gen) paramRep(v *types.Var) ptrRep {
	if p := g.res.Param(v); p != nil && p.Mode == borrow.Owned {
		return repOptBox // kept: moved in (RFC 0017, step 8)
	}
	r := repOptRef
	if g.ptrs.NonNilVar(v) {
		r = repRef
	}
	if p := g.res.Param(v); p != nil && p.Mode == borrow.RefMut {
		r += repOptRefMut - repOptRef
	}
	return r
}

// rawPtr renders owned pointer e as it is held (ptrRep), without the Some
// that value adds to a Box.
func (g *gen) rawPtr(e ast.Expr) node {
	e = ast.Unparen(e)
	if b, ok := g.bound[types.ExprString(e)]; ok {
		return text(b.name)
	}
	if g.isPlace(e) {
		return g.place(e)
	}
	g.raw[e] = true
	n := g.value(e)
	delete(g.raw, e)
	return n
}

// ptrAs renders owned pointer e held as want: a borrow lent to a
// parameter, or the value read or changed through.
func (g *gen) ptrAs(e ast.Expr, want ptrRep) node {
	if g.info.Types[e].IsNil() {
		return text("None") // nil lent to a parameter that may be nil
	}
	if ta, ok := ast.Unparen(e).(*ast.TypeAssertExpr); ok && g.ptrs.AssertRef(ta) != nil {
		return g.value(e) // x.(*T): already the borrow its helper lends
	}
	if g.ptrs.LentAddr(e) {
		// &x of a plain local lent to the call (subset.PtrPlan.LentAddr).
		ref := "&"
		if want.mut() {
			ref = "&mut "
		}
		n := node(text(ref + g.varName(g.lentVar(e))))
		if want.optional() {
			n = &call{head: "Some", open: "(", close: ")", args: []node{n}}
		}
		return n
	}
	have := g.ptrRep(e)
	if have == want && !have.mut() {
		return g.rawPtr(e) // the same Copy borrow, or the same Box
	}
	// The base of an exclusive borrow is itself reached exclusively.
	saved := g.writing
	g.writing = want.mut()
	n := g.rawPtr(e)
	g.writing = saved
	unboxed := g.unboxed(g.typeOf(e)) && (have == repOptBox || have == repBox)
	method := func(m string) node {
		if unboxed {
			// Option<T> itself, not Option<Box<T>>: as_ref and as_mut, since
			// as_deref would deref the String or Vec inside.
			switch m {
			case ".as_deref":
				m = ".as_ref"
			case ".as_deref_mut":
				m = ".as_mut"
			}
		}
		return &call{recv: n, head: m, open: "(", close: ")"}
	}
	some := func(prefix string) node {
		if unboxed {
			prefix = strings.TrimSuffix(prefix, "*") // &x, not &*x: the value itself
		}
		return &call{head: "Some", open: "(", close: ")", args: []node{atom{prefix + paren(n, precUnary), precUnary}}}
	}
	unwrap := func(m node) node { return &call{recv: m, head: ".unwrap", open: "(", close: ")"} }
	switch want {
	case repOptRef:
		switch have {
		case repOptBox, repOptRefMut:
			return method(".as_deref")
		case repBox, repRefMut:
			return some("&*")
		}
		return &call{head: "Some", open: "(", close: ")", args: []node{n}} // a &T passed on
	case repRef:
		switch have {
		case repOptBox:
			return unwrap(method(".as_ref")) // unwrapping None is Go's nil dereference
		case repOptRef:
			return method(".unwrap")
		case repOptRefMut:
			return unwrap(method(".as_deref"))
		case repRefMut:
			return n // &mut T coerces to &T
		}
		return &addrOf{"&", n} // a Box lent as &T: deref coercion
	case repOptRefMut:
		switch {
		case have == repOptRefMut && !g.usedAfter(e):
			return n // its last use: moved, not reborrowed (clippy)
		case have == repOptBox || have == repOptRefMut:
			return method(".as_deref_mut")
		}
		return some("&mut *") // a Box or a &mut T, reborrowed
	}
	switch have { // repRefMut
	case repOptBox:
		return unwrap(method(".as_mut")) // &mut Box<T> coerces to &mut T
	case repOptRefMut:
		if !g.usedAfter(e) {
			return method(".unwrap") // its last use: moved (clippy)
		}
		return unwrap(method(".as_deref_mut"))
	case repRefMut:
		return n // reborrowed at the call
	}
	return &addrOf{"&mut ", n} // a Box lent as &mut T
}

// usedAfter reports whether e, a variable, may be used again after this
// use: named later in its function, or used inside a loop, which may come
// round to it again.
func (g *gen) usedAfter(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident) // a parameter, a borrow, or a call's temporary
	return ok && (g.usedAfterPos(id, id.Pos()) || g.usedInHoisting(id))
}

// usedInHoisting reports whether id is in a call computed first out of
// the statement being hoisted (hoistCalls), which names id's variable
// elsewhere too: that use comes after the call, wherever it is written.
func (g *gen) usedInHoisting(id *ast.Ident) bool {
	if g.hoisting == nil {
		return false
	}
	v, again := g.localVar(id), false
	ast.Inspect(g.hoisting, func(n ast.Node) bool {
		if x, ok := n.(*ast.Ident); ok && x != id && g.localVar(x) == v {
			again = true
		}
		return !again
	})
	return again
}

// usedAfterPos reports whether id's variable may be used again after pos:
// named later in its function, or id is inside a loop, which may come
// round to it again.
func (g *gen) usedAfterPos(id *ast.Ident, pos token.Pos) bool {
	v := g.localVar(id)
	again := false
	var loops []ast.Node
	ast.Inspect(g.fn.Decl.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			// The values are evaluated before the places are written, so
			// a place naming v comes after a use of v among the values.
			for _, r := range x.Rhs {
				if r.Pos() <= id.Pos() && id.End() <= r.End() {
					for _, l := range x.Lhs {
						ast.Inspect(l, func(m ast.Node) bool {
							if y, ok := m.(*ast.Ident); ok && g.localVar(y) == v {
								again = true
							}
							return !again
						})
					}
				}
			}
		case *ast.ForStmt, *ast.RangeStmt:
			if x.Pos() <= id.Pos() && id.End() <= x.End() {
				loops = append(loops, x)
			}
		case *ast.Ident:
			if x.Pos() > pos && g.localVar(x) == v && !g.inValuesOf(x, id) {
				again = true
			}
		}
		return true
	})
	return again || len(loops) > 0
}

// inValuesOf reports whether x is among the values of the assignment
// whose places hold id: evaluated before id's place is written.
func (g *gen) inValuesOf(x, id *ast.Ident) bool {
	found := false
	ast.Inspect(g.fn.Decl.Body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || found {
			return !found
		}
		inLhs := false
		for _, l := range as.Lhs {
			inLhs = inLhs || l.Pos() <= id.Pos() && id.End() <= l.End()
		}
		for _, r := range as.Rhs {
			found = found || inLhs && r.Pos() <= x.Pos() && x.End() <= r.End()
		}
		return !found
	})
	return found
}

// ptrThrough renders owned pointer e as the base of a field read or
// write, or of a method call: whatever derefs to T, exclusively when the
// place is being written (g.writing).
func (g *gen) ptrThrough(e ast.Expr) node {
	if g.ptrs.LentAddr(e) {
		return text(g.varName(g.lentVar(e))) // x itself, which Rust borrows for the call
	}
	switch r := g.ptrRep(e); {
	case !r.optional():
		return g.rawPtr(e) // a Box, a &T or a &mut T
	case r == repOptRef:
		return g.ptrAs(e, repRef)
	case g.writing:
		saved := g.writing
		n := g.rawPtr(e)
		g.writing = saved
		return &call{recv: &call{recv: n, head: ".as_mut", open: "(", close: ")"}, head: ".unwrap", open: "(", close: ")"}
	}
	return &call{recv: &call{recv: g.rawPtr(e), head: ".as_ref", open: "(", close: ")"}, head: ".unwrap", open: "(", close: ")"}
}

// heapOrder computes the program's calls in s first when s reads through a
// heap pointer: Go makes every call of an expression before it reads the
// variables (measured with gc), and a call can change what a pointer
// points at.
func (g *gen) heapOrder(s ast.Stmt) {
	switch s.(type) {
	case *ast.ExprStmt, *ast.AssignStmt, *ast.ReturnStmt, *ast.IncDecStmt, *ast.DeclStmt:
	default:
		return // a condition is checked for this by the subset checker
	}
	calls := subset.HoistableCalls(g.pkg, s)
	if len(calls) == 0 || !g.ownedBeforeCall(s, calls) && !g.changedBeforeRead(s) {
		return
	}
	roots := 0
	switch s := s.(type) {
	case *ast.AssignStmt:
		roots = len(s.Rhs)
	case *ast.ReturnStmt:
		roots = len(s.Results)
	}
	g.hoistCalls(s, roots > 1)
}

// changedBeforeRead reports whether statement s reads a variable, or a
// place under one, before a call that changes it (borrow.ChangedBeforeRead):
// gc makes the call first, so Rust makes it first too.
func (g *gen) changedBeforeRead(s ast.Stmt) bool {
	return borrow.ChangedBeforeRead(g.info, g.res.Funcs, s)
}

// ownedBeforeCall reports whether statement s reads through an owned
// pointer (RFC 0016) before one of calls, the program's calls in s, in
// Rust's order (left to right) but after it in Go's: gc makes an
// expression's calls before it reads the variables, so a nil pointer read
// there panics only after the call's effects. A read inside a call's
// arguments comes before the calls after that call, as in Go. A read
// through a variable p comes before a call that reads through p itself in
// both, since that call panics on the same nil before it runs.
func (g *gen) ownedBeforeCall(s ast.Stmt, calls []*ast.CallExpr) bool {
	var reads []*ast.SelectorExpr
	var all []*ast.CallExpr // every call, of the program or not
	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		ast.Inspect(n, func(x ast.Node) bool {
			switch x := x.(type) {
			case *ast.BinaryExpr:
				if x.Op == token.LAND || x.Op == token.LOR {
					walk(x.X) // the right side runs after, in both
					return false
				}
			case *ast.CallExpr:
				all = append(all, x)
			case *ast.SelectorExpr:
				if sel := g.info.Selections[x]; sel != nil && sel.Kind() == types.FieldVal && g.ownPtr(g.typeOf(x.X)) {
					if r := g.ptrRep(x.X); r == repOptBox || r == repOptRef {
						reads = append(reads, x) // one never nil cannot panic
					}
				}
			}
			return true
		})
	}
	walk(s)
	within := func(n, outer ast.Node) bool { return outer.Pos() <= n.Pos() && n.End() <= outer.End() }
	for _, r := range reads {
		var d *ast.CallExpr // the innermost call r is an argument of
		for _, c := range all {
			if within(r, c) && r != c.Fun && (d == nil || within(c, d)) {
				d = c
			}
		}
		for _, c := range calls {
			if c.Pos() < r.Pos() || within(r, c) || within(c, r) || d != nil && !within(c, d) {
				continue // in Go too, r is read before c is made, or after
			}
			if g.readsThrough(c, ast.Unparen(r.X)) {
				continue
			}
			return true
		}
	}
	return false
}

// readsThrough reports whether call c reads a field through p, a local
// variable, in its receiver or arguments: it panics there for a nil p,
// before it runs.
func (g *gen) readsThrough(c *ast.CallExpr, p ast.Expr) bool {
	id, ok := p.(*ast.Ident)
	if !ok || g.localVar(id) == nil {
		return false
	}
	found := false
	ast.Inspect(c, func(x ast.Node) bool {
		if sel, ok := x.(*ast.SelectorExpr); ok && !found {
			if b, ok := ast.Unparen(sel.X).(*ast.Ident); ok && g.localVar(b) == g.localVar(id) {
				if k := g.info.Selections[sel]; k != nil && k.Kind() == types.FieldVal {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

// heapAddr renders &x for a heap pointer: a new box for &T{...}, and for
// a local variable whose address escapes, the box it moves into (the only
// other operand the checker allows a heap pointer to be taken of).
func (g *gen) heapAddr(e *ast.UnaryExpr) node {
	if x, ok := ast.Unparen(e.X).(*ast.CompositeLit); ok {
		return g.ptrNew(g.typeOf(e), g.composite(x))
	}
	return g.ptrNew(g.typeOf(e), text(g.varName(g.ptrs.EscapeBox(e)))) // x moves into its box
}

// heapCompare renders p == q, p != q, p == nil and p != nil for heap
// pointers.
func (g *gen) heapCompare(e *ast.BinaryExpr) node {
	x, y := e.X, e.Y
	if g.info.Types[x].IsNil() {
		x, y = y, x
	}
	if g.info.Types[y].IsNil() {
		test := ".is_none"
		if e.Op == token.NEQ {
			test = ".is_some"
		}
		// Only an Option is tested: a pointer never nil is never compared
		// with nil (RFC 0017).
		return &call{recv: g.rawPtr(x), head: test, open: "(", close: ")"}
	}
	return g.ptrEq(e.Op, x, y)
}

// ptrEq renders x == y (or !=) for two owned pointers, by address, as
// Rust compares references: std::ptr::eq(a, b) for two never nil; for one
// that may be nil, o.is_some_and(|o| std::ptr::eq(o, n)); for two,
// a.map(std::ptr::from_ref) == b.map(std::ptr::from_ref) (RFC 0017).
func (g *gen) ptrEq(op token.Token, x, y ast.Expr) node {
	not := func(n node) node {
		if op == token.NEQ {
			return atom{"!" + paren(n, precUnary), precUnary}
		}
		return n
	}
	ox, oy := g.ptrRep(x).optional(), g.ptrRep(y).optional()
	ref := func(e ast.Expr) node {
		if g.ptrRep(e) == repBox {
			if g.unboxed(g.typeOf(e)) {
				// a value pointer is the value itself: its place
				return atom{"&" + paren(g.rawPtr(e), precUnary), precUnary}
			}
			// &*b: the Node a Box owns, not the Box's own place
			return atom{"&*" + paren(g.rawPtr(e), precUnary), precUnary}
		}
		return g.ptrAs(e, repRef)
	}
	switch {
	case !ox && !oy:
		return not(&call{head: "std::ptr::eq", open: "(", close: ")", args: []node{ref(x), ref(y)}})
	case ox && oy:
		addr := func(e ast.Expr) node {
			return &call{recv: g.ptrAs(e, repOptRef), head: ".map", open: "(", close: ")", args: []node{text("std::ptr::from_ref")}}
		}
		return &binary{op: binOps[op].op, l: addr(x), r: addr(y), p: precCmp, nonAssoc: true}
	}
	if !ox {
		x, y = y, x // x may be nil, y never
	}
	name := g.lnames.name("", func(string) string { return "p" })
	eq := &call{head: "std::ptr::eq", open: "(", close: ")", args: []node{text(name), ref(y)}}
	return not(&call{recv: g.ptrAs(x, repOptRef), head: ".is_some_and", open: "(", close: ")", args: []node{atom{"|" + name + "| " + eq.flat(), precAtom}}})
}

// heapRecv reports whether method m has a pointer receiver of a heap
// pointer type: it takes the pointer itself, &Ptr<T>, so its accesses are
// each their own borrow.
func (g *gen) heapRecv(m *types.Func) bool {
	r := m.Signature().Recv()
	return r != nil && isPointerType(r.Type()) && g.heapPtr(r.Type()) && !g.ownSelf(m)
}

// ownSelf reports whether method m's receiver is an owned pointer that is
// never nil: &self (RFC 0017, step 2).
func (g *gen) ownSelf(m *types.Func) bool {
	r := m.Signature().Recv()
	return r != nil && g.ownPtr(r.Type()) && g.ptrs.NonNilVar(r)
}

// heapRecvArg renders the receiver of a call of a method with a heap
// pointer receiver that may be nil: the pointer, lent.
func (g *gen) heapRecvArg(x ast.Expr, p *borrow.Param) node {
	return g.arg(ast.Unparen(x), p)
}

// printMethodOf returns the method fmt prints a value of type t through:
// of its value, or, for a heap pointer, of the pointer's method set.
func printMethodOf(t types.Type) *types.Func {
	if isPointerType(t) {
		return subset.PtrPrintMethod(t)
	}
	return subset.PrintMethod(t)
}

// printCall renders the call of print method m on e, of type t.
func (g *gen) printCall(e ast.Expr, t types.Type, m *types.Func) node {
	if isPointerType(t) {
		return g.ptrShow(g.ref(e), t, m)
	}
	return g.methodCall(e, m, nil)
}

// ptrShow renders %v of the heap pointer p (a reference to its Option, or
// to a borrow of one) of type t through its print method m: rt::show_opt
// prints <nil> where Go's fmt does, for a method that panics on a nil
// pointer.
func (g *gen) ptrShow(p node, t types.Type, m *types.Func) node {
	pt := t.(*types.Pointer)
	if fn := g.res.Funcs[m]; !g.unboxed(pt) && fn.Recv.Mode == borrow.Ref && !g.paramRep(fn.Recv.Var).optional() {
		// &self: the method itself, lent the object (clippy prefers the
		// path); a nil pointer, which it would dereference, prints <nil>.
		g.rt["show_ref"] = true
		head := g.qual(m.Pkg(), g.types[g.recvTypeName(m)]) + "::" + g.funcs[m]
		if g.freeMethods(g.recvTypeName(m)) {
			head = g.qual(m.Pkg(), g.funcs[m])
		}
		return &call{head: "rt::show_ref", open: "(", close: ")", args: []node{p, text(head)}}
	}
	g.rt["show_opt"] = true
	return &call{head: "rt::show_opt", open: "(", close: ")", args: []node{p, atom{"|x| " + g.methodOnOwned(m, t.(*types.Pointer), "x", nil), precAtom}}}
}

// methodOnOwned renders method m called on v, a reference to an owned
// pointer of type p held in an interface's dynamic value (RFC 0017): the
// value in its box lent to the method, or a copy of it for a value
// receiver that keeps or changes its own copy.
func (g *gen) methodOnOwned(m *types.Func, p *types.Pointer, v string, as []string) string {
	fn := g.res.Funcs[m]
	mut := fn.Recv.Mode == borrow.RefMut
	view := ".as_deref"
	if g.unboxed(p) {
		view = ".as_ref"
	}
	if mut {
		view = map[string]string{".as_deref": ".as_deref_mut", ".as_ref": ".as_mut"}[view]
	}
	lent := v + view + "()"
	var recv string
	switch {
	case !isPointerType(fn.Recv.Var.Type()) && fn.Recv.Mode == borrow.Owned:
		recv = lent + ".unwrap().clone()" // a value receiver's own copy
	case !isPointerType(fn.Recv.Var.Type()) && borrow.KindOf(p.Elem()) == borrow.Copy:
		recv = "*" + lent + ".unwrap()"
	case isPointerType(fn.Recv.Var.Type()) && g.paramRep(fn.Recv.Var).optional():
		recv = lent // the method takes the pointer, which may be nil
	default:
		recv = lent + ".unwrap()" // &self or &mut self; a nil pointer panics, as Go's does
	}
	if g.freeMethods(g.recvTypeName(m)) {
		return g.qual(m.Pkg(), g.funcs[m]) + "(" + strings.Join(append([]string{recv}, as...), ", ") + ")"
	}
	if strings.HasPrefix(recv, "*") || !g.ownSelf(m) && isPointerType(fn.Recv.Var.Type()) {
		head := g.qual(m.Pkg(), g.types[g.recvTypeName(m)]) + "::" + g.funcs[m]
		return head + "(" + strings.Join(append([]string{recv}, as...), ", ") + ")"
	}
	return recv + "." + g.funcs[m] + "(" + strings.Join(as, ", ") + ")"
}
