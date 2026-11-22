package borrow

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

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
					unsupported(n.X, "ranging over a slice held through a pointer that escapes")
				}
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
