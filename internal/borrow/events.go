package borrow

import (
	"github.com/pigfox/gopatina/internal/load"
	"github.com/pigfox/gopatina/internal/subset"
	"go/ast"
	"go/constant"
	"go/printer"
	"go/token"
	"go/types"
	"strings"
)

// An evKind is what an expression does with a variable.
type evKind int

const (
	evRead    evKind = iota // reads it (or a part of it)
	evMut                   // changes it (or a part of it) in place
	evConsume               // hands its value to a new owner
	evDef                   // assigns the whole variable a new value
	evBorrow                // takes its address, for a pointer variable
)

// An event is one use of a local variable, in evaluation order.
type event struct {
	cloneWhy string // for a refused interface clone: why it cannot move
	kind     evKind
	v        *types.Var // the root variable of the place
	e        ast.Expr   // the place expression (nil for evDef)
	path     string     // the place as a path, such as "m.Body[]"
	// indirect: the place goes through a slice element or a pointer, so
	// a change is seen by every Go copy that shares the memory.
	indirect bool
	whole    bool // the place is the variable itself
	elem     bool // the place is (inside) a slice element
	ref      bool // the variable is a borrow (a reference binding)
	pos      token.Pos

	// For evConsume: where the value goes.
	into     string // "render", "out.Queue", "first", "a return"
	intoLine int
	param    *Param // the parameter it is handed to, if any
	force    bool   // it is also borrowed earlier in the same call
	// readBy names the call an earlier operand of which reads the
	// consumed value, which is copied so the read keeps Go's meaning.
	readBy string
	// For evBorrow: the pointer variable being defined.
	ptr *types.Var
	// For evDef: the variable is being declared and initialised.
	init bool
	// A self-append to a slice, not a write to its existing elements.
	grow bool
	// A write to the header itself, not an element write or a self-append.
	replacesSlice bool
}

// walker turns expressions and simple statements into events.
type walker struct {
	a   *analyzer
	fn  *Func
	evs []event
	// stmtCall is the call that is the whole expression of the statement
	// being walked, if any.
	stmtCall ast.Expr
}

func (w *walker) info() *types.Info { return w.a.pkg.Info }

func (w *walker) add(ev event) {
	w.evs = append(w.evs, ev)
}

// passedClosures records, at a call of the program, the changes made by
// the closures it is passed (RFC 0018, step 2): the callee calls them.
func (w *walker) passedClosures(e *ast.CallExpr) {
	info := w.info()
	for _, a := range e.Args {
		var cl *subset.Closure
		switch a := ast.Unparen(a).(type) {
		case *ast.FuncLit:
			cl = subset.Inline(w.a.pkg)[a]
		case *ast.Ident:
			if v, ok := info.Uses[a].(*types.Var); ok {
				cl = subset.Closures(w.a.pkg)[v]
			}
		}
		if cl != nil {
			w.changes(cl, e)
		}
	}
}

// changes records that the closure cl changes, at e, the variables it
// changes.
func (w *walker) changes(cl *subset.Closure, e *ast.CallExpr) {
	for _, c := range cl.Captures {
		if cl.Writes[c] {
			w.add(event{kind: evMut, v: c, e: e.Fun, whole: true, pos: e.Pos()})
		}
	}
}

// localVar returns the local variable (or parameter) id names, or nil.
func (w *walker) localVar(id *ast.Ident) *types.Var {
	v, ok := w.info().Uses[id].(*types.Var)
	if !ok {
		v, ok = w.info().Defs[id].(*types.Var)
	}
	if !ok || v.IsField() || v.Pkg() == nil || v.Parent() == v.Pkg().Scope() || v.Parent() == nil {
		return nil
	}
	return v
}

