package borrow

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"github.com/pigfox/gopatina/internal/diag"
	"github.com/pigfox/gopatina/internal/subset"
)

// Places through pointers that escape (RFC 0013). A cell is borrowed for
// one access at a time, so what is read out of it is a copy. That copy is
// Go's own for a value of an exact type: a number, a string, a pointer, an
// interface value, or a struct of those. A slice, or a struct holding one,
// would no longer share its elements, so such a value is used only inside
// its cell: indexed, measured, written, appended to, printed, or lent to a
// call that cannot reach any heap pointer, which runs inside the cell's one
// borrow. Everything else is reported here.

// HeapExact reports whether a copy of a value of type t is Go's copy: it
// shares nothing that can change.
// (A struct cannot hold itself but through a pointer, so this ends.)
func HeapExact(t types.Type) bool {
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return true
	case *types.Pointer:
		return KindOf(t) != PointerKind
	case *types.Interface:
		return true
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if !HeapExact(u.Field(i).Type()) {
				return false
			}
		}
		return true
	}
	return false
}

// HeapFree reports whether no value of type t can reach a heap pointer, so
// a function whose parameters are all of such types cannot reach a cell.
func HeapFree(t types.Type) bool { return heapFree(t, map[types.Type]bool{}) }

func heapFree(t types.Type, seen map[types.Type]bool) bool {
	if seen[t] {
		return true
	}
	seen[t] = true
	switch u := t.Underlying().(type) {
	case *types.Pointer:
		return KindOf(t) == PointerKind && heapFree(u.Elem(), seen)
	case *types.Slice:
		return heapFree(u.Elem(), seen)
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if !heapFree(u.Field(i).Type(), seen) {
				return false
			}
		}
	}
	return true
}

// HeapFreeFunc reports whether fn's receiver and parameters are all heap
// free.
func HeapFreeFunc(fn *types.Func) bool {
	sig := fn.Signature()
	if r := sig.Recv(); r != nil && !HeapFree(r.Type()) {
		return false
	}
	for i := 0; i < sig.Params().Len(); i++ {
		if !HeapFree(sig.Params().At(i).Type()) {
			return false
		}
	}
	return true
}

// HeapPlace reports whether e is a place that goes through a heap pointer
// or is a variable in a cell.
func HeapPlace(info *types.Info, cells map[*types.Var]bool, e ast.Expr) bool {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.Ident:
			v, _ := info.Uses[x].(*types.Var)
			return v != nil && cells[v]
		case *ast.StarExpr:
			return heapPtrType(info.TypeOf(x.X))
		case *ast.SelectorExpr:
			sel := info.Selections[x]
			if sel == nil || sel.Kind() != types.FieldVal {
				return false
			}
			if heapPtrType(info.TypeOf(x.X)) {
				return true
			}
			e = x.X
		case *ast.IndexExpr:
			if _, ok := info.TypeOf(x.X).Underlying().(*types.Slice); !ok {
				return false
			}
			e = x.X
		default:
			return false
		}
	}
}

func heapPtrType(t types.Type) bool {
	_, ok := t.(*types.Pointer)
	return ok && KindOf(t) != PointerKind
}

// readOnlyStd are the standard functions that only read a slice they are
// given, so a copy of it reads the same.
var readOnlyStd = map[string]bool{"slices.Clone": true, "bytes.Clone": true, "strings.Join": true}

