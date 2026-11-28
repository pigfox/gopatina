package borrow

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"

	"github.com/pigfox/gopatina/internal/diag"
	"github.com/pigfox/gopatina/internal/load"
	"github.com/pigfox/gopatina/internal/subset"
)

type analyzer struct {
	pkg      *load.Package
	funcs    map[*types.Func]*Func
	params   map[*types.Var]*Param
	pointers map[*types.Var]*Pointer
	refs     map[*types.Var]bool // range variables bound to a borrowed element
	res      *Result
	diags    map[token.Pos]diag.Diagnostic
	fixes    map[token.Pos]Fix
	// Suffix calls encountered while walking evaluated expressions.
	suffixes map[*ast.CallExpr]*Func
	// final is set for the last pass, which records decisions.
	final bool
}

// Check runs the borrow checker on pkg, which must already be inside the
// subset (see package subset). It returns the decisions, and the errors
// for code that no safe fix can express.
func Check(pkg *load.Package) (*Result, []diag.Diagnostic) {
	SetHeap(subset.Ptrs(pkg).HeapType)
	a := &analyzer{
		pkg:      pkg,
		funcs:    map[*types.Func]*Func{},
		params:   map[*types.Var]*Param{},
		pointers: map[*types.Var]*Pointer{},
		refs:     map[*types.Var]bool{},
		diags:    map[token.Pos]diag.Diagnostic{},
		fixes:    map[token.Pos]Fix{},
		suffixes: map[*ast.CallExpr]*Func{},
	}
	a.res = &Result{
		Funcs:        a.funcs,
		Actions:      map[ast.Expr]Action{},
		Mut:          map[*types.Var]bool{},
		Pointers:     a.pointers,
		Ranges:       map[*ast.RangeStmt]RangeForm{},
		RangeCopy:    map[*ast.RangeStmt]bool{},
		RangeMut:     map[*ast.RangeStmt]bool{},
		RangeMutVars: map[*types.Var]bool{},
		Hoist:        map[*ast.CallExpr]bool{},
		Suffixes:     map[*ast.CallExpr]*ast.SliceExpr{},
		Counted:      map[*ast.ForStmt]bool{},
		CopyFirst:    map[ast.Expr]bool{},
		RefElems:     a.refs,
		Closures:     map[*types.Var]*Func{},
		Lits:         map[*ast.FuncLit]*Func{},
		FuncParams:   map[*types.Var]*Func{},
		Lent:         map[*types.Var]bool{},
		params:       a.params,
	}
	var decls []*ast.FuncDecl
	var fns []*Func
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				decls = append(decls, fd)
				fns = append(fns, a.declare(fd, pkg.Info.Defs[fd.Name].(*types.Func)))
			}
		}
	}
	// A closure's body is checked as a function of its own (RFC 0018), its
	// returns its own; its calls take its parameters by value.
	for _, cl := range sortedClosures(pkg) {
		sig := pkg.Info.Types[cl.Lit].Type.(*types.Signature)
		obj := types.NewFunc(cl.Lit.Pos(), pkg.Types, "closure", sig)
		fd := &ast.FuncDecl{Name: &ast.Ident{Name: obj.Name(), NamePos: cl.Lit.Pos()}, Type: cl.Lit.Type, Body: cl.Lit.Body}
		fn := a.declare(fd, obj)
		a.res.Lits[cl.Lit] = fn
		if cl.Var != nil {
			a.res.Closures[cl.Var] = fn
		}
		fns = append(fns, fn)
		for v := range cl.Writes {
			a.res.Mut[v] = true // it changes v, so v is let mut
		}
	}
	// A parameter of function type (RFC 0018, step 2) is called with its
	// arguments by value.
	values := []*types.Var{}
	for v := range subset.FuncParams(pkg) {
		values = append(values, v)
	}
	for v := range subset.FuncVars(pkg) {
		values = append(values, v)
		a.res.Mut[v] = true // calling a Box<dyn FnMut> changes it (step 3)
	}
	for _, v := range values {
		sig := v.Type().Underlying().(*types.Signature)
		fn := &Func{Obj: types.NewFunc(v.Pos(), pkg.Types, v.Name(), sig)}
		for i := 0; i < sig.Params().Len(); i++ {
			fn.Params = append(fn.Params, a.newParam(sig.Params().At(i)))
		}
		a.res.FuncParams[v] = fn
	}
	// Infer parameter modes and pointer kinds until nothing changes. Every
	// fact only ever turns on, so this ends.
	for changed := true; changed; {
		changed = false
		for _, fn := range fns {
			if a.summarize(fn) {
				changed = true
			}
		}
	}
	a.final = true
	for _, fn := range fns {
		a.liveness(fn)
	}
	a.checkSuffixes()
	a.checkHeap(decls)
	a.cycleNotes()
	// A variable changed through a pointer to it is changed too.
	for _, p := range a.pointers {
		if p.Target != nil && p.Exclusive {
			a.res.Mut[p.Target] = true
			if param := a.params[p.Target]; param != nil {
				param.Mut = true
			}
		}
	}
	return a.result()
}

