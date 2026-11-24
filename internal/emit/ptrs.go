package emit

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/pigfox/gopatina/internal/borrow"
	"github.com/pigfox/gopatina/internal/subset"
)

// Pointers that escape (RFC 0013). A heap pointer is an rt::Ptr<T>, a
// shared cell; a variable whose address escapes is declared as one. Every
// read through a heap pointer is p.with(|v| ...) and every write
// p.with_mut(|v| ...), so a borrow of a cell lasts for the one access: the
// expression inside is rendered by the usual code with the cell's contents,
// v, standing for the pointed-at variable (cellRoot), and a write computes
// its value first.

// heapPtr reports whether t is a heap pointer type.
func (g *gen) heapPtr(t types.Type) bool {
	return t != nil && g.ptrs != nil && g.ptrs.HeapType(t)
}

// cellVar reports whether v lives in a cell: its address escapes.
func (g *gen) cellVar(v *types.Var) bool {
	return v != nil && g.ptrs != nil && g.ptrs.Cells[v]
}

// ownPtr reports whether t is a heap pointer type that is never shared
// (RFC 0016): a box with one owner, read through as a plain borrow.
func (g *gen) ownPtr(t types.Type) bool {
	return g.heapPtr(t) && g.ptrs.Owned(t)
}

// ptrKind returns the runtime type of shared heap pointer type t, Ptr,
// and marks it used.
func (g *gen) ptrKind(t types.Type) string {
	g.rt["Ptr"] = true
	return "Ptr"
}

// ptrType renders the Rust type of heap pointer type t: an owned pointer
// is Option<Box<T>> (RFC 0016), as Rust writes a pointer that may be nil;
// a shared one is a cell.
func (g *gen) ptrType(t types.Type) string {
	elem := g.rustType(types.Unalias(t).(*types.Pointer).Elem())
	if g.ownPtr(t) {
		return "Option<" + g.boxOf(t) + ">"
	}
	return "rt::" + g.ptrKind(t) + "<" + elem + ">"
}

// ptrNil renders nil of heap pointer type t.
func (g *gen) ptrNil(t types.Type) node {
	if g.ownPtr(t) {
		return text("None")
	}
	return text("rt::" + g.ptrKind(t) + "::nil()")
}

// ptrNew renders a new heap pointer of type t holding v: Go's &T{...}.
// An owned one is a Box<T>, never nil (RFC 0017, step 2); value wraps it
// in Some where an Option<Box<T>> is wanted.
func (g *gen) ptrNew(t types.Type, v node) node {
	if g.unboxed(t) {
		return v // the value itself (unboxed)
	}
	if g.ownPtr(t) {
		return &call{head: "Box::new", open: "(", close: ")", args: []node{v}}
	}
	return &call{head: "rt::" + g.ptrKind(t) + "::new", open: "(", close: ")", args: []node{v}}
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
	if !g.ownPtr(t) {
		return false
	}
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
	if g.changesThrough(g.fn.Decl.Body, &ast.Ident{Name: v.Name()}) {
		return repRefMut
	}
	return repRef
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
		if fn := g.programFunc(x); fn != nil && g.ptrs.NonNilResult(fn.Obj) {
			return repBox
		}
	case *ast.UnaryExpr:
		return repBox // &T{...}, the only owned pointer taken
	}
	return repOptBox
}

// paramRep returns how parameter v, of an owned pointer type, is held.
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
	id := ast.Unparen(e).(*ast.Ident) // an Option<&mut T> is a parameter
	return g.usedAfterPos(id, id.Pos())
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

