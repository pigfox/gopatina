package borrow

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/pigfox/gopatina/internal/subset"
)

// A use is the nearest later use of a variable.
type use struct {
	pos  token.Pos
	path string
	kind evKind
}

// A set maps each live variable to its nearest later use.
type set map[*types.Var]use

func (s set) copy() set {
	t := make(set, len(s))
	for k, v := range s {
		t[k] = v
	}
	return t
}

// union returns the variables live in s or t, keeping the earlier use.
func union(s, t set) set {
	u := s.copy()
	for k, v := range t {
		if old, ok := u[k]; !ok || v.pos < old.pos {
			u[k] = v
		}
	}
	return u
}

func equal(s, t set) bool {
	if len(s) != len(t) {
		return false
	}
	for k, v := range s {
		if w, ok := t[k]; !ok || w != v {
			return false
		}
	}
	return true
}

// live computes, backwards through one function, which variables are
// still used after each point, and decides every hand-off with it.
type live struct {
	a  *analyzer
	fn *Func
	// Targets of break and continue, innermost last. A switch has no
	// continue target (nil).
	brk, cont []set
}

func (a *analyzer) liveness(fn *Func) {
	ast.Inspect(fn.Decl.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.RangeStmt:
			a.rangeForm(fn, n)
		case *ast.ForStmt:
			a.counted(fn, n)
		}
		return true
	})
	l := &live{a: a, fn: fn}
	l.stmts(fn.Decl.Body.List, set{})
	for _, p := range a.allParams(fn) {
		a.paramFix(fn, p)
	}
}

// paramFix records how a parameter is passed, for -explain, and rejects a
// pointer parameter that is kept.
func (a *analyzer) paramFix(fn *Func, p *Param) {
	// The subset checker already rejects a pointer that is stored, returned
	// or reassigned; a pointer parameter is always a borrow.
	if p.Kind == Copy || p.Kind == Writer || p.Kind == Error {
		return
	}
	who := "parameter " + p.Var.Name()
	if fn.Recv == p {
		who = "receiver " + p.Var.Name()
	}
	switch p.Mode {
	case Ref:
		a.fix(p.Var.Pos(), fmt.Sprintf("%s is a shared borrow: %s only reads it", who, fn.Obj.Name()))
	case RefMut:
		a.fix(p.Var.Pos(), fmt.Sprintf("%s is an exclusive borrow: %s changes it, and the caller sees the change", who, fn.Obj.Name()))
	case Owned:
		why := "keeps it"
		if !p.escapes {
			why = "changes its own copy"
		}
		a.fix(p.Var.Pos(), fmt.Sprintf("%s is owned: %s %s", who, fn.Obj.Name(), why))
	}
}