// place decomposes a place expression into its root variable and path.
// ok is false when e is not a place rooted at a local variable.
func (w *walker) place(e ast.Expr) (ev event, ok bool) {
	ev = event{e: e, whole: true, pos: e.Pos()}
	var parts []string
	deref := false
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
			continue
		case *ast.Ident:
			v := w.localVar(x)
			if v == nil {
				return ev, false
			}
			ev.v = v
			if deref {
				// Through a pointer. A pointer initialised with &T{...}
				// owns its value, so that is not an indirection.
				if p := w.a.pointers[v]; p == nil || p.Target != nil {
					ev.indirect = true
				}
			}
			ev.ref = w.a.isRef(v)
			for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
				parts[i], parts[j] = parts[j], parts[i]
			}
			ev.path = x.Name + strings.Join(parts, "")
			return ev, true
		case *ast.SelectorExpr:
			if w.info().Selections[x] == nil || w.info().Selections[x].Kind() != types.FieldVal {
				return ev, false
			}
			ev.whole = false
			if _, isPtr := w.info().Types[x.X].Type.(*types.Pointer); isPtr {
				deref = true
			}
			parts = append(parts, "."+x.Sel.Name)
			e = x.X
		case *ast.IndexExpr:
			if t := w.info().Types[x.X].Type; !isSliceType(t) && !isMapType(t) {
				return ev, false // a byte of a string is a value
			}
			ev.whole = false
			ev.indirect = true
			ev.elem = true
			parts = parts[:0] // a change anywhere inside the element
			parts = append(parts, "[]")
			e = x.X
		case *ast.StarExpr:
			ev.whole = false
			deref = true
			e = x.X
		default:
			return ev, false
		}
	}
}

func isSliceType(t types.Type) bool {
	_, ok := t.Underlying().(*types.Slice)
	return ok
}

// isMapType reports whether t is a map type (RFC 0019): setting or
// deleting an entry changes the map.
func isMapType(t types.Type) bool {
	_, ok := t.Underlying().(*types.Map)
	return ok
}

// read records that e is evaluated for reading.
func (w *walker) read(e ast.Expr) { w.expr(e, evRead) }

// expr walks e, which is used as kind (evRead, evMut or evConsume).
func (w *walker) expr(e ast.Expr, kind evKind) {
	info := w.info()
	if tv, ok := info.Types[e]; ok && tv.Value != nil {
		return
	}
	if ev, ok := w.place(e); ok {
		w.subExprs(e)
		if kind == evConsume && KindOf(info.Types[e].Type) == Copy {
			kind = evRead
		}
		ev.kind = kind
		w.add(ev)
		return
	}
	switch e := e.(type) {
	case *ast.ParenExpr:
		w.expr(e.X, kind)
	case *ast.SelectorExpr:
		w.read(e.X)
	case *ast.IndexExpr:
		w.read(e.X)
		w.read(e.Index)
	case *ast.UnaryExpr:
		if subset.Ptrs(w.a.pkg).EscapeBox(e) != nil {
			w.consume(e.X, "a new pointer", nil) // x moves into its box
			return
		}
		w.read(e.X) // &x appears only as an argument or a pointer's start
	case *ast.BinaryExpr:
		if KindOf(w.info().Types[e.X].Type) == Copy {
			w.read(e.X)
			w.read(e.Y)
			return
		}
		// A string concatenation or comparison borrows both operands.
		ops := w.operands()
		ops.add(e.X, true, false, false, func() { w.read(e.X) })
		ops.add(e.Y, true, false, false, func() { w.read(e.Y) })
		ops.check(nil, "the "+e.Op.String()+" operator", e.Pos(), false)
	case *ast.CompositeLit:
		for _, el := range e.Elts {
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				el = kv.Value
			}
			w.consume(el, "a composite literal", nil)
		}
	case *ast.CallExpr:
		w.call(e)
	case *ast.TypeAssertExpr:
		w.read(e.X) // x.(T) copies the dynamic value out (RFC 0012)
	}
}

// subExprs walks the index expressions inside a place.
func (w *walker) subExprs(e ast.Expr) {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			w.subExprs(x.X)
			w.read(x.Index)
			return
		case *ast.StarExpr:
			e = x.X
		default:
			return
		}
	}
}