func (a *analyzer) result() (*Result, []diag.Diagnostic) {
	var ds []diag.Diagnostic
	for _, d := range a.diags {
		ds = append(ds, d)
	}
	diag.Sort(ds)
	var keys []token.Pos
	for pos := range a.fixes {
		keys = append(keys, pos)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, pos := range keys {
		a.res.Fixes = append(a.res.Fixes, a.fixes[pos])
	}
	return a.res, ds
}

func (a *analyzer) declare(fd *ast.FuncDecl, obj *types.Func) *Func {
	f := &Func{Obj: obj, Decl: fd}
	sig := obj.Signature()
	if r := sig.Recv(); r != nil {
		f.Recv = a.newParam(r)
	}
	for i := 0; i < sig.Params().Len(); i++ {
		f.Params = append(f.Params, a.newParam(sig.Params().At(i)))
	}
	a.funcs[obj] = f
	return f
}

// sortedClosures returns pkg's closures, bound and passed, in source
// order.
func sortedClosures(pkg *load.Package) []*subset.Closure {
	var out []*subset.Closure
	for _, cl := range subset.Closures(pkg) {
		out = append(out, cl)
	}
	for _, cl := range subset.Inline(pkg) {
		out = append(out, cl)
	}
	for _, cl := range subset.Returned(pkg) {
		out = append(out, cl)
	}
	for _, cl := range subset.Recursive(pkg) {
		out = append(out, cl)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Lit.Pos() < out[j].Lit.Pos() })
	return out
}

func (a *analyzer) newParam(v *types.Var) *Param {
	p := &Param{Var: v, Kind: KindOf(v.Type()), owned: subset.Ptrs(a.pkg).Owned(v.Type())}
	p.Mode = p.mode()
	a.params[v] = p
	return p
}

// mode derives a parameter's mode from what the callee does with it.
func (p *Param) mode() Mode {
	if p.owned {
		// An owned pointer (RFC 0016) is lent: changed through, or only
		// read (RFC 0017, step 3); or kept, moved in (step 8).
		if p.escapes {
			return Owned
		}
		if p.changesShared || p.changesFields {
			return RefMut
		}
		return Ref
	}
	switch p.Kind {
	case Copy:
		return ByValue
	case PointerKind:
		if p.changesShared {
			return RefMut
		}
		return Ref
	case Writer:
		return RefMut
	}
	switch {
	case p.escapes || p.reassigned || p.changesFields:
		return Owned
	case p.changesShared:
		return RefMut
	}
	return Ref
}

// isRef reports whether v is bound to a borrow rather than owning its
// value: a parameter passed as a borrow, or a range variable bound to an
// element.
func (a *analyzer) isRef(v *types.Var) bool {
	if p := a.params[v]; p != nil {
		return p.Mode == Ref || p.Mode == RefMut
	}
	return a.refs[v]
}

// summarize walks fn's body and updates what it does with its parameters
// and pointer variables. It reports whether anything changed.
func (a *analyzer) summarize(fn *Func) bool {
	w := &walker{a: a, fn: fn}
	w.block(fn.Decl.Body)
	changed := false
	set := func(b *bool) {
		if !*b {
			*b = true
			changed = true
		}
	}
	plan := subset.Ptrs(a.pkg)
	for _, ev := range w.evs {
		if c := plan.RefCall(ev.v); c != nil && ev.kind == evMut && plan.LocalRef(ev.v) {
			// s := c.Self(); s.Hit(): a change through the borrow a call
			// returned changes the call's input (RFC 0017, decision 4).
			if fn := a.funcs[calleeFunc(a.pkg.Info, c)]; fn != nil {
				in := plan.RetBorrow(fn.Obj)
				for _, p := range a.allParams(fn) {
					if p.Var == in {
						set(&p.changesShared)
					}
				}
			}
		}
		if p := a.params[ev.v]; p != nil && p.Kind != Copy {
			switch ev.kind {
			case evMut:
				if ptr, ok := p.Var.Type().(*types.Pointer); ok && ev.replacesSlice && isSliceType(ptr.Elem()) {
					set(&p.replacesSlice)
				}
				switch {
				case ev.indirect, p.Kind == PointerKind, p.owned:
					// Through the pointer, or the pointer passed on to change
					// its target: a reborrow. An owned pointer (RFC 0016) is
					// lent like any pointer (RFC 0017, step 3).
					set(&p.changesShared)
				case ev.whole:
					set(&p.reassigned)
				default:
					set(&p.changesFields)
				}
			case evDef:
				set(&p.reassigned)
			case evConsume:
				if ev.indirect || ev.elem {
					break // a copy taken out through a borrow
				}
				if !p.escapes {
					p.firstEscape = ev.pos
				}
				set(&p.escapes)
			case evBorrow:
				// A pointer to the parameter's own copy.
				set(&p.changesFields)
			}
		} else if p != nil && (ev.kind == evMut || ev.kind == evDef) {
			set(&p.Mut)
		}
		if ptr := a.pointers[ev.v]; ptr != nil && ev.kind == evMut && !ptr.Exclusive {
			ptr.Exclusive = true
			changed = true
		}
	}
	for _, p := range a.allParams(fn) {
		if m := p.mode(); m != p.Mode {
			p.Mode = m
			changed = true
		}
		if p.Kind != Copy && p.Mode == Owned && (p.reassigned || p.changesFields || p.changesShared) && !p.Mut {
			p.Mut = true
			changed = true
		}
	}
	return changed
}

func (a *analyzer) allParams(fn *Func) []*Param {
	ps := fn.Params
	if fn.Recv != nil {
		ps = append([]*Param{fn.Recv}, ps...)
	}
	return ps
}

func (a *analyzer) errorf(pos token.Pos, format string, args ...any) diag.Diagnostic {
	return diag.Errorf(a.pkg.Fset.Position(pos), format, args...)
}

// report records an error, once per position.
func (a *analyzer) report(pos token.Pos, d diag.Diagnostic) {
	if a.final {
		if _, ok := a.diags[pos]; !ok {
			a.diags[pos] = d
		}
	}
}

// fix records an automatic decision for -explain.
func (a *analyzer) fix(pos token.Pos, text string) {
	if a.final {
		a.fixes[pos] = Fix{Pos: a.pkg.Fset.Position(pos), Text: text}
	}
}

func (a *analyzer) line(pos token.Pos) int { return a.pkg.Fset.Position(pos).Line }
