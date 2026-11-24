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
	o := &ownership{p: p, info: p.pkg.Info, why: map[types.Type]sharing{}, boxed: Ifaces(p.pkg).Boxed, chained: map[types.Type]bool{}}
	o.run()
	for _, h := range p.heap {
		if s, ok := o.whyOf(h); ok {
			p.shared = append(p.shared, s)
		} else {
			p.owned = append(p.owned, h)
		}
	}
	for _, e := range o.errs {
		if _, shared := o.whyOf(e.t); !shared {
			p.ownerErrs = append(p.ownerErrs, e)
		}
	}
	for t := range o.chained {
		p.chained = append(p.chained, t)
	}
}

// Chained reports whether owned pointers to t are moved into fields
// (RFC 0017, step 4), so that a chain of t's may be built in a loop and
// be longer than any call depth: freeing it must not recurse per link.
func (p *PtrPlan) Chained(t types.Type) bool {
	p.ownedOnce()
	for _, c := range p.chained {
		if types.Identical(c, t) {
			return true
		}
	}
	return false
}

// ownerErrors returns the uses of owned pointers that one owner per
// object refuses (RFC 0017), each with its position, message and notes.
func (p *PtrPlan) ownerErrors() []ownerError {
	p.ownedOnce()
	return p.ownerErrs
}

// ownerError is a use of an owned pointer that one owner per object
// refuses (RFC 0017).
type ownerError struct {
	t     types.Type
	pos   token.Pos
	msg   string
	notes []string
}

// ownership finds the heap types that are shared.
type ownership struct {
	errs []ownerError
	// kept are the parameters their function keeps: returns, stores or
	// moves (RFC 0017, step 8). The caller moves its pointer in.
	kept map[*types.Var]bool
	// chained are the element types a pointer is moved into a field of
	// (RFC 0017, step 4): their chains may be longer than any call depth.
	chained map[types.Type]bool
	p       *PtrPlan
	info    *types.Info
	why     map[types.Type]sharing // by the type's element, the first reason
	// params are the parameters, receivers and results of the program's
	// functions; inputs only the parameters and receivers, which are lent.
	params, inputs map[*types.Var]bool
	// boxed are the expressions converted to an interface (RFC 0012),
	// found once: finding them walks the whole program.
	boxed map[ast.Expr]types.Type
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

// refuse records an error at e for owned element type t: a use one owner
// per object cannot express (RFC 0017). It is reported only if t stays
// owned; a shared type keeps today's cells, which express it.
func (o *ownership) refuse(t types.Type, e ast.Expr, msg string, notes ...string) {
	o.errs = append(o.errs, ownerError{t: t, pos: e.Pos(), msg: msg, notes: notes})
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
			case *types.Slice:
				if o.ptrSlice(ft) {
					continue // a slice of them, a Vec owning each (RFC 0017, step 7)
				}
			}
			o.share(h, f.Pos(), fmt.Sprintf("its field %s is of type %s", f.Name(), types.TypeString(f.Type(), types.RelativeTo(o.p.pkg.Types))))
			break
		}
	}
	o.params, o.inputs = map[*types.Var]bool{}, map[*types.Var]bool{}
	for _, obj := range o.info.Defs {
		if fn, ok := obj.(*types.Func); ok {
			sig := fn.Signature()
			if r := sig.Recv(); r != nil {
				o.params[r] = true
				o.inputs[r] = true
			}
			for i := 0; i < sig.Params().Len(); i++ {
				o.inputs[sig.Params().At(i)] = true
			}
			for _, tup := range []*types.Tuple{sig.Params(), sig.Results()} {
				for i := 0; i < tup.Len(); i++ {
					o.params[tup.At(i)] = true
				}
			}
		}
	}
	o.vars()
	o.keptParams()
	o.ranges()
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
		if v.IsField() && o.candidateField(v) && o.ptrSlice(t) {
			continue // owned by the field's Vec (RFC 0017, step 7)
		}
		o.heldIn(t, id.Pos(), map[types.Type]bool{})
	}
}

