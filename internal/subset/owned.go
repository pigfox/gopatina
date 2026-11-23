package subset

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
)

// Owned pointers (RFC 0016, step 1). A heap pointer type *T is owned when
// no *T of the program is ever shared: each one is made fresh (&T{...},
// new(T), or a function's result, which is fresh in turn), moved once into
// the variable, field or result that owns it, and otherwise only read
// through, tested for nil or lent to a function, which reads through it
// in turn. Its pointee is then never written after it is made, and never
// reached but through its owner, so a Box is exact: the shape is a tree
// built and then read, such as binary-trees'.
//
// This first version admits no write through an owned pointer and no
// moves out of a place; a program that needs either keeps *T a shared
// heap cell, exactly as before. Writes, and moves out of a place, are the
// next steps.

// Owned reports whether t, a heap pointer type, is owned.
func (p *PtrPlan) Owned(t types.Type) bool {
	pt, ok := types.Unalias(t).(*types.Pointer)
	if !ok || !p.HeapType(t) {
		return false
	}
	p.ownedOnce()
	for _, o := range p.owned {
		if types.Identical(o, pt.Elem()) {
			return true
		}
	}
	return false
}

// SharedReason returns why the heap pointer type t is not owned, and
// where: the first use that shares it. It returns false for an owned or a
// non-heap type.
func (p *PtrPlan) SharedReason(t types.Type) (token.Pos, string, bool) {
	pt, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return token.NoPos, "", false
	}
	for _, s := range p.shared {
		if types.Identical(s.t, pt.Elem()) {
			return s.pos, s.why, true
		}
	}
	return token.NoPos, "", false // owned, or not a heap type
}

// sharing is why a heap pointer type's element t is shared.
type sharing struct {
	t   types.Type
	pos token.Pos
	why string
}

// ownedOnce computes the owned types the first time they are asked for.
func (p *PtrPlan) ownedOnce() {
	if p.ownedDone {
		return
	}
	p.ownedDone = true
	o := &ownership{p: p, info: p.pkg.Info, why: map[types.Type]sharing{}}
	o.run()
	for _, h := range p.heap {
		if s, ok := o.whyOf(h); ok {
			p.shared = append(p.shared, s)
		} else {
			p.owned = append(p.owned, h)
		}
	}
}

// ownership finds the heap types that are shared.
type ownership struct {
	p    *PtrPlan
	info *types.Info
	why  map[types.Type]sharing // by the type's element, the first reason
	// params are the parameters and receivers of the program's functions.
	params map[*types.Var]bool
}

// whyOf returns the reason element type t is shared, if it is.
func (o *ownership) whyOf(t types.Type) (sharing, bool) {
	for k, s := range o.why {
		if types.Identical(k, t) {
			return s, true
		}
	}
	return sharing{}, false
}

// share records that element type t is shared, at pos, because of why.
func (o *ownership) share(t types.Type, pos token.Pos, why string) {
	for k, s := range o.why {
		if types.Identical(k, t) {
			if pos.IsValid() && s.pos.IsValid() && pos < s.pos {
				o.why[k] = sharing{t: k, pos: pos, why: why}
			}
			return
		}
	}
	o.why[t] = sharing{t: t, pos: pos, why: why}
}

// elemOf returns the element type T of e's type if it is a heap pointer
// type *T, or nil.
func (o *ownership) elemOf(t types.Type) types.Type {
	if t == nil || !o.p.HeapType(t) {
		return nil
	}
	return types.Unalias(t).(*types.Pointer).Elem()
}

func (o *ownership) run() {
	// A heap pointer type is a candidate only if its element is a struct
	// of the program whose fields are numbers, booleans, strings or heap
	// pointers: its value is never needed outside its box.
	for _, h := range o.p.heap {
		named, ok := types.Unalias(h).(*types.Named)
		st, isStruct := h.Underlying().(*types.Struct)
		if !ok || !isStruct || named.Obj().Pkg() != o.p.pkg.Types {
			o.share(h, token.NoPos, "its element is not a struct type of the program")
			continue
		}
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			switch ft := types.Unalias(f.Type()).Underlying().(type) {
			case *types.Basic:
				continue
			case *types.Pointer:
				if o.p.HeapType(ft) {
					continue
				}
			}
			o.share(h, f.Pos(), fmt.Sprintf("its field %s is of type %s", f.Name(), types.TypeString(f.Type(), types.RelativeTo(o.p.pkg.Types))))
			break
		}
	}
	o.params = map[*types.Var]bool{}
	for _, obj := range o.info.Defs {
		if fn, ok := obj.(*types.Func); ok {
			sig := fn.Signature()
			if r := sig.Recv(); r != nil {
				o.params[r] = true
			}
			for _, tup := range []*types.Tuple{sig.Params(), sig.Results()} {
				for i := 0; i < tup.Len(); i++ {
					o.params[tup.At(i)] = true
				}
			}
		}
	}
	o.vars()
	for _, f := range o.p.pkg.Files {
		var stack []ast.Node
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			if e, ok := n.(ast.Expr); ok {
				o.expr(e, stack)
			}
			return true
		})
	}
	// A shared type holding a pointer of another type shares that one too:
	// its cell would hold the box.
	for changed := true; changed; {
		changed = false
		for _, h := range o.p.heap {
			if _, ok := o.whyOf(h); !ok {
				continue
			}
			st, ok := h.Underlying().(*types.Struct)
			if !ok {
				continue
			}
			for i := 0; i < st.NumFields(); i++ {
				f := st.Field(i)
				if el := o.elemOf(f.Type()); el != nil {
					if _, ok := o.whyOf(el); !ok {
						o.share(el, f.Pos(), "a field of a shared type holds it")
						changed = true
					}
				}
			}
		}
	}
}