// checkHeap reports the uses of places through heap pointers that patina
// mode cannot write exactly.
func (a *analyzer) checkHeap(decls []*ast.FuncDecl) {
	plan := subset.Ptrs(a.pkg)
	if !plan.Heap() {
		return
	}
	info := a.pkg.Info
	unsupported := func(n ast.Node, what string) {
		d := diag.Errorf(a.pkg.Fset.Position(n.Pos()), "%s is not supported in patina mode yet", what)
		a.report(n.Pos(), d.With("see docs/compat.md for what patina mode supports"))
	}
	heap := func(e ast.Expr) bool { return HeapPlace(info, plan.Cells, e) }
	// A range over a slice in a cell ranges over a copy of it (patina mode
	// cannot hold the cell's borrow through the body), which is Go's range
	// unless the program changes an element of a slice of that type held
	// through a pointer, where Go's range would see it: no other copy of a
	// slice in a cell exists (one cannot be copied out).
	var ranges []*ast.RangeStmt
	a.heapWritten = nil
	for _, fd := range decls { // every one has a body (the subset checker)
		var stack []ast.Node
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			var parent ast.Node
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			stack = append(stack, n)
			switch n := n.(type) {
			case *ast.IfStmt:
				a.heapCondition(n.Cond, unsupported)
			case *ast.ForStmt:
				a.heapCondition(n.Cond, unsupported)
			case *ast.SwitchStmt:
				a.heapCondition(n.Tag, unsupported)
			case *ast.RangeStmt:
				if heap(n.X) {
					ranges = append(ranges, n)
				}
			case *ast.AssignStmt:
				for _, l := range n.Lhs {
					a.elemWrite(l, heap)
				}
			case *ast.IncDecStmt:
				a.elemWrite(n.X, heap)
			case *ast.CallExpr:
				a.heapCall(n, heap, unsupported)
			}
			e, ok := n.(ast.Expr)
			if !ok || !heap(e) || HeapExact(info.TypeOf(e)) {
				return true
			}
			if a.heapUseOK(e, parent) {
				return true
			}
			unsupported(e, fmt.Sprintf("copying a value of type %s out of a pointer that escapes (a copy would not share its elements)", info.TypeOf(e)))
			return false
		})
	}
	a.heapDynamic(unsupported)
	for _, r := range ranges {
		for _, w := range a.heapWritten {
			if types.Identical(w, info.TypeOf(r.X)) {
				unsupported(r.X, fmt.Sprintf("ranging over a slice held through a pointer in a program that changes the elements of a %s held through one", w))
				break
			}
		}
	}
}

// heapDynamic reports a pointer converted to an interface whose value
// methods would run on a copy that does not share the value's elements: a
// call through the interface copies *p out of its cell.
func (a *analyzer) heapDynamic(unsupported func(ast.Node, string)) {
	boxed := subset.Ifaces(a.pkg).Boxed
	var es []ast.Expr
	for e, t := range boxed {
		if heapPtrType(t) {
			es = append(es, e)
		}
	}
	sort.Slice(es, func(i, j int) bool { return es[i].Pos() < es[j].Pos() })
	seen := map[string]bool{}
	for _, e := range es {
		t := boxed[e].(*types.Pointer)
		if HeapExact(t.Elem()) || seen[t.String()] {
			continue
		}
		ms := types.NewMethodSet(t.Elem())
		if ms.Len() > 0 {
			seen[t.String()] = true
			unsupported(e, fmt.Sprintf("converting %s to an interface, where %s holds a slice and has value methods (a copy would not share its elements)", t, t.Elem()))
		}
	}
}

// elemWrite records the slice types whose elements the assignment target
// l changes through a heap pointer: s.f[i] = x, s.f[i].g++.
func (a *analyzer) elemWrite(l ast.Expr, heap func(ast.Expr) bool) {
	for {
		switch x := ast.Unparen(l).(type) {
		case *ast.IndexExpr:
			if heap(x.X) {
				a.heapWritten = append(a.heapWritten, a.pkg.Info.TypeOf(x.X))
			}
			l = x.X
		case *ast.SelectorExpr:
			l = x.X
		default:
			return
		}
	}
}

// lentSlices records the slice types inside a place lent to a call, whose
// elements the call may change.
func (a *analyzer) lentSlices(t types.Type, seen map[types.Type]bool) {
	if seen[t] {
		return
	}
	seen[t] = true
	switch u := t.Underlying().(type) {
	case *types.Pointer:
		a.lentSlices(u.Elem(), seen)
	case *types.Slice:
		a.heapWritten = append(a.heapWritten, t)
		a.lentSlices(u.Elem(), seen)
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			a.lentSlices(u.Field(i).Type(), seen)
		}
	}
}