// heapRoot returns the point where place e goes through a heap pointer:
// the node that stands for the pointed-at variable (a cell variable, *p,
// or p in p.f), and the expression whose value is the pointer. It
// reports false for a place that does not go through one.
func (g *gen) heapRoot(e ast.Expr) (root, ptr ast.Expr, ok bool) {
	if g.ptrs == nil || !g.ptrs.Heap() {
		return nil, nil, false
	}
	e = ast.Unparen(e)
	for {
		if _, done := g.cellRoot[e]; done {
			return nil, nil, false // being rendered inside its cell
		}
		switch x := e.(type) {
		case *ast.Ident:
			if g.cellVar(g.localVar(x)) {
				return x, x, true
			}
			return nil, nil, false
		case *ast.StarExpr:
			if g.heapPtr(g.typeOf(x.X)) && !g.ownPtr(g.typeOf(x.X)) {
				return x, x.X, true
			}
			return nil, nil, false
		case *ast.SelectorExpr:
			sel := g.info.Selections[x]
			if sel == nil || sel.Kind() != types.FieldVal {
				return nil, nil, false
			}
			base := ast.Unparen(x.X)
			if _, done := g.cellRoot[base]; !done && g.heapPtr(g.typeOf(base)) && !g.ownPtr(g.typeOf(base)) {
				return base, base, true
			}
			e = base
		case *ast.IndexExpr:
			if !isSlice(g.typeOf(x.X)) {
				return nil, nil, false
			}
			e = ast.Unparen(x.X)
		default:
			return nil, nil, false
		}
	}
}

// ptrValue renders the pointer a heap root reads through, without moving
// it: a variable by its name, anything else as a value.
func (g *gen) ptrValue(ptr ast.Expr) node {
	if id, ok := ptr.(*ast.Ident); ok {
		if v := g.localVar(id); v != nil {
			return text(g.varName(v))
		}
	}
	return g.value(ptr)
}

// cellParam returns the name of the closure parameter that stands for a
// cell's contents in the current function.
func (g *gen) cellParam() string {
	if g.cellName == "" {
		g.cellName = g.lnames.name("", func(string) string { return "v" })
	}
	return g.cellName
}

// inCell renders, with render, an expression over the contents of the cell
// that root reads through, and wraps it in ptr.with(|v| ...) (or with_mut).
func (g *gen) inCell(root, ptr ast.Expr, method string, render func() node) node {
	p := g.ptrValue(ptr)
	name := g.cellParam()
	g.cellRoot[root] = name
	saved := g.inCellRead
	g.inCellRead = true
	n := render()
	g.inCellRead = saved
	delete(g.cellRoot, root)
	return &call{recv: p, head: "." + method, open: "(", close: ")", args: []node{atom{"|" + name + "| " + n.flat(), precAtom}}}
}

// heapValue renders e, a place that goes through a heap pointer, as a value
// the caller owns: a copy of a Copy value, a clone of anything else, which
// for a pointer field is a copy of the pointer.
func (g *gen) heapValue(e ast.Expr) (node, bool) {
	root, ptr, ok := g.heapRoot(e)
	if !ok {
		return nil, false
	}
	if ast.Unparen(e) == root {
		// The whole variable: *p, or a cell variable.
		if borrow.KindOf(g.typeOf(e)) == borrow.Copy {
			return g.inCell(root, ptr, "with", func() node { return g.place(e) }), true
		}
		return &call{recv: g.ptrValue(ptr), head: ".load", open: "(", close: ")"}, true
	}
	return g.inCell(root, ptr, "with", func() node {
		p := g.place(e)
		if borrow.KindOf(g.typeOf(e)) == borrow.Copy {
			return p
		}
		return &call{recv: p, head: ".clone", open: "(", close: ")"}
	}), true
}