// vars shares the types whose pointers or values are held in a variable
// that is not a local variable, a parameter, a result or a field of a
// candidate: a package-level variable, a slice, map or array of them, a
// struct value that is not a candidate's.
func (o *ownership) vars() {
	for id, obj := range o.info.Defs {
		v, ok := obj.(*types.Var)
		if !ok {
			continue
		}
		t := v.Type()
		if el := o.elemOf(t); el != nil {
			switch {
			case v.Parent() == v.Pkg().Scope():
				o.share(el, id.Pos(), "a package-level variable holds it")
			case v.IsField() && !o.candidateField(v):
				o.share(el, id.Pos(), "a field of a struct that is not boxed holds it")
			}
			continue
		}
		o.heldIn(t, id.Pos(), map[types.Type]bool{})
	}
}

// candidateField reports whether field v belongs to the element type of a
// heap pointer type.
func (o *ownership) candidateField(v *types.Var) bool {
	for _, h := range o.p.heap {
		if st, ok := h.Underlying().(*types.Struct); ok {
			for i := 0; i < st.NumFields(); i++ {
				if st.Field(i) == v {
					return true
				}
			}
		}
	}
	return false
}

// heldIn shares every heap pointer type, and every candidate element
// type, that a value of type t holds other than as itself: in a slice, a
// map, an array, a channel or a struct value.
func (o *ownership) heldIn(t types.Type, pos token.Pos, seen map[types.Type]bool) {
	if seen[t] {
		return
	}
	seen[t] = true
	if el := o.elemOf(t); el != nil {
		o.share(el, pos, "a slice, map, array, channel or struct value holds it")
		return
	}
	for _, h := range o.p.heap {
		if types.Identical(h, t) {
			o.share(h, pos, "a value of the type itself is held outside its box")
			return
		}
	}
	switch u := t.Underlying().(type) {
	case *types.Slice:
		o.heldIn(u.Elem(), pos, seen)
	case *types.Array:
		o.heldIn(u.Elem(), pos, seen)
	case *types.Map:
		o.heldIn(u.Key(), pos, seen)
		o.heldIn(u.Elem(), pos, seen)
	case *types.Chan:
		o.heldIn(u.Elem(), pos, seen)
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			o.heldIn(u.Field(i).Type(), pos, seen)
		}
	case *types.Signature:
		// A function type: values pass through its calls, which are checked
		// where they are written.
	}
}

// parent returns the node above stack's top, skipping parentheses, the
// child under it, and the parent's index in stack. Stack's top is an
// expression, so it has a parent: at least the file.
func parent(stack []ast.Node) (ast.Node, ast.Node, int) {
	child := stack[len(stack)-1]
	i := len(stack) - 2
	for {
		if _, ok := stack[i].(*ast.ParenExpr); !ok {
			return stack[i], child, i
		}
		child = stack[i]
		i--
	}
}