// wrappedArg returns the argument of fmt.Errorf call e that its %w
// wraps, or nil.
func wrappedArg(info *types.Info, e *ast.CallExpr) ast.Expr {
	// The subset takes a constant format only.
	verbs, _ := subset.ParseFormat(constant.StringVal(info.Types[e.Args[0]].Value))
	n := 1
	for _, v := range verbs {
		if v.Verb == 0 {
			continue
		}
		if v.Verb == 'w' && n < len(e.Args) {
			return e.Args[n]
		}
		n++
	}
	return nil
}

// consume records that e is handed to a new owner, described by into.
func (w *walker) consume(e ast.Expr, into string, p *Param) {
	before := len(w.evs)
	w.expr(e, evConsume)
	for i := before; i < len(w.evs); i++ {
		if w.evs[i].kind == evConsume && w.evs[i].e == ast.Unparen(e) {
			w.evs[i].into = into
			w.evs[i].intoLine = w.a.pkg.Fset.Position(e.Pos()).Line
			w.evs[i].param = p
		}
	}
}

// addressOf records &x. ptr is the pointer variable being defined, or nil
// when the address is passed straight to a parameter in mode mode.
func (w *walker) addressOf(e *ast.UnaryExpr, ptr *types.Var, exclusive bool) {
	x := ast.Unparen(e.X)
	if lit, ok := x.(*ast.CompositeLit); ok {
		w.expr(lit, evRead)
		return
	}
	ev, _ := w.place(x) // the subset allows & only on a variable, field or literal
	w.subExprs(x)
	switch {
	case ptr != nil:
		ev.kind = evBorrow
		ev.ptr = ptr
	case exclusive:
		ev.kind = evMut
	default:
		ev.kind = evRead
	}
	w.add(ev)
}

