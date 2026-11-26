package borrow

import (
	"fmt"
	"go/ast"
	"go/types"
	"sort"

	"github.com/pigfox/gopatina/internal/diag"
	"github.com/pigfox/gopatina/internal/subset"
)

// Places through pointers that escape (RFC 0013). Each such pointer has
// one owner (RFC 0017), so a place through it is a plain Rust place. What
// is reported here: a condition that reads through one and calls a
// function of the program (Go makes the call first), and a value method
// that would run on a copy not sharing the value's elements, called
// through one or through an interface.

// HeapExact reports whether a copy of a value of type t is Go's copy: it
// shares nothing that can change.
// (A struct cannot hold itself but through a pointer, so this ends.)
func HeapExact(t types.Type) bool {
	switch u := t.Underlying().(type) {
	case *types.Basic, *types.Interface:
		return true
	case *types.Pointer:
		return KindOf(t) != PointerKind
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

func heapPtrType(t types.Type) bool {
	_, ok := t.(*types.Pointer)
	return ok && KindOf(t) != PointerKind
}

// checkHeap reports the uses of places through heap pointers that patina
// mode cannot write exactly.
func (a *analyzer) checkHeap(decls []*ast.FuncDecl) {
	if !subset.Ptrs(a.pkg).Heap() {
		return
	}
	unsupported := func(n ast.Node, what string) {
		d := diag.Errorf(a.pkg.Fset.Position(n.Pos()), "%s is not supported in patina mode yet", what)
		a.report(n.Pos(), d.With("see docs/compat.md for what patina mode supports"))
	}
	for _, fd := range decls { // every one has a body (the subset checker)
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.IfStmt:
				a.heapCondition(n.Cond, unsupported)
			case *ast.ForStmt:
				a.heapCondition(n.Cond, unsupported)
			case *ast.SwitchStmt:
				a.heapCondition(n.Tag, unsupported)
			case *ast.CallExpr:
				a.valueMethodCopy(n, unsupported)
			}
			return true
		})
	}
	a.heapDynamic(unsupported)
}

// valueMethodCopy reports p.M() for a value method that changes its copy,
// of a type holding a slice, called through a pointer that escapes: Go's
// copy shares the slice's elements, and a Rust clone would not.
func (a *analyzer) valueMethodCopy(c *ast.CallExpr, unsupported func(ast.Node, string)) {
	info := a.pkg.Info
	f := a.funcs[calleeFunc(info, c)]
	sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr)
	if f == nil || f.Recv == nil || !ok || isPointerT(f.Recv.Var.Type()) || f.Recv.Mode == Ref || f.Recv.Mode == RefMut {
		return
	}
	x := ast.Unparen(sel.X)
	if t := info.TypeOf(x); heapPtrType(t) && !HeapExact(t.(*types.Pointer).Elem()) {
		unsupported(x, "calling a method that changes its copy of a value held through a pointer that escapes")
	}
}

// heapDynamic reports a pointer converted to an interface whose value
// methods would run on a copy that does not share the value's elements: a
// call through the interface copies *p out of its box.
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

// heapCondition reports a condition that reads through a heap pointer and
// calls a function of the program: Go makes the call first, which patina
// mode writes for simple statements only.
func (a *analyzer) heapCondition(cond ast.Expr, unsupported func(ast.Node, string)) {
	if cond == nil || len(subset.HoistableCalls(a.pkg, cond)) == 0 {
		return
	}
	info := a.pkg.Info
	found := false
	ast.Inspect(cond, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.StarExpr:
			found = heapPtrType(info.TypeOf(x.X))
		case *ast.SelectorExpr:
			s := info.Selections[x]
			found = s != nil && s.Kind() == types.FieldVal && heapPtrType(info.TypeOf(x.X))
		}
		return !found
	})
	if found {
		unsupported(cond, "a condition that reads through a pointer that escapes and calls a function of the program")
	}
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

// cycleNotes notes, for -explain, that the pointers of each struct type
// of the program that escape have one owner (RFC 0016, RFC 0017): no
// cycle can be built of them.
func (a *analyzer) cycleNotes() {
	info := a.pkg.Info
	plan := subset.Ptrs(a.pkg)
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
				if _, ok := obj.Type().Underlying().(*types.Struct); ok && plan.HeapType(types.NewPointer(obj.Type())) {
					a.fix(ts.Name.Pos(), fmt.Sprintf("pointers of type *%s are never shared: each has one owner (RFC 0016)", obj.Name()))
				}
			}
		}
	}
}