// rangeForm decides how a range loop is written.
func (a *analyzer) rangeForm(fn *Func, r *ast.RangeStmt) {
	info := a.pkg.Info
	w := &walker{a: a, fn: fn}
	w.block(r.Body)
	var root *types.Var
	path := ""
	if ev, ok := w.place(r.X); ok {
		root, path = ev.v, ev.path
	}
	grow, write, replaced := false, false, false
	var replacement token.Pos
	var valueVar *types.Var
	if id, ok := r.Value.(*ast.Ident); ok {
		valueVar = w.localVar(id)
	}
	valueChanged := false
	for _, ev := range w.evs {
		if ev.replacesSlice && !replacement.IsValid() && root != nil {
			target, targetPath := ev.v, ev.path
			if p := a.pointers[ev.v]; p != nil && p.Target != nil && ev.v != root {
				target, targetPath = p.Target, p.path
			}
			if target == root && path == targetPath {
				replacement = ev.pos
			}
		}
		if root != nil && ev.v == root && overlaps(ev.path, path) {
			switch {
			case ev.kind == evMut && ev.grow:
				grow = true
			case ev.kind == evDef, ev.kind == evConsume && ev.path == path:
				replaced = true
			case ev.kind == evMut, ev.kind == evBorrow:
				write = true
			}
		}
		if valueVar != nil && ev.v == valueVar && (ev.kind == evMut || ev.kind == evDef || ev.kind == evBorrow) {
			valueChanged = true
		}
	}
	t := info.Types[r.X].Type
	if !isSliceType(t) {
		if grow || write || replaced {
			a.res.Ranges[r] = RangeIndex // a string: iterate over a copy
			a.fix(r.X.Pos(), fmt.Sprintf("copied %s for the loop: the body changes it, and Go ranges over the value it had when the loop started", path))
		}
		return
	}
	switch {
	case replacement.IsValid():
		a.report(r.X.Pos(), a.errorf(r.X.Pos(), "%s is replaced on line %d inside the loop that ranges over it", path, a.line(replacement)).With(
			"Go ranges over the original slice even after it is replaced",
			"range over a copy (slices.Clone("+exprString(r.X)+")), or move the replacement after the loop"))
		return
	case replaced || (grow && write):
		a.report(r.X.Pos(), a.errorf(r.X.Pos(), "%s is replaced or both changed and appended to inside the loop that ranges over it", path).With(
			"Go ranges over the slice as it was when the loop started, so the loop would see some changes and not others",
			"range over a copy (slices.Clone("+path+")), or collect the changes and apply them after the loop"))
		return
	case grow || write:
		a.res.Ranges[r] = RangeIndex
	default:
		a.res.Ranges[r] = RangeIter
	}
	if valueVar == nil {
		return
	}
	if root == nil {
		return // a fresh slice: the loop owns its elements
	}
	elem := t.Underlying().(*types.Slice).Elem()
	k := KindOf(elem)
	if valueChanged {
		a.res.RangeCopy[r] = true
	}
	switch {
	case k == Copy:
	case k == Shared && (valueChanged || a.res.Ranges[r] == RangeIndex):
		a.report(r.Value.Pos(), a.errorf(r.Value.Pos(), "%s is a copy of an element of %s that shares memory with it", valueVar.Name(), path).With(
			"in Go, changes through one are seen through the other; patina mode does not copy such values on its own",
			"use "+path+"[i] directly"))
	case valueChanged || a.res.Ranges[r] == RangeIndex:
		a.fix(r.Value.Pos(), fmt.Sprintf("copied each element of %s into %s: a copy of a string, or of a struct of strings and numbers, keeps the program's meaning", path, valueVar.Name()))
	default:
		a.refs[valueVar] = true
	}
}

// overlaps reports whether one place path is a prefix of the other.
func overlaps(p, q string) bool {
	if len(p) > len(q) {
		p, q = q, p
	}
	return q == p || strings.HasPrefix(q, p+".") || strings.HasPrefix(q, p+"[")
}

// counted decides whether a 3-clause loop is a Rust range.
func (a *analyzer) counted(fn *Func, f *ast.ForStmt) {
	info := a.pkg.Info
	init, ok := f.Init.(*ast.AssignStmt)
	if !ok || init.Tok != token.DEFINE || len(init.Lhs) != 1 {
		return
	}
	id := init.Lhs[0].(*ast.Ident)
	v, _ := info.Defs[id].(*types.Var)
	if v == nil || !isInteger(v.Type()) {
		return
	}
	post, ok := f.Post.(*ast.IncDecStmt)
	if !ok || post.Tok != token.INC || !isIdent(post.X, id.Name) {
		return
	}
	cond, ok := f.Cond.(*ast.BinaryExpr)
	if !ok || (cond.Op != token.LSS && cond.Op != token.LEQ) || !isIdent(cond.X, id.Name) {
		return
	}
	w := &walker{a: a, fn: fn}
	w.block(f.Body)
	// The bound must not change: a constant, a variable the body does not
	// change, or len of one.
	bound := ast.Unparen(cond.Y)
	var boundVar *types.Var
	switch b := bound.(type) {
	case *ast.Ident:
		boundVar = w.localVar(b)
	case *ast.CallExpr:
		if fid, ok := b.Fun.(*ast.Ident); ok && fid.Name == "len" {
			if x, ok := ast.Unparen(b.Args[0]).(*ast.Ident); ok {
				boundVar = w.localVar(x)
			}
		}
		if boundVar == nil {
			return
		}
	}
	if info.Types[bound].Value == nil && boundVar == nil {
		return
	}
	for _, ev := range w.evs {
		if ev.v == v && ev.kind != evRead {
			return
		}
		if ev.v == nil || ev.v != boundVar {
			continue
		}
		// A mutable borrow of the whole variable (such as clear(&s)) can
		// replace a slice header. An indirect write to a slice's elements,
		// including through a []T parameter, cannot change its length.
		if ev.kind == evDef || ev.kind == evConsume || ev.kind == evBorrow || ev.grow ||
			(ev.kind == evMut && (isInteger(boundVar.Type()) || (ev.whole && !ev.indirect))) {
			return
		}
	}
	a.res.Counted[f] = true
}

func isIdent(e ast.Expr, name string) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && id.Name == name
}

func isInteger(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsInteger != 0
}