// heapWrite writes s, an assignment or ++/-- whose target goes through a
// heap pointer: the value first, then the store, in one borrow of the
// cell. It reports false for any other statement.
func (g *gen) heapWrite(s ast.Stmt) bool {
	var lhs ast.Expr
	var rhs []ast.Expr
	switch s := s.(type) {
	case *ast.AssignStmt:
		if s.Tok == token.DEFINE {
			for _, l := range s.Lhs {
				if id, ok := l.(*ast.Ident); ok && g.info.Defs[id] == nil && g.cellVar(g.localVar(id)) {
					return g.heapMulti(s) // x, err := ... with x an existing cell
				}
			}
			return false
		}
		if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
			return g.heapMulti(s)
		}
		lhs, rhs = s.Lhs[0], s.Rhs
	case *ast.IncDecStmt:
		lhs = s.X
	default:
		return false
	}
	root, ptr, ok := g.heapRoot(lhs)
	if !ok {
		return false
	}
	g.hoistIndexes([]ast.Expr{lhs}, true)
	if as, ok := s.(*ast.AssignStmt); ok && g.isSelfAppend(lhs, rhs[0]) {
		for _, a := range rhs[0].(*ast.CallExpr).Args[1:] {
			g.tempOf(a)
		}
		_ = as
	} else {
		for _, r := range rhs {
			g.tempOf(r)
		}
	}
	g.cellStmt(root, ptr, func() { g.stmt(s) })
	return true
}

// heapMulti writes a multiple assignment some of whose targets go through
// a heap pointer: every value first, then each store on its own.
func (g *gen) heapMulti(s *ast.AssignStmt) bool {
	any := false
	for _, l := range s.Lhs {
		if _, _, ok := g.heapRoot(l); ok {
			any = true
		}
	}
	if !any {
		return false
	}
	g.hoistIndexes(s.Lhs, true)
	var temps []string
	if len(s.Rhs) == 1 {
		for range s.Lhs {
			temps = append(temps, g.lnames.name("", func(string) string { return "val" }))
		}
		g.stmtLine("let ("+strings.Join(temps, ", ")+") = ", g.value(s.Rhs[0]), ";")
	} else {
		for _, r := range s.Rhs {
			temps = append(temps, g.tempOf(r))
		}
	}
	for i, l := range s.Lhs {
		if id, ok := l.(*ast.Ident); ok && id.Name == "_" {
			continue
		}
		if id, ok := l.(*ast.Ident); ok && g.info.Defs[id] != nil {
			v := g.info.Defs[id].(*types.Var) // declared here: x, err := ...
			if g.cellVar(v) {
				g.cellDecl(v, text(temps[i]))
			} else {
				g.line("let " + g.mutPrefix(v) + g.varName(v) + " = " + temps[i] + ";")
			}
			continue
		}
		if root, ptr, ok := g.heapRoot(l); ok {
			g.cellStmt(root, ptr, func() { g.stmtLine(g.place(l).flat()+" = ", text(temps[i]), ";") })
			continue
		}
		g.stmtLine(g.place(l).flat()+" = ", text(temps[i]), ";")
	}
	return true
}

// tempOf computes e, owned, into a new local first, unless it is a
// constant or a local variable, which the store can read itself.
func (g *gen) tempOf(e ast.Expr) string {
	if g.info.Types[e].Value != nil {
		return g.value(e).flat()
	}
	if id, ok := ast.Unparen(e).(*ast.Ident); ok && g.localVar(id) != nil && !g.cellVar(g.localVar(id)) && borrow.KindOf(g.typeOf(e)) == borrow.Copy {
		return g.varName(g.localVar(id))
	}
	name := g.lnames.name("", func(string) string { return "val" })
	g.stmtLine("let "+name+" = ", g.owned(e), ";")
	g.temps[ast.Unparen(e)] = name
	return name
}