// ranges shares the type of the pointers a range over a slice of them
// copies into a variable that is not a temporary borrow: the element and
// the variable would both own the pointer (RFC 0017, step 7).
func (o *ownership) ranges() {
	for _, f := range o.p.pkg.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			r, ok := n.(*ast.RangeStmt)
			if !ok || !o.ptrSlice(o.info.TypeOf(r.X)) {
				return true
			}
			id, ok := r.Value.(*ast.Ident)
			if !ok || id.Name == "_" {
				return true
			}
			if v, ok := o.info.Defs[id].(*types.Var); ok && !o.p.BorrowVar(v) {
				o.share(o.elemOf(v.Type()), id.Pos(), "a range variable takes a copy of an element and keeps it")
			}
			return true
		})
	}
}

// borrowedBy reports whether l, assigned, is a local temporary borrow
// (RFC 0017, step 5): the value it is given is lent to it, not moved.
func (o *ownership) borrowedBy(l ast.Expr) bool {
	id, ok := ast.Unparen(l).(*ast.Ident)
	return ok && o.p.BorrowVar(o.localOf(id))
}

// keptVar reports whether e names a kept parameter.
func (o *ownership) keptVar(e ast.Expr) bool {
	v, _ := o.info.Uses[ast.Unparen(e).(*ast.Ident)].(*types.Var) // asked only of a local variable
	return o.kept[v]
}

// movable reports whether e, passed to a kept parameter, can be moved
// into it: a new pointer, a local variable or kept parameter (dead from
// there, which the borrow checker holds to), or a field the call's result
// is assigned back to, taken out for the call (t.Left = insert(t.Left, v)).
func (o *ownership) movable(e ast.Expr) bool {
	if o.fresh(e) || o.info.Types[e].IsNil() {
		return true
	}
	if o.localVar(e) {
		return !o.input(e) || o.keptVar(e)
	}
	_, taken := o.p.Take(e)
	return taken
}

// keptParams finds the parameters of owned candidate types that their
// function keeps: returns, moves into a variable, a field or a literal, or
// passes to a parameter kept in turn, to a fixed point over the calls.
func (o *ownership) keptParams() {
	o.kept = map[*types.Var]bool{}
	var decls []*ast.FuncDecl
	for _, f := range o.p.pkg.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
				decls = append(decls, fd)
			}
		}
	}
	keeps := func(e ast.Expr) *types.Var {
		id, ok := ast.Unparen(e).(*ast.Ident)
		if !ok {
			return nil
		}
		v, _ := o.info.Uses[id].(*types.Var)
		if !o.inputs[v] || o.elemOf(v.Type()) == nil {
			return nil
		}
		return v
	}
	for changed := true; changed; {
		changed = false
		mark := func(v *types.Var) {
			if v != nil && !o.kept[v] {
				o.kept[v] = true
				changed = true
			}
		}
		for _, fd := range decls {
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.ReturnStmt:
					for _, r := range x.Results {
						mark(keeps(r))
					}
				case *ast.AssignStmt:
					for i, r := range x.Rhs {
						if len(x.Lhs) == len(x.Rhs) && o.borrowedBy(x.Lhs[i]) {
							continue // lent to a temporary borrow (step 5), not kept
						}
						mark(keeps(r))
					}
				case *ast.KeyValueExpr:
					mark(keeps(x.Value))
				case *ast.CallExpr:
					if fn := o.programCallee(x); fn != nil {
						sig := fn.Signature()
						for i, a := range x.Args {
							if i < sig.Params().Len() && o.kept[sig.Params().At(i)] {
								mark(keeps(a))
							}
						}
					} else if id, ok := ast.Unparen(x.Fun).(*ast.Ident); ok && id.Name == "append" {
						for _, a := range x.Args[1:] {
							mark(keeps(a))
						}
					}
				}
				return true
			})
		}
	}
}

// ptrSlice reports whether t is a slice of heap pointers.
func (o *ownership) ptrSlice(t types.Type) bool {
	s, ok := types.Unalias(t).Underlying().(*types.Slice)
	return ok && o.elemOf(s.Elem()) != nil
}

// elemPlace reports whether e is an element of a slice of pointers held
// in a field: a place that owns its pointer (RFC 0017, step 7).
func (o *ownership) elemPlace(e ast.Expr) bool {
	ix, ok := ast.Unparen(e).(*ast.IndexExpr)
	return ok && o.ptrSlice(o.info.TypeOf(ix.X)) && o.fieldPlace(ix.X)
}