// heapUseOK reports whether e, a place through a heap pointer of a type
// that is not exact, is used where patina mode reads or writes it inside
// its cell.
func (a *analyzer) heapUseOK(e ast.Expr, parent ast.Node) bool {
	info := a.pkg.Info
	switch p := parent.(type) {
	case *ast.SelectorExpr:
		return p.X == e
	case *ast.IndexExpr:
		return p.X == e
	case *ast.ParenExpr:
		return true
	case *ast.RangeStmt:
		return p.X == e // a copy, where that is Go's range (checkHeap)
	case *ast.UnaryExpr:
		return true // &e (no other operator applies): checked with its call
	case *ast.AssignStmt:
		for _, l := range p.Lhs {
			if l == e {
				return true // a write
			}
		}
		return false
	case *ast.CallExpr: // e is an argument: function values are not in the subset
		if id, ok := ast.Unparen(p.Fun).(*ast.Ident); ok {
			if b, ok := info.Uses[id].(*types.Builtin); ok {
				// len, or append, which the subset checker allows only as
				// s.f = append(s.f, ...): it grows the slice in its cell.
				return b.Name() == "len" || b.Name() == "append"
			}
		}
		if fn := calleeFunc(info, p); fn != nil {
			if fn.Pkg() != nil && (fn.Pkg().Path() == "fmt" || readOnlyStd[fn.Pkg().Path()+"."+fn.Name()]) {
				return true
			}
			if f := a.funcs[fn]; f != nil {
				return true // checked with the call (heapCall)
			}
		}
		return false
	}
	return false
}

// heapCall checks a call of a program function that is lent a place
// through a heap pointer: the call runs inside that cell's one borrow, so
// it must not reach any cell itself, and it is lent at most one.
func (a *analyzer) heapCall(c *ast.CallExpr, heap func(ast.Expr) bool, unsupported func(ast.Node, string)) {
	info := a.pkg.Info
	fn := calleeFunc(info, c)
	if fn == nil || a.funcs[fn] == nil {
		return
	}
	f := a.funcs[fn]
	lent := 0
	var at ast.Node
	if sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr); ok && f.Recv != nil {
		x := ast.Unparen(sel.X)
		if u, ok := x.(*ast.UnaryExpr); ok && u.Op == token.AND {
			x = ast.Unparen(u.X)
		}
		recvPtr := isPointerT(f.Recv.Var.Type())
		if !recvPtr && heapPtrType(info.TypeOf(x)) && !HeapExact(info.TypeOf(x).(*types.Pointer).Elem()) {
			// p.M() for a value method: *p, inside its cell.
			if f.Recv.Mode != Ref && f.Recv.Mode != RefMut {
				unsupported(x, "calling a method that changes its copy of a value held through a pointer that escapes")
				return
			}
			lent++
			at = x
		} else if heap(x) {
			switch {
			case recvPtr && KindOf(f.Recv.Var.Type()) == PointerKind:
				lent++ // x.M() takes &x inside the cell
				at = x
			case !recvPtr && !HeapExact(info.TypeOf(x)):
				if f.Recv.Mode != Ref && f.Recv.Mode != RefMut {
					unsupported(x, "calling a method that changes its copy of a value held through a pointer that escapes")
					return
				}
				lent++
				at = x
			}
		}
	}
	for i, arg := range c.Args { // variadic functions are not in the subset
		x := ast.Unparen(arg)
		borrowArg := false
		if u, ok := x.(*ast.UnaryExpr); ok && u.Op == token.AND && KindOf(info.TypeOf(u)) == PointerKind && heap(u.X) {
			borrowArg = true // &p.f
		}
		if heap(x) && !HeapExact(info.TypeOf(x)) {
			switch f.Params[i].Mode {
			case Ref, RefMut:
				borrowArg = true
			default:
				unsupported(x, fmt.Sprintf("handing a value of type %s held through a pointer that escapes to %s, which keeps it", info.TypeOf(x), fn.Name()))
				return
			}
		}
		if borrowArg {
			lent++
			at = x
		}
	}
	if lent == 0 {
		return
	}
	a.lentSlices(info.TypeOf(at.(ast.Expr)), map[types.Type]bool{})
	if lent > 1 {
		unsupported(c, "lending two places held through pointers that escape to one call")
		return
	}
	if !HeapFreeFunc(fn) {
		unsupported(at, fmt.Sprintf("lending a place held through a pointer that escapes to %s, which can reach pointers that escape", fn.Name()))
		return
	}
	// The other arguments are computed first, as values; one the call
	// changes must be the place itself, so it cannot read a cell.
	for i, arg := range c.Args {
		if x := ast.Unparen(arg); x != at && f.Params[i].Mode == RefMut && readsHeap(info, heap, x) {
			unsupported(x, "an argument the call changes that reads through a pointer that escapes, next to a place lent from one")
		}
	}
}