// cellStmt writes, with write, a statement over the contents of the cell
// root reads through, inside ptr.with_mut(|v| ...).
func (g *gen) cellStmt(root, ptr ast.Expr, write func()) {
	pn := g.ptrValue(ptr)
	if _, simple := pn.(atom); !simple {
		// A pointer read through others: named first, which reads the
		// same (reads have no effects), so the write is one short line.
		tmp := g.lnames.name("", func(string) string { return "ptr" })
		g.stmtLine("let "+tmp+" = ", pn, ";")
		pn = text(tmp)
	}
	p := pn.flat()
	name := g.cellParam()
	saved, savedInd := g.out, g.ind
	var b strings.Builder
	g.out, g.ind = &b, 0
	g.cellRoot[root] = name
	g.inCellWrite = true
	write()
	g.inCellWrite = false
	delete(g.cellRoot, root)
	g.out, g.ind = saved, savedInd
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	head := p + ".with_mut(|" + name + "| "
	if len(lines) == 1 && strings.HasSuffix(lines[0], ";") {
		inner := atom{"|" + name + "| " + strings.TrimSuffix(lines[0], ";"), precAtom}
		n := &call{recv: pn, head: ".with_mut", open: "(", close: ")", args: []node{inner}}
		if fits(n, n.flat(), g.ind*indentWidth, 1) || !strings.Contains(p, "(") {
			g.stmtLine("", n, ";")
			return
		}
	}
	g.line(head + "{")
	g.ind++
	for _, l := range lines {
		g.line(l)
	}
	g.ind--
	g.line("});")
}