// appendedToField reports whether call c is append(x.f, ...) assigned
// back to x.f, a candidate's slice of pointers: its arguments move into
// the Vec.
func (o *ownership) appendedToField(c *ast.CallExpr, stack []ast.Node) bool {
	id, ok := ast.Unparen(c.Fun).(*ast.Ident)
	if !ok || len(c.Args) == 0 {
		return false
	}
	if b, ok := o.info.Uses[id].(*types.Builtin); !ok || b.Name() != "append" {
		return false
	}
	for i := len(stack) - 1; i >= 0; i-- {
		if as, ok := stack[i].(*ast.AssignStmt); ok {
			return len(as.Lhs) == 1 && len(as.Rhs) == 1 && ast.Unparen(as.Rhs[0]) == ast.Expr(c) &&
				o.fieldPlace(as.Lhs[0]) && o.ptrSlice(o.info.TypeOf(as.Lhs[0])) &&
				types.ExprString(ast.Unparen(as.Lhs[0])) == types.ExprString(ast.Unparen(c.Args[0]))
		}
	}
	return false
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
	if _, boxed := o.boxed[e]; boxed {
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
	case *ast.IndexExpr:
		if o.elemPlace(e) {
			o.placeUse(el, e, stack) // an element of a field's Vec (RFC 0017, step 7)
			return
		}
		o.share(el, e.Pos(), "a pointer is computed in a way that may share it")
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
				if r == child && len(p.Lhs) == len(p.Rhs) && (o.localVar(p.Lhs[i]) || o.fieldPlace(p.Lhs[i])) {
					if o.fieldPlace(p.Lhs[i]) {
						o.chained[el] = true
					}
					return // a parameter assigned is refused where it is assigned; a field gets it (RFC 0017, step 4)
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
	case *ast.CallExpr:
		if p.Args[0] != child && o.appendedToField(p, stack[:at+1]) {
			o.chained[el] = true
			return // appended to a field's Vec, which owns it (RFC 0017, step 7)
		}
		if o.readOrLend(p, child, stack[:at+1]) {
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
	if _, ok := par.(*ast.ReturnStmt); ok && o.localVar(e) && (!o.param(e) || o.keptVar(e)) {
		return // returning a local variable, or a kept parameter, moves it
	}
	if o.lentToBorrow(par, child) {
		return // a temporary borrow takes it for a while (RFC 0017, step 5)
	}
	if _, taken := o.p.Take(e); taken {
		return // moved out of its field, which is refilled at once (step 6)
	}
	if as, ok := par.(*ast.AssignStmt); ok && as.Tok == token.ASSIGN {
		for _, l := range as.Lhs {
			if l == child {
				if o.input(l) {
					o.refuse(el, l, fmt.Sprintf("parameter %s is assigned another pointer", ast.Unparen(l).(*ast.Ident).Name),
						"a parameter lends the caller's object for the call, so in Rust it holds a borrow, which a new object cannot replace (RFC 0017)",
						"assign the new pointer to a new local variable instead, and use that from here on")
					return
				}
				if o.localVar(l) || o.fieldPlace(l) {
					return // a variable or a field assigned a pointer, which moves there (RFC 0017, step 4)
				}
				o.share(el, e.Pos(), "it is written through")
				return
			}
		}
	}
	if sel, ok := par.(*ast.SelectorExpr); ok && sel.X == child {
		if s := o.info.Selections[sel]; s != nil && s.Kind() == types.FieldVal && o.written(stack[:at+1]) {
			if o.plainWrite(stack[:at+1]) {
				return // a number, boolean or string changed in place (RFC 0017, step 3)
			}
			o.share(el, e.Pos(), "it is written through")
			return
		}
	}
	if o.readOrLend(par, child, stack[:at+1]) {
		return
	}
	if o.moved(e, par, child, stack[:at+1]) {
		return // moved to a new owner; the borrow checker refuses a later use (RFC 0017, step 4)
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
						if i < params.Len() && o.kept[params.At(i)] {
							return o.movable(a.(ast.Expr)) // moved in (RFC 0017, step 8)
						}
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

// lentToBorrow reports whether child is the value par assigns to, or
// declares, a temporary borrow (PtrPlan.BorrowVar): it is lent, not moved.
func (o *ownership) lentToBorrow(par, child ast.Node) bool {
	switch p := par.(type) {
	case *ast.AssignStmt:
		for i, r := range p.Rhs {
			if r == child && len(p.Lhs) == len(p.Rhs) {
				id, ok := ast.Unparen(p.Lhs[i]).(*ast.Ident)
				return ok && o.p.BorrowVar(o.localOf(id))
			}
		}
	case *ast.ValueSpec:
		for i, r := range p.Values {
			if r == child && len(p.Values) == len(p.Names) {
				return o.p.BorrowVar(o.localOf(p.Names[i]))
			}
		}
	}
	return false
}

// localOf returns the variable id defines or names.
func (o *ownership) localOf(id *ast.Ident) *types.Var {
	obj := o.info.Defs[id]
	if obj == nil {
		obj = o.info.Uses[id]
	}
	v, _ := obj.(*types.Var)
	return v
}

// fieldPlace reports whether e is a field reached from a variable through
// owned pointers and struct fields: a place that can own a pointer.
func (o *ownership) fieldPlace(e ast.Expr) bool {
	sel, ok := ast.Unparen(e).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return rootVar(o.info, sel) != nil
}

// moved reports whether e, a local variable that is not a parameter,
// moves to a new owner at par: a variable or a field assigned it, a
// variable declared with it, or a field of a new pointer's literal.
func (o *ownership) moved(e ast.Expr, par, child ast.Node, stack []ast.Node) bool {
	if !o.localVar(e) || o.input(e) && !o.keptVar(e) {
		return false
	}
	el := o.elemOf(o.info.TypeOf(e))
	switch p := par.(type) {
	case *ast.AssignStmt:
		for i, r := range p.Rhs {
			if r == child && len(p.Lhs) == len(p.Rhs) {
				if o.fieldPlace(p.Lhs[i]) {
					o.chained[el] = true
					return true
				}
				return o.localVar(p.Lhs[i]) && !o.input(p.Lhs[i])
			}
		}
	case *ast.ValueSpec:
		return true // a local variable declared with it
	case *ast.CallExpr:
		if p.Args[0] != child && o.appendedToField(p, stack) {
			o.chained[el] = true
			return true // appended to a field's Vec (RFC 0017, step 7)
		}
	case *ast.KeyValueExpr:
		o.chained[el] = true
		return p.Value == child && o.boxedLit(stack[:len(stack)-1])
	case *ast.CompositeLit:
		o.chained[el] = true
		return o.boxedLit(stack)
	}
	return false
}

// rootVar returns the variable that place e starts from, through field
// selections, or nil.
func rootVar(info *types.Info, e ast.Expr) *types.Var {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.Ident:
			v, _ := info.Uses[x].(*types.Var)
			return v
		case *ast.SelectorExpr:
			e = x.X // a field: a place assigned a pointer is never a method
		case *ast.IndexExpr:
			e = x.X // an element of a slice (RFC 0017, step 7)
		default:
			return nil // a field of a call's result
		}
	}
}

// plainWrite reports whether the place written at the top of stack (see
// written) is assigned or incremented, not lent by &, and holds a number,
// a boolean or a string: a change in place, through a &mut borrow in
// Rust, that gives no value a second name.
func (o *ownership) plainWrite(stack []ast.Node) bool {
	place := stack[len(stack)-1]
	for j := len(stack) - 2; ; j-- {
		switch p := stack[j].(type) {
		case *ast.ParenExpr:
			place = p
			continue
		case *ast.SelectorExpr:
			if p.X == place {
				place = p
				continue
			}
		case *ast.IndexExpr:
			if p.X == place && o.ptrSlice(o.info.TypeOf(p.X)) {
				place = p // an element of a Vec of owned pointers, changed through (step 7)
				continue
			}
		case *ast.AssignStmt, *ast.IncDecStmt:
			t := o.info.TypeOf(place.(ast.Expr))
			_, basic := t.Underlying().(*types.Basic)
			if as, ok := p.(*ast.AssignStmt); ok && o.ptrSlice(t) && len(as.Rhs) == 1 {
				// x.f = append(x.f, ...): pushed onto the field's own Vec
				// (RFC 0017, step 7).
				c, isCall := ast.Unparen(as.Rhs[0]).(*ast.CallExpr)
				return isCall && o.appendedToField(c, append(stack[:j+1:j+1], c))
			}
			// A pointer field assigned: the value moves there, and is
			// checked where it is (RFC 0017, step 4).
			return basic || o.elemOf(t) != nil
		}
		return false // &p.f, or p.s[i] = x
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

// input reports whether e names a parameter or receiver: lent, so it is
// never assigned another pointer (that would be a Box where a borrow is
// held).
func (o *ownership) input(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false // a field
	}
	v, _ := o.info.Uses[id].(*types.Var)
	return o.inputs[v]
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