// expr checks one expression, the top of stack.
func (o *ownership) expr(e ast.Expr, stack []ast.Node) {
	if _, ok := e.(*ast.ParenExpr); ok {
		return
	}
	if o.info.Types[e].IsType() {
		return // a type, not a value
	}
	switch p := stack[len(stack)-2].(type) {
	case *ast.KeyValueExpr:
		if p.Key == e {
			return // a field name in a composite literal
		}
	case *ast.SelectorExpr:
		if p.Sel == e {
			return // the name selected, checked as the selector
		}
	}
	if id, ok := e.(*ast.Ident); ok {
		if _, ok := o.info.Defs[id].(*types.TypeName); ok {
			return
		}
	}
	t := o.info.TypeOf(e)
	if t == nil {
		return
	}
	// A value of a candidate's element type outside its box: *p, or a
	// composite literal not under &.
	for _, h := range o.p.heap {
		if types.Identical(h, t) {
			par, child, _ := parent(stack)
			if _, lit := e.(*ast.CompositeLit); lit {
				if u, ok := par.(*ast.UnaryExpr); ok && u.Op == token.AND && u.X == child {
					return
				}
			}
			o.share(h, e.Pos(), "a value of the type itself is used outside its box")
			return
		}
	}
	el := o.elemOf(t)
	if el == nil {
		return
	}
	if _, boxed := Ifaces(o.p.pkg).Boxed[e]; boxed {
		o.share(el, e.Pos(), "an interface value holds it")
		return
	}
	if o.fresh(e) {
		o.freshUse(el, e, stack)
		return
	}
	switch e.(type) {
	case *ast.Ident, *ast.SelectorExpr:
		if id, ok := e.(*ast.Ident); ok {
			if _, def := o.info.Defs[id]; def {
				return // a declaration, not a use
			}
		}
		if sel, ok := e.(*ast.SelectorExpr); ok {
			if s := o.info.Selections[sel]; s == nil || s.Kind() != types.FieldVal {
				return // a method or a qualified name, not a place
			}
		}
		o.placeUse(el, e, stack)
	default:
		o.share(el, e.Pos(), "a pointer is computed in a way that may share it")
	}
}

// fresh reports whether e makes a new pointer: &T{...}, new(T) or a call
// of a program function.
func (o *ownership) fresh(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.UnaryExpr:
		_, lit := ast.Unparen(x.X).(*ast.CompositeLit)
		return x.Op == token.AND && lit
	case *ast.CallExpr:
		if id, ok := ast.Unparen(x.Fun).(*ast.Ident); ok {
			if b, ok := o.info.Uses[id].(*types.Builtin); ok {
				return b.Name() == "new"
			}
		}
		return o.programCallee(x) != nil
	}
	return false
}

// programCallee returns the function or method of the program that c
// calls statically, or nil.
func (o *ownership) programCallee(c *ast.CallExpr) *types.Func {
	var fn *types.Func
	switch f := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		fn, _ = o.info.Uses[f].(*types.Func)
	case *ast.SelectorExpr:
		fn, _ = o.info.Uses[f.Sel].(*types.Func)
	}
	if fn == nil || fn.Pkg() != o.p.pkg.Types {
		return nil
	}
	return fn
}

// freshUse checks where a new pointer goes: returned, declared into a
// local variable, assigned to a local variable, a field of a composite
// literal under &, read through or lent to a call.
func (o *ownership) freshUse(el types.Type, e ast.Expr, stack []ast.Node) {
	par, child, at := parent(stack)
	switch p := par.(type) {
	case *ast.ReturnStmt, *ast.ExprStmt:
		return
	case *ast.ValueSpec:
		return // a local variable: package-level ones are shared in vars
	case *ast.AssignStmt:
		if p.Tok == token.DEFINE || p.Tok == token.ASSIGN {
			for i, r := range p.Rhs {
				if r == child && len(p.Lhs) == len(p.Rhs) && o.localVar(p.Lhs[i]) {
					return
				}
			}
		}
	case *ast.KeyValueExpr:
		if p.Value == child && o.boxedLit(stack[:len(stack)-1]) {
			return
		}
	case *ast.CompositeLit:
		if o.boxedLit(stack[:len(stack)-1]) {
			return
		}
	default:
		if o.readOrLend(p, child, stack[:at+1]) {
			return
		}
	}
	o.share(el, e.Pos(), "a new pointer is stored where it may be shared")
}

// placeUse checks a use of a pointer held in a variable or a field: only
// read through, tested for nil or lent to a call.
func (o *ownership) placeUse(el types.Type, e ast.Expr, stack []ast.Node) {
	if id, ok := e.(*ast.Ident); ok {
		if v, ok := o.info.Uses[id].(*types.Var); ok && o.captured(v, id, stack) {
			o.share(el, e.Pos(), "a function literal captures it")
			return
		}
	}
	par, child, at := parent(stack)
	if _, ok := par.(*ast.ReturnStmt); ok && o.localVar(e) && !o.param(e) {
		return // returning a local variable moves it: nothing can use it after
	}
	if as, ok := par.(*ast.AssignStmt); ok && as.Tok == token.ASSIGN {
		for _, l := range as.Lhs {
			if l == child {
				if o.localVar(l) {
					return // a local variable assigned a new pointer, checked there
				}
				o.share(el, e.Pos(), "it is written through")
				return
			}
		}
	}
	if sel, ok := par.(*ast.SelectorExpr); ok && sel.X == child {
		if s := o.info.Selections[sel]; s != nil && s.Kind() == types.FieldVal && o.written(stack[:at+1]) {
			o.share(el, e.Pos(), "it is written through")
			return
		}
	}
	if o.readOrLend(par, child, stack[:at+1]) {
		return
	}
	if b, ok := par.(*ast.BinaryExpr); ok && (b.Op == token.EQL || b.Op == token.NEQ) {
		other := b.X
		if other == child {
			other = b.Y
		}
		if o.isNil(other) {
			return
		}
		o.share(el, e.Pos(), "it is compared with another pointer")
		return
	}
	o.share(el, e.Pos(), "a copy of it is kept")
}