// readsHeap reports whether n reads through a heap pointer.
func (g *gen) readsHeap(n ast.Node) bool {
	if g.ptrs == nil || !g.ptrs.Heap() {
		return false
	}
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		if found {
			return false
		}
		if e, ok := x.(ast.Expr); ok {
			if _, _, ok := g.heapRoot(e); ok {
				found = true
			}
		}
		return true
	})
	return found
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
	if len(calls) == 0 || !g.readsHeap(s) && !g.ownedBeforeCall(s, calls) && !g.changedBeforeRead(s) {
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

// heapAddr renders &x for a heap pointer: a new cell for &T{...}, the cell
// itself for a cell variable (the only other operand the checker allows
// a heap pointer to be taken of).
func (g *gen) heapAddr(e *ast.UnaryExpr) node {
	if x, ok := ast.Unparen(e.X).(*ast.CompositeLit); ok {
		return g.ptrNew(g.typeOf(e), g.composite(x))
	}
	if v := g.ptrs.EscapeBox(e); v != nil {
		return g.ptrNew(g.typeOf(e), text(g.varName(v))) // x moves into its box
	}
	return &call{recv: text(g.varName(g.localVar(ast.Unparen(e.X).(*ast.Ident)))), head: ".clone", open: "(", close: ")"}
}

// heapCompare renders p == q, p != q, p == nil and p != nil for heap
// pointers.
func (g *gen) heapCompare(e *ast.BinaryExpr) node {
	operand := func(x ast.Expr) node {
		if id, ok := ast.Unparen(x).(*ast.Ident); ok && g.localVar(id) != nil {
			return text(g.varName(g.localVar(id)))
		}
		return g.value(x)
	}
	x, y := e.X, e.Y
	if g.info.Types[x].IsNil() {
		x, y = y, x
	}
	if g.info.Types[y].IsNil() {
		if g.ownPtr(g.typeOf(x)) {
			test := ".is_none"
			if e.Op == token.NEQ {
				test = ".is_some"
			}
			// Only an Option is tested: a new pointer compared with nil is
			// shared (RFC 0016), and one never nil is never compared.
			return &call{recv: g.rawPtr(x), head: test, open: "(", close: ")"}
		}
		var n node = &call{recv: operand(x), head: ".is_nil", open: "(", close: ")"}
		if root, ptr, ok := g.heapRoot(x); ok && ast.Unparen(x) != root {
			// p.f == nil tests the field inside p's cell: no copy of the
			// pointer, so no reference count changes.
			n = g.inCell(root, ptr, "with", func() node { return &call{recv: g.place(x), head: ".is_nil", open: "(", close: ")"} })
		}
		if e.Op == token.NEQ {
			n = atom{"!" + paren(n, precUnary), precUnary}
		}
		return n
	}
	return &binary{op: binOps[e.Op].op, l: operand(x), r: operand(y), p: precCmp, nonAssoc: true}
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
// pointer receiver: the pointer, or the cell of a cell variable (x.M()
// takes &x).
func (g *gen) heapRecvArg(x ast.Expr, p *borrow.Param) node {
	x = ast.Unparen(x)
	if u, ok := x.(*ast.UnaryExpr); ok && u.Op == token.AND {
		x = ast.Unparen(u.X)
	}
	if id, ok := x.(*ast.Ident); ok && g.cellVar(g.localVar(id)) {
		name := g.varName(g.localVar(id))
		if p.Mode == borrow.Ref {
			return text("&" + name)
		}
		return text(name + ".clone()")
	}
	return g.arg(x, p)
}

// cellDecl declares cell variable v holding value.
func (g *gen) cellDecl(v *types.Var, value node) {
	g.rt["Ptr"] = true
	g.stmtLine("let "+g.varName(v)+": "+g.ptrType(types.NewPointer(v.Type()))+" = ", &call{head: "rt::Ptr::new", open: "(", close: ")", args: []node{value}}, ";")
}

// cellShadow moves a variable just declared by other means into its cell.
func (g *gen) cellShadow(v *types.Var) {
	g.rt["Ptr"] = true
	name := g.varName(v)
	g.line("let " + name + " = rt::Ptr::new(" + name + ");")
}

// cellShadows moves the cell variables a multiple := declares into their
// cells.
func (g *gen) cellShadows(s *ast.AssignStmt) {
	if s.Tok != token.DEFINE {
		return
	}
	for _, l := range s.Lhs {
		if id, ok := l.(*ast.Ident); ok {
			if v, ok := g.info.Defs[id].(*types.Var); ok && g.cellVar(v) {
				g.cellShadow(v)
			}
		}
	}
}

// cellParamDecl moves parameter p, whose address escapes, into its cell.
func (g *gen) cellParamDecl(p *borrow.Param) {
	if p.Mode == borrow.Ref || p.Mode == borrow.RefMut {
		g.rt["Ptr"] = true
		name := g.varName(p.Var)
		own := ".clone()"
		if isStringType(p.Var.Type()) {
			own = ".to_string()" // a borrowed string is a &str
		}
		g.line("let " + name + " = rt::Ptr::new(" + name + own + ");")
		return
	}
	g.cellShadow(p.Var)
}

// cellCall renders a call of a program function that is lent a place
// inside a cell: &p.f, a slice p.f it changes or reads, or the receiver of
// x.M() for a method of x's type. The other arguments that read through a
// heap pointer are computed first; then the call runs inside the cell's one
// borrow, which the borrow checker has checked the callee cannot reach
// (borrow.HeapFreeFunc). It reports false for any other call.
func (g *gen) cellCall(e *ast.CallExpr) (node, bool) {
	if g.ptrs == nil || !g.ptrs.Heap() || g.inCellCall[e] {
		return nil, false
	}
	fn := g.programFunc(e)
	if fn == nil {
		return nil, false
	}
	var lent ast.Expr // the place lent, inside its cell
	mut := false
	if sel, ok := ast.Unparen(e.Fun).(*ast.SelectorExpr); ok && fn.Recv != nil {
		x := ast.Unparen(sel.X)
		if u, ok := x.(*ast.UnaryExpr); ok && u.Op == token.AND {
			x = ast.Unparen(u.X)
		}
		recv := fn.Recv.Var.Type()
		if !isPointerType(recv) && g.heapPtr(g.typeOf(x)) && !borrow.HeapExact(types.Unalias(g.typeOf(x)).(*types.Pointer).Elem()) {
			// p.M() for a value method: *p, inside its cell.
			g.inCellCall[e] = true
			saved := g.inCellWrite
			g.inCellWrite = true
			method := "with"
			if fn.Recv.Mode == borrow.RefMut {
				method = "with_mut"
			}
			n := g.inCell(x, x, method, func() node { return g.call(e) })
			g.inCellWrite = saved
			delete(g.inCellCall, e)
			return n, true
		}
		if _, _, ok := g.heapRoot(x); ok {
			switch {
			case isPointerType(recv) && !g.heapPtr(recv):
				lent, mut = x, true
			case !isPointerType(recv) && !borrow.HeapExact(g.typeOf(x)):
				lent, mut = x, fn.Recv.Mode == borrow.RefMut
			}
		}
	}
	for i, a := range e.Args { // variadic functions are not in the subset
		x := ast.Unparen(a)
		if u, ok := x.(*ast.UnaryExpr); ok && u.Op == token.AND && !g.heapPtr(g.typeOf(u)) {
			if _, _, ok := g.heapRoot(u.X); ok {
				lent, mut = u.X, true
			}
			continue
		}
		if _, _, ok := g.heapRoot(x); ok && !borrow.HeapExact(g.typeOf(x)) {
			lent, mut = x, fn.Params[i].Mode == borrow.RefMut
		}
	}
	if lent == nil {
		return nil, false
	}
	root, ptr, _ := g.heapRoot(lent)
	for _, a := range e.Args {
		if x := ast.Unparen(a); x != lent && !isAddrOf(x, lent) && g.readsHeap(x) {
			g.tempOf(x)
		}
	}
	method := "with"
	if mut {
		method = "with_mut"
	}
	g.inCellCall[e] = true
	saved := g.inCellWrite
	g.inCellWrite = true
	n := g.inCell(root, ptr, method, func() node { return g.call(e) })
	g.inCellWrite = saved
	delete(g.inCellCall, e)
	return n, true
}

// isAddrOf reports whether x is &y.
func isAddrOf(x, y ast.Expr) bool {
	u, ok := x.(*ast.UnaryExpr)
	return ok && u.Op == token.AND && ast.Unparen(u.X) == y
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

// ptrShow renders %v of the heap pointer p (a reference to it) of type t
// through its print method m: rt::show_ptr prints <nil> where Go's fmt
// does, for a method that panics on a nil pointer.
func (g *gen) ptrShow(p node, t types.Type, m *types.Func) node {
	g.rt["show_ptr"] = true
	show := g.methodOn(m, t, "x", nil)
	var f node = atom{"|x| " + show, precAtom}
	if head, ok := strings.CutSuffix(show, "(x)"); ok && !strings.Contains(head, "(") {
		f = text(head) // the method itself, as clippy prefers
	}
	return &call{head: "rt::show_ptr", open: "(", close: ")", args: []node{p, f}}
}

// methodOnPtr renders method m called on v, a reference to a heap pointer
// of type p, with the rendered arguments: the pointer itself for a
// pointer-receiver method, a copy of the value for a value-receiver one
// (the borrow checker allows that copy only where it is Go's).
func (g *gen) methodOnPtr(m *types.Func, p *types.Pointer, v string, as []string) string {
	fn := g.res.Funcs[m]
	free := g.freeMethods(g.recvTypeName(m))
	if g.heapRecv(m) {
		recv := v
		if fn.Recv.Mode != borrow.Ref {
			recv = v + ".clone()"
		}
		head := g.qual(m.Pkg(), g.types[g.recvTypeName(m)]) + "::" + g.funcs[m]
		if free {
			head = g.qual(m.Pkg(), g.funcs[m])
		}
		return head + "(" + strings.Join(append([]string{recv}, as...), ", ") + ")"
	}
	recv := v + ".load()"
	if free {
		if fn.Recv.Mode == borrow.Ref || fn.Recv.Mode == borrow.RefMut {
			recv = "&" + recv
		}
		return g.qual(m.Pkg(), g.funcs[m]) + "(" + strings.Join(append([]string{recv}, as...), ", ") + ")"
	}
	return recv + "." + g.funcs[m] + "(" + strings.Join(as, ", ") + ")"
}