// heapCondition reports a condition that reads through a heap pointer and
// calls a function of the program: Go makes the call first, which patina
// mode writes for simple statements only.
func (a *analyzer) heapCondition(cond ast.Expr, unsupported func(ast.Node, string)) {
	if cond == nil || len(subset.HoistableCalls(a.pkg, cond)) == 0 {
		return
	}
	plan := subset.Ptrs(a.pkg)
	found := false
	ast.Inspect(cond, func(n ast.Node) bool {
		if e, ok := n.(ast.Expr); ok && HeapPlace(a.pkg.Info, plan.Cells, e) {
			found = true
		}
		return !found
	})
	if found {
		unsupported(cond, "a condition that reads through a pointer that escapes and calls a function of the program")
	}
}

// readsHeap reports whether e reads a place through a heap pointer.
func readsHeap(info *types.Info, heap func(ast.Expr) bool, e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if x, ok := n.(ast.Expr); ok && heap(x) {
			found = true
		}
		return !found
	})
	return found
}

func isPointerT(t types.Type) bool {
	_, ok := t.(*types.Pointer)
	return ok
}

// calleeFunc returns the function or method c calls, or nil.
func calleeFunc(info *types.Info, c *ast.CallExpr) *types.Func {
	switch f := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		fn, _ := info.Uses[f].(*types.Func)
		return fn
	case *ast.SelectorExpr:
		fn, _ := info.Uses[f.Sel].(*types.Func)
		return fn
	}
	return nil
}

// cycleNotes notes, for -explain, each pointer field (or slice of pointers)
// of a struct type of the program through which a value can reach a value
// of the same struct type: a cycle through it is not reclaimed, since a
// reference count frees no cycle (RFC 0013's known limitation).
func (a *analyzer) cycleNotes() {
	info := a.pkg.Info
	for _, f := range a.pkg.Files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				obj := info.Defs[ts.Name].(*types.TypeName)
				st, ok := obj.Type().Underlying().(*types.Struct)
				if !ok {
					continue
				}
				for i := 0; i < st.NumFields(); i++ {
					fld := st.Field(i)
					if !heapHolder(fld.Type()) {
						continue
					}
					if c := reaches(fld.Type(), obj.Type(), map[types.Type]bool{}); c != nil {
						name := types.TypeString(c, types.RelativeTo(obj.Pkg()))
						a.fix(fld.Pos(), fmt.Sprintf("%s.%s can point back to %s %s: a cycle through it is not reclaimed (RFC 0013)", obj.Name(), fld.Name(), article(name), name))
					}
				}
			}
		}
	}
}

// heapHolder reports whether a field of type t holds heap pointers: a heap
// pointer, or a slice of them.
func heapHolder(t types.Type) bool {
	if s, ok := t.Underlying().(*types.Slice); ok {
		t = s.Elem()
	}
	return heapPtrType(t)
}

// reaches returns the type of a cell that a value of type t can lead to,
// through heap pointers, slices and struct fields, and that holds a value
// of type target (is one, or has one in a field); or nil.
func reaches(t, target types.Type, seen map[types.Type]bool) types.Type {
	if seen[t] {
		return nil
	}
	seen[t] = true
	switch u := t.Underlying().(type) {
	case *types.Pointer: // a heap pointer: one in a field or a slice escapes
		if holds(u.Elem(), target) {
			return u.Elem()
		}
		return reaches(u.Elem(), target, seen)
	case *types.Slice:
		return reaches(u.Elem(), target, seen)
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if c := reaches(u.Field(i).Type(), target, seen); c != nil {
				return c
			}
		}
	}
	return nil
}

// holds reports whether a value of type t is, or holds in a field (not
// through a pointer), a value of type target.
func holds(t, target types.Type) bool {
	if types.Identical(t, target) {
		return true
	}
	if st, ok := t.Underlying().(*types.Struct); ok {
		for i := 0; i < st.NumFields(); i++ {
			if holds(st.Field(i).Type(), target) {
				return true
			}
		}
	}
	return false
}

// article is "an" before a vowel, else "a".
func article(name string) string {
	if strings.ContainsRune("AEIOUaeiou", rune(name[0])) {
		return "an"
	}
	return "a"
}