// call walks a call expression.
func (w *walker) call(e *ast.CallExpr) {
	info := w.info()
	fun := ast.Unparen(e.Fun)
	if tv := info.Types[fun]; tv.IsType() {
		w.read(e.Args[0])
		return
	}
	w.passedClosures(e)
	switch f := fun.(type) {
	case *ast.Ident:
		if b, ok := info.Uses[f].(*types.Builtin); ok {
			w.builtin(e, b.Name())
			return
		}
		if v, ok := info.Uses[f].(*types.Var); ok {
			// A closure's call (RFC 0018): its arguments by value, and the
			// variables it changes, changed here; or a call of a parameter
			// of function type.
			if fn := w.a.res.Closures[v]; fn != nil {
				w.args(w.operands(), e, fn, f.Name)
				if cl := subset.Closures(w.a.pkg)[v]; cl != nil {
					w.changes(cl, e) // a recursive one captures nothing
				}
				return
			}
			w.args(w.operands(), e, w.a.res.FuncParams[v], f.Name)
			return
		}
		callee := w.a.funcs[info.Uses[f].(*types.Func)]
		w.args(w.operands(), e, callee, f.Name)
	case *ast.SelectorExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			if pn, ok := info.Uses[id].(*types.PkgName); ok && w.a.pkg.Local[pn.Imported()] {
				if fn, ok := info.Uses[f.Sel].(*types.Func); ok && w.a.funcs[fn] != nil {
					// A function of another package of the program (RFC 0009).
					w.args(w.operands(), e, w.a.funcs[fn], id.Name+"."+f.Sel.Name)
					return
				}
			}
			if _, ok := info.Uses[id].(*types.PkgName); ok {
				name := id.Name + "." + f.Sel.Name
				ops := w.operands()
				for i, a := range e.Args {
					if i == 0 && (name == "fmt.Fprint" || name == "fmt.Fprintln" || name == "fmt.Fprintf") {
						ops.add(a, false, false, false, func() { w.expr(a, evMut) })
						continue
					}
					if i == 0 && subset.Sorts[name] {
						ops.add(a, false, false, false, func() { w.expr(a, evMut) }) // sorted in place
						continue
					}
					if i == 1 && name == "errors.As" {
						target := stripAddr(a) // errors.As sets it (RFC 0012)
						ops.add(target, false, false, false, func() { w.expr(target, evMut) })
						continue
					}
					if name == "fmt.Errorf" && a == wrappedArg(info, e) {
						// %w: the new error holds the one it wraps, which
						// moves in (RFC 0012); its message reads it first.
						ops.add(a, false, false, false, func() { w.consume(a, "fmt.Errorf's %w", nil) })
						continue
					}
					ops.add(a, true, false, false, func() { w.read(a) })
				}
				ops.check(nil, name, e.Pos(), false)
				return
			}
		}
		method, _ := info.Selections[f].Obj().(*types.Func)
		callee := w.a.funcs[method]
		name := exprString(f.X) + "." + f.Sel.Name
		if callee == nil { // err.Error(), or a provided method (RFC 0011)
			if t := w.a.res.IfaceKeeps(subset.Ifaces(w.a.pkg).Dyn, method); t != nil && subset.IsIface(info.TypeOf(f.X)) {
				w.a.report(e.Pos(), w.a.errorf(e.Pos(), "%s would move the %s out of the interface %s, which still owns it: %s keeps its receiver",
					name, types.TypeString(t, types.RelativeTo(w.a.pkg.Types)), exprString(f.X), f.Sel.Name).With(
					"an interface value owns its object (RFC 0017), so a method keeping its pointer receiver, called through one, would give the object a second owner",
					"call "+f.Sel.Name+" on the object before converting it to an interface, or have "+f.Sel.Name+" return a new object"))
			}
			if subset.IsIface(info.TypeOf(f.X)) && w.a.res.IfaceMut(subset.Ifaces(w.a.pkg).Dyn, method) {
				// Through an interface whose dynamic value the method changes:
				// the interface value is lent mutably (RFC 0012 with RFC 0017).
				w.expr(f.X, evMut)
				for _, a := range e.Args {
					w.read(a)
				}
				return
			}
			w.read(f.X)
			for _, a := range e.Args {
				w.read(a)
			}
			return
		}
		ops := w.operands()
		w.receiver(ops, f.X, callee)
		w.args(ops, e, callee, name)
	}
}

// receiver walks the receiver of a method call.
func (w *walker) receiver(ops *operands, x ast.Expr, callee *Func) {
	r := callee.Recv
	switch {
	case r.Mode == RefMut:
		ops.add(x, true, true, false, func() { w.expr(x, evMut) })
	case r.Mode == Owned && !isPointerT(r.Var.Type()) && subset.Ptrs(w.a.pkg).Owned(w.info().TypeOf(x)):
		// A value method through an owned pointer (RFC 0017) takes a copy
		// of the box's value: the pointer is only read.
		ops.add(x, true, false, false, func() { w.read(x) })
	case r.Mode == Owned:
		ops.add(x, false, false, false, func() { w.consume(x, exprString(x)+"'s method "+callee.Obj.Name(), r) })
	case r.Mode == Ref:
		ops.add(x, true, false, false, func() { w.read(x) })
	default:
		ops.add(x, false, false, false, func() { w.read(x) })
	}
}