func (l *live) stmts(list []ast.Stmt, out set) set {
	for i := len(list) - 1; i >= 0; i-- {
		out = l.stmt(list[i], out)
	}
	return out
}

// walk collects the events of f into a fresh walker.
func (l *live) walk(f func(w *walker)) []event {
	w := &walker{a: l.a, fn: l.fn}
	f(w)
	return w.evs
}

func (l *live) stmt(s ast.Stmt, out set) set {
	switch s := s.(type) {
	case *ast.BlockStmt:
		return l.stmts(s.List, out)
	case *ast.IfStmt:
		in := l.stmts(s.Body.List, out)
		if s.Else != nil {
			in = union(in, l.stmt(s.Else, out))
		} else {
			in = union(in, out)
		}
		in = l.apply(l.walk(func(w *walker) { w.read(s.Cond) }), in)
		return l.simple(s.Init, in)
	case *ast.ForStmt:
		head := out.copy()
		for {
			post := l.simple(s.Post, head)
			body := l.loopBody(s.Body, out, post)
			next := body
			if s.Cond != nil {
				next = l.apply(l.walk(func(w *walker) { w.read(s.Cond) }), union(out, body))
			}
			next = union(next, head)
			if equal(next, head) {
				break
			}
			head = next
		}
		return l.simple(s.Init, head)
	case *ast.RangeStmt:
		head := out.copy()
		var defs []*types.Var
		for _, e := range []ast.Expr{s.Key, s.Value} {
			if id, ok := e.(*ast.Ident); ok {
				if v := l.a.pkg.Info.Defs[id]; v != nil {
					defs = append(defs, v.(*types.Var))
				}
			}
		}
		for {
			body := l.loopBody(s.Body, out, head)
			for _, v := range defs {
				delete(body, v)
			}
			next := union(union(out, body), head)
			if equal(next, head) {
				break
			}
			head = next
		}
		return l.apply(l.walk(func(w *walker) { w.read(s.X) }), head)
	case *ast.SwitchStmt:
		l.brk = append(l.brk, out)
		l.cont = append(l.cont, nil)
		in := set{}
		hasDefault := false
		for _, c := range s.Body.List {
			c := c.(*ast.CaseClause)
			in = union(in, l.stmts(c.Body, out))
			hasDefault = hasDefault || c.List == nil
		}
		l.brk, l.cont = l.brk[:len(l.brk)-1], l.cont[:len(l.cont)-1]
		if !hasDefault {
			in = union(in, out)
		}
		for i := len(s.Body.List) - 1; i >= 0; i-- {
			c := s.Body.List[i].(*ast.CaseClause)
			for j := len(c.List) - 1; j >= 0; j-- {
				in = l.apply(l.walk(func(w *walker) { w.read(c.List[j]) }), in)
			}
		}
		if s.Tag != nil {
			in = l.apply(l.walk(func(w *walker) { w.read(s.Tag) }), in)
		}
		return l.simple(s.Init, in)
	case *ast.TypeSwitchStmt:
		// As a switch whose cases compare no values (RFC 0012); each
		// clause's variable is defined at its start.
		l.brk = append(l.brk, out)
		l.cont = append(l.cont, nil)
		in := set{}
		hasDefault := false
		for _, c := range s.Body.List {
			c := c.(*ast.CaseClause)
			body := l.stmts(c.Body, out)
			if v, ok := l.a.pkg.Info.Implicits[c].(*types.Var); ok {
				delete(body, v)
			}
			in = union(in, body)
			hasDefault = hasDefault || c.List == nil
		}
		l.brk, l.cont = l.brk[:len(l.brk)-1], l.cont[:len(l.cont)-1]
		if !hasDefault {
			in = union(in, out)
		}
		return l.apply(l.walk(func(w *walker) { w.read(typeSwitchX(s)) }), in)
	case *ast.ReturnStmt:
		in := set{}
		if len(s.Results) == 0 {
			res := l.fn.Obj.Signature().Results()
			for i := 0; i < res.Len(); i++ {
				in[res.At(i)] = use{pos: s.Pos(), path: res.At(i).Name()}
			}
		}
		return l.simple(s, in)
	case *ast.BranchStmt:
		if s.Tok == token.BREAK {
			return l.brk[len(l.brk)-1].copy()
		}
		for i := len(l.cont) - 1; ; i-- {
			if l.cont[i] != nil {
				return l.cont[i].copy()
			}
		}
	case *ast.ExprStmt:
		if diverges(l.a.pkg.Info, s.X) {
			out = set{}
		}
	}
	return l.simple(s, out)
}