// readOrLend reports whether child, a pointer, is read through by par (a
// field read or a method call with a pointer receiver), or lent to a
// call of a program function, neither of which keeps it.
func (o *ownership) readOrLend(par, child ast.Node, stack []ast.Node) bool {
	switch p := par.(type) {
	case *ast.SelectorExpr:
		// child is p.X: the name selected is skipped in expr, and a
		// pointer's selector is a field or a method.
		s := o.info.Selections[p]
		switch s.Kind() {
		case types.FieldVal:
			return !o.written(stack)
		case types.MethodVal:
			above, fun, _ := parent(stack)
			call, ok := above.(*ast.CallExpr)
			if !ok || call.Fun != fun {
				return false // a method value keeps its receiver
			}
			recv := s.Obj().(*types.Func).Signature().Recv()
			_, ptrRecv := types.Unalias(recv.Type()).(*types.Pointer)
			return ptrRecv && o.programCallee(call) != nil && !o.deferred(stack)
		}
	case *ast.CallExpr:
		for _, a := range p.Args {
			if a == child {
				fn := o.programCallee(p)
				if fn == nil || o.deferred(stack) {
					return false
				}
				params := fn.Signature().Params()
				for i, b := range p.Args {
					if b == a {
						return i < params.Len() && !fn.Signature().Variadic() && types.Identical(params.At(i).Type(), o.info.TypeOf(a.(ast.Expr)))
					}
				}
			}
		}
	}
	return false
}

// written reports whether the field selected at the top of stack is
// written: assigned, incremented, or has its address taken, directly or as
// the base of a longer place.
func (o *ownership) written(stack []ast.Node) bool {
	place := stack[len(stack)-1]
	for j := len(stack) - 2; ; j-- {
		switch p := stack[j].(type) {
		case *ast.ParenExpr:
			place = p
			continue
		case *ast.SelectorExpr:
			if s := o.info.Selections[p]; p.X == place && s != nil && s.Kind() == types.FieldVal {
				place = p
				continue
			}
		case *ast.IndexExpr:
			if p.X == place {
				place = p
				continue
			}
		case *ast.AssignStmt:
			for _, l := range p.Lhs {
				if l == place {
					return true
				}
			}
		case *ast.IncDecStmt:
			return p.X == place
		case *ast.UnaryExpr:
			return p.Op == token.AND && p.X == place
		}
		return false
	}
}

// deferred reports whether the innermost call on stack is in a go or
// defer statement, which would lend its arguments beyond the statement.
func (o *ownership) deferred(stack []ast.Node) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i].(type) {
		case *ast.GoStmt, *ast.DeferStmt:
			return true
		case ast.Stmt:
			return false
		}
	}
	return false
}

// localVar reports whether e names a local variable (not a field or a
// package-level variable).
func (o *ownership) localVar(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	obj := o.info.Defs[id]
	if obj == nil {
		obj = o.info.Uses[id]
	}
	v, ok := obj.(*types.Var)
	return ok && !v.IsField() && v.Parent() != v.Pkg().Scope()
}

// boxedLit reports whether the composite literal on top of stack (or the
// one holding the key-value pair there) is under &: the fields of a new
// box.
func (o *ownership) boxedLit(stack []ast.Node) bool {
	for i := len(stack) - 1; ; i-- {
		if _, ok := stack[i].(*ast.CompositeLit); ok {
			par, child, _ := parent(stack[:i+1])
			u, ok := par.(*ast.UnaryExpr)
			return ok && u.Op == token.AND && u.X == child
		}
	}
}

// captured reports whether the use id of v is inside a function literal
// that v is declared outside of.
func (o *ownership) captured(v *types.Var, id *ast.Ident, stack []ast.Node) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		if lit, ok := stack[i].(*ast.FuncLit); ok {
			return v.Pos() < lit.Pos() || v.Pos() > lit.End()
		}
	}
	return false
}

// param reports whether e names a parameter, receiver or named result of
// the function it is in: a parameter is lent, so it is never moved.
func (o *ownership) param(e ast.Expr) bool {
	v, _ := o.info.Uses[ast.Unparen(e).(*ast.Ident)].(*types.Var)
	return o.params[v]
}

// isNil reports whether e is the predeclared nil.
func (o *ownership) isNil(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	_, isNil := o.info.Uses[id].(*types.Nil)
	return isNil
}