// args walks the arguments of a call to a function in the package.
func (w *walker) args(ops *operands, e *ast.CallExpr, callee *Func, name string) {
	if s := suffixArg(e); s != nil {
		// The subset permits this only as the sole argument of a standalone
		// call. Read both operands even before we know whether it is safe.
		w.read(s.X)
		w.read(s.Low)
		if w.a.final {
			w.a.suffixes[e] = callee
		}
		return
	}
	borrowed := map[string]bool{}
	for i, a := range e.Args {
		p := callee.Params[i]
		if w.a.final && p.Mode != Ref && p.Kind != Copy && p.Kind != Value && isGlobal(w.a.pkg, a) {
			// A package-level slice nothing changes (RFC 0010) is shared
			// by every reader: a callee that changes or keeps its argument
			// would change what the others see.
			w.a.report(a.Pos(), w.a.errorf(a.Pos(), "%s may change or keep its argument, a package-level variable of type %s (patina mode passes one only to a function that just reads it)", name, p.Var.Type()))
		}
		start := len(w.evs)
		addr, isAddr := ast.Unparen(a).(*ast.UnaryExpr)
		isAddr = isAddr && addr.Op == token.AND
		switch p.Mode {
		case RefMut:
			ops.add(a, true, true, true, func() {
				if isAddr {
					w.addressOf(addr, nil, true)
				} else {
					w.expr(a, evMut)
				}
			})
		case Owned:
			ops.add(a, false, false, false, func() { w.consume(a, name, p) })
		case Ref:
			ops.add(a, true, false, false, func() {
				if isAddr {
					w.addressOf(addr, nil, false)
				} else {
					w.read(a)
				}
			})
		default:
			ops.add(a, false, false, false, func() { w.read(a) })
		}
		for j := start; j < len(w.evs); j++ {
			ev := &w.evs[j]
			if ev.kind == evMut && ev.e == ast.Unparen(a) && p.Mode == RefMut && isSliceType(p.Var.Type()) {
				// A callee borrowing a slice mutably can write its backing
				// array, not replace the caller's slice header.
				ev.indirect = true
			}
			if p.replacesSlice && ev.kind == evMut && ast.Unparen(ev.e) == ast.Unparen(stripAddr(a)) {
				ev.replacesSlice = true
			}
			if ev.kind == evConsume && ev.e == ast.Unparen(a) && borrowed[rootName(ev.path)] {
				ev.force = true
			}
		}
		if ev, ok := w.place(ast.Unparen(stripAddr(a))); ok && p.Mode != Owned {
			borrowed[rootName(ev.path)] = true
		}
	}
	ops.check(e, name, e.Pos(), e == w.stmtCall)
}

func stripAddr(e ast.Expr) ast.Expr {
	if u, ok := ast.Unparen(e).(*ast.UnaryExpr); ok && u.Op == token.AND {
		return u.X
	}
	return e
}

func rootName(path string) string {
	if i := strings.IndexAny(path, ".["); i >= 0 {
		return path[:i]
	}
	return path
}

// builtin walks a call of a built-in function.
func (w *walker) builtin(e *ast.CallExpr, name string) {
	switch name {
	case "append":
		// Only as s = append(s, ...): the statement records the change to s.
		for _, a := range e.Args[1:] {
			if e.Ellipsis.IsValid() {
				w.read(a)
				continue
			}
			w.consume(a, "append", nil)
		}
	case "delete":
		// delete(m, k) changes m (RFC 0019); the checker keeps m a variable.
		m := ast.Unparen(e.Args[0]).(*ast.Ident)
		w.add(event{kind: evMut, v: w.info().Uses[m].(*types.Var), e: m, whole: true, pos: e.Pos()})
		w.read(e.Args[1])
	default:
		for _, a := range e.Args {
			if tv := w.info().Types[a]; tv.IsType() {
				continue
			}
			w.read(a)
		}
	}
}

// exprString renders a Go expression for messages, as gofmt prints it.
func exprString(e ast.Expr) string {
	var b strings.Builder
	printer.Fprint(&b, token.NewFileSet(), e)
	return b.String()
}

// isGlobal reports whether e is a package-level variable of the program
// (os.Stdout is not one): v or pkg.V.
func isGlobal(pkg *load.Package, e ast.Expr) bool {
	var id *ast.Ident
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		id = x
	case *ast.SelectorExpr:
		id = x.Sel
	default:
		return false
	}
	v, ok := pkg.Info.Uses[id].(*types.Var)
	return ok && !v.IsField() && pkg.Local[v.Pkg()] && v.Parent() == v.Pkg().Scope()
}