// loopBody computes the variables live at the start of a loop body.
func (l *live) loopBody(body *ast.BlockStmt, brk, cont set) set {
	l.brk = append(l.brk, brk)
	l.cont = append(l.cont, cont)
	in := l.stmts(body.List, cont)
	l.brk, l.cont = l.brk[:len(l.brk)-1], l.cont[:len(l.cont)-1]
	return in
}

// diverges reports whether e, a call, never returns: os.Exit or panic.
func diverges(info *types.Info, e ast.Expr) bool {
	call := ast.Unparen(e).(*ast.CallExpr)
	if f, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
		return f.Name == "panic" && info.Uses[f] == types.Universe.Lookup("panic")
	}
	fn, _ := info.Uses[ast.Unparen(call.Fun).(*ast.SelectorExpr).Sel].(*types.Func)
	return fn != nil && fn.Pkg() != nil && fn.Pkg().Path() == "os" && fn.Name() == "Exit"
}

func (l *live) simple(s ast.Stmt, out set) set {
	if s == nil {
		return out
	}
	return l.apply(l.walk(func(w *walker) { w.simple(s) }), out)
}

// apply processes events backwards from the live set after them, deciding
// each one, and returns the live set before them.
func (l *live) apply(evs []event, out set) set {
	lv := out.copy()
	for i := len(evs) - 1; i >= 0; i-- {
		ev := evs[i]
		l.conflicts(ev, lv)
		l.mutability(ev)
		if ev.kind == evDef {
			delete(lv, ev.v)
			continue
		}
		if ev.kind == evConsume {
			l.handOff(ev, lv)
		}
		lv[ev.v] = use{pos: ev.pos, path: ev.path, kind: ev.kind}
	}
	return lv
}

// mutability records the variables whose Rust binding must be mut.
func (l *live) mutability(ev event) {
	a := l.a
	switch {
	case ev.kind == evDef && !ev.init, ev.kind == evMut:
	default:
		return
	}
	v := ev.v
	if p := a.params[v]; p != nil {
		return // decided by the parameter's mode
	}
	if ptr := a.pointers[v]; ptr != nil {
		if ptr.Target != nil && ev.kind == evMut && ev.indirect {
			return // a change through a borrow
		}
	}
	a.res.Mut[v] = true
}

// conflicts reports a use of a variable while a pointer to it is still
// in use.
func (l *live) conflicts(ev event, lv set) {
	a := l.a
	var p *types.Var
	for q := range lv {
		ptr := a.pointers[q]
		if ptr == nil || ptr.Target != ev.v || q == ev.ptr || (!ptr.Exclusive && ev.kind == evRead) {
			continue
		}
		if p == nil || ptr.pos < a.pointers[p].pos {
			p = q
		}
	}
	if p != nil {
		ptr, u := a.pointers[p], lv[p]
		a.report(ev.pos, a.errorf(ev.pos, "%s is used while the pointer %s, taken on line %d, is still in use (line %d)",
			ev.v.Name(), p.Name(), a.line(ptr.pos), a.line(u.pos)).With(
			fmt.Sprintf("use %s through %s, or finish with %s before using %s again", ev.v.Name(), p.Name(), p.Name(), ev.v.Name())))
	}
	l.borrowConflicts(ev, lv)
}

// borrowConflicts reports a change to a variable, or a move of it, while a
// temporary borrow of it (RFC 0017, step 5) is still in use: in Rust the
// borrow's owner cannot change until the borrow's last use.
func (l *live) borrowConflicts(ev event, lv set) {
	if ev.kind != evMut && ev.kind != evConsume && ev.kind != evDef {
		return
	}
	a := l.a
	plan := subset.Ptrs(a.pkg)
	for q, u := range lv {
		if q == ev.v || !plan.Owned(q.Type()) || !plan.BorrowVar(q) || !plan.BorrowSources(q)[ev.v] {
			continue
		}
		what := "changed"
		if ev.kind == evConsume {
			what = "moved"
		}
		a.report(ev.pos, a.errorf(ev.pos, "%s is %s on line %d while %s, a borrow of it, is still in use on line %d",
			ev.v.Name(), what, a.line(ev.pos), q.Name(), a.line(u.pos)).With(
			fmt.Sprintf("%s only borrows %s (it is never stored or returned), so %s cannot change until %s's last use (RFC 0017)", q.Name(), ev.v.Name(), ev.v.Name(), q.Name()),
			fmt.Sprintf("finish with %s before %s changes, or read what you need from %s into a variable first", q.Name(), ev.v.Name(), q.Name())))
		return
	}
}
