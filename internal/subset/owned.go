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
	o := &ownership{p: p, info: p.pkg.Info, why: map[types.Type]sharing{}, ifaced: map[types.Type]bool{}, boxed: Ifaces(p.pkg).Boxed, chained: map[types.Type]bool{}}
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
	ifaced := o.ifacedTypes()
	for _, e := range o.aliases {
		if !ifaced[e.t] {
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
	// aliases are the uses that give an object a second owner: refused
	// (RFC 0017) unless an interface holds the type's pointers, which keep
	// their cells until interface values are owned too.
	aliases []ownerError
	// ifaced are the element types an interface value holds a pointer to.
	ifaced map[types.Type]bool
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

// alias records that element type t is shared at e because a use gives
// its object a second owner, which one owner per object refuses (RFC
// 0017): why says so for SharedReason, msg and notes for the error.
func (o *ownership) alias(t types.Type, e ast.Node, why, msg string, notes ...string) {
	o.share(t, e.Pos(), why)
	o.aliases = append(o.aliases, ownerError{t: t, pos: e.Pos(), msg: msg, notes: notes})
}

// ifacedTypes returns the element types whose pointers stay cells because
// an interface holds them: those an interface value holds, and those a
// field of one holds, through any number of fields.
func (o *ownership) ifacedTypes() map[types.Type]bool {
	set := map[types.Type]bool{}
	for t := range o.ifaced {
		set[t] = true
	}
	for changed := true; changed; {
		changed = false
		for _, h := range o.p.heap {
			if !set[h] {
				continue
			}
			st, ok := h.Underlying().(*types.Struct)
			if !ok {
				continue
			}
			for i := 0; i < st.NumFields(); i++ {
				if el := o.elemOf(st.Field(i).Type()); el != nil && !set[el] {
					set[el] = true
					changed = true
				}
			}
		}
	}
	return set
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
// type *T, or nil. It is the heap's own T, so identical types are one map
// key.
func (o *ownership) elemOf(t types.Type) types.Type {
	pt, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return nil
	}
	for _, h := range o.p.heap {
		if types.Identical(h, pt.Elem()) {
			return h
		}
	}
	return nil
}

func (o *ownership) run() {
	// A heap pointer type is a candidate only if its element is a struct
	// of the program whose fields are numbers, booleans, strings or heap
	// pointers: its value is never needed outside its box.
	for _, h := range o.p.heap {
		named, ok := types.Unalias(h).(*types.Named)
		st, isStruct := h.Underlying().(*types.Struct)
		if !isStruct && plainData(h) {
			continue // a number, a string, a slice of data: a Box<i64>, a Box<String> (RFC 0017)
		}
		if !ok || !isStruct || !o.p.pkg.Local[named.Obj().Pkg()] {
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
			if plainData(f.Type()) {
				continue // data the box owns: a Vec<i64>, a struct of numbers (RFC 0017)
			}
			if o.ownerStruct(f.Type()) {
				continue // a struct value owning its pointers, as Rust's fields own Boxes (RFC 0017)
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
	// A variable whose address is kept while the variable is still used
	// lives in a cell the two names share: not an escape box, nor only
	// lent to callees that keep nothing.
	held := o.heldAddrs()
	for v := range o.p.Cells {
		el := o.elemOf(types.NewPointer(v.Type()))
		if el == nil || o.escapeVar(v) {
			continue
		}
		if ps := o.p.lends[v]; len(ps) > 0 && !o.keepsAny(ps) {
			o.p.lentOK[v] = true
			continue
		}
		at := held[v]
		if at == nil || !o.usedAfter(v, at.End()) {
			// &x lent to calls, or kept with no use of x after: not
			// supported yet for every x, but no second owner.
			o.share(el, v.Pos(), "its address is kept")
			continue
		}
		o.alias(el, at, "its address is kept while the variable is still used",
			fmt.Sprintf("the address of %s is kept while %s is still used by name, so the object has two owners", v.Name(), v.Name()),
			"in Rust a variable whose address is kept is borrowed for as long as the pointer lives, and is not used by name meanwhile (RFC 0017)",
			fmt.Sprintf("after taking its address, use %[1]s only through the pointer; or make %[1]s a pointer from the start, with new or &T{...}", v.Name()))
	}
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
			case v.IsField() && !o.candidateField(v) && !o.ownerStructField(v):
				o.share(el, id.Pos(), "a field of a struct that is not boxed holds it")
			}
			continue
		}
		if v.IsField() && (o.candidateField(v) || o.ownerStructField(v)) && o.ptrSlice(t) {
			continue // owned by the field's Vec (RFC 0017, step 7)
		}
		if !v.IsField() && v.Parent() != v.Pkg().Scope() && o.ptrSlice(t) {
			continue // a local Vec owning each (RFC 0017, step 9)
		}
		if o.escapeVar(v) {
			continue // moved into a new box at its & (escapes.go)
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
				o.alias(o.elemOf(v.Type()), id, "a range variable takes a copy of an element and keeps it",
					fmt.Sprintf("range variable %s keeps a second pointer to an element of %s", id.Name, types.ExprString(r.X)),
					"the slice owns each element, so in Rust a range over it lends each one, and the borrow ends with the iteration (RFC 0017)",
					fmt.Sprintf("only read through %s, or range over the indices and change %s[i]", id.Name, types.ExprString(r.X)))
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

// plainData reports whether t holds only data: numbers, booleans,
// strings, and slices and structs of them, with no pointer, interface,
// map or channel inside.
func plainData(t types.Type) bool { return plainDataIn(t, map[types.Type]bool{}) }

// valueData reports whether t is plain data with no slice inside:
// numbers, booleans, strings, and structs of them. Go's copy of
// one shares nothing, as Rust's copy or clone does; a copy of a slice
// shares its elements.
func valueData(t types.Type) bool {
	if !plainData(t) {
		return false
	}
	switch u := t.Underlying().(type) {
	case *types.Slice:
		return false
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if !valueData(u.Field(i).Type()) {
				return false
			}
		}
	}
	return true
}

func plainDataIn(t types.Type, seen map[types.Type]bool) bool {
	if seen[t] {
		return true // a type that holds itself: data if the rest is
	}
	seen[t] = true
	switch u := types.Unalias(t).Underlying().(type) {
	case *types.Basic:
		return true
	case *types.Slice:
		return plainDataIn(u.Elem(), seen)
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if !plainDataIn(u.Field(i).Type(), seen) {
				return false
			}
		}
		return true
	}
	return false
}

// ownerStruct reports whether t is a struct of the program, not itself a
// heap pointer's element, that holds owned pointers in its fields as
// values (RFC 0017): a struct value that owns them. A copy of one while the
// original is used would give them two owners, which the borrow checker
// refuses.
func (o *ownership) ownerStruct(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok || !o.p.pkg.Local[n.Obj().Pkg()] {
		return false
	}
	st, ok := n.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for _, h := range o.p.heap {
		if types.Identical(h, t) {
			return false // boxed: a candidate's own rules
		}
	}
	holds := false
	for i := 0; i < st.NumFields(); i++ {
		ft := st.Field(i).Type()
		switch {
		case o.elemOf(ft) != nil || o.ptrSlice(ft):
			holds = true
		case plainData(ft):
		default:
			return false
		}
	}
	return holds
}

// ownerStructField reports whether field v belongs to an owner struct.
func (o *ownership) ownerStructField(v *types.Var) bool {
	for _, f := range o.p.pkg.Files {
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, sp := range g.Specs {
				ts, ok := sp.(*ast.TypeSpec)
				if !ok {
					continue
				}
				t := o.info.Defs[ts.Name].Type()
				if st, ok := t.Underlying().(*types.Struct); ok && o.ownerStruct(t) {
					for i := 0; i < st.NumFields(); i++ {
						if st.Field(i) == v {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// ownerLit reports whether the composite literal on top of stack (or
// holding the key-value pair there) is an owner struct's value.
func (o *ownership) ownerLit(stack []ast.Node) bool {
	for i := len(stack) - 1; ; i-- {
		if c, ok := stack[i].(*ast.CompositeLit); ok {
			return o.ownerStruct(o.info.TypeOf(c))
		}
	}
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
					fn, _ := o.info.Defs[fd.Name].(*types.Func)
					for _, r := range x.Results {
						if v := keeps(r); v == nil || o.p.retBorrow[fn] != v {
							mark(v) // a returned borrow is lent back, not kept (returns.go)
						}
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
	return ok && o.ptrSlice(o.info.TypeOf(ix.X)) && rootVar(o.info, ix.X) != nil
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
				(o.fieldPlace(as.Lhs[0]) || o.localVar(as.Lhs[0]) && !o.input(as.Lhs[0])) && o.ptrSlice(o.info.TypeOf(as.Lhs[0])) &&
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
	if o.ownerStruct(t) {
		return // a struct value owning its pointers (RFC 0017)
	}
	for _, h := range o.p.heap {
		if types.Identical(h, t) {
			if !plainData(h) {
				o.share(h, pos, "a value of the type itself is held outside its box")
			}
			return // plain data: a copy of it shares nothing (RFC 0017)
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
			if o.escapeValue(e) {
				return // a local moved into a new box at its & (escapes.go)
			}
			par, child, _ := parent(stack)
			if _, lit := e.(*ast.CompositeLit); lit {
				if u, ok := par.(*ast.UnaryExpr); ok && u.Op == token.AND && u.X == child {
					return
				}
			}
			if !plainData(h) {
				o.share(h, e.Pos(), "a value of the type itself is used outside its box")
			}
			return // plain data: a copy of it shares nothing (RFC 0017)
		}
	}
	el := o.elemOf(t)
	if el == nil {
		return
	}
	if _, boxed := o.boxed[e]; boxed {
		o.share(el, e.Pos(), "an interface value holds it")
		o.ifaced[el] = true
		return
	}
	if c, ok := e.(*ast.CallExpr); ok && o.p.retBorrowCall(c) != nil {
		o.borrowUse(el, e, stack) // a borrow of the call's argument (returns.go)
		return
	}
	if o.fresh(e) {
		o.freshUse(el, e, stack)
		return
	}
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		if id, ok := ast.Unparen(u.X).(*ast.Ident); ok && len(o.p.lends[o.localOf(id)]) > 0 {
			if as, ok := stack[len(stack)-2].(*ast.AssignStmt); ok && as.Tok == token.DEFINE {
				return // p := &x, a local borrow (escapes.go): checked through p
			}
			o.placeUse(el, e, stack) // &x lent to a call (escapes.go)
			return
		}
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
		return x.Op == token.AND && (lit || o.p.escapes[x] != nil)
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

// escapeValue reports whether e, a value of a candidate's element type,
// is a variable moved into a new box at its & (escapes.go) or the literal
// declaring one: a plain local until then.
func (o *ownership) escapeValue(e ast.Expr) bool {
	if lit, ok := e.(*ast.CompositeLit); ok {
		return o.p.escapeInits[lit]
	}
	id, ok := e.(*ast.Ident)
	if !ok {
		return false
	}
	return o.escapeVar(o.localOf(id))
}

// keepsAny reports whether a callee keeps any of params (keptParams), or
// a local they are lent to is more than a local borrow (localRefs) that
// lends on only to callees keeping nothing.
func (o *ownership) keepsAny(params []*types.Var) bool {
	for _, p := range params {
		if o.inputs[p] && o.kept[p] {
			return true
		}
		if !o.inputs[p] && !o.params[p] {
			ps, ok := o.p.refs[p]
			if !ok || o.keepsAny(ps) {
				return true // a variable that keeps it
			}
		}
	}
	return false
}

// escapeVar reports whether v is moved into a new box at its &.
func (o *ownership) escapeVar(v *types.Var) bool {
	for _, w := range o.p.escapes {
		if w == v {
			return true
		}
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
	if fn == nil || !o.p.pkg.Local[fn.Pkg()] {
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
				if r == child && len(p.Lhs) == len(p.Rhs) && (o.localVar(p.Lhs[i]) || o.fieldPlace(p.Lhs[i]) || o.elemPlace(p.Lhs[i])) {
					if o.fieldPlace(p.Lhs[i]) {
						o.chained[el] = true
					}
					return // a parameter assigned is refused where it is assigned; a field gets it (RFC 0017, step 4)
				}
			}
		}
	case *ast.KeyValueExpr:
		if p.Value == child && (o.boxedLit(stack[:len(stack)-1]) || o.ownerLit(stack[:len(stack)-1])) {
			return
		}
	case *ast.CompositeLit:
		if o.boxedLit(stack[:len(stack)-1]) || o.ptrSlice(o.info.TypeOf(p)) || o.ownerStruct(o.info.TypeOf(p)) {
			return // a field of a new pointer or struct value, or an element of a new Vec (step 9)
		}
	case *ast.BinaryExpr:
		if (p.Op == token.EQL || p.Op == token.NEQ) && !o.info.Types[p.X].IsNil() && !o.info.Types[p.Y].IsNil() {
			return // compared by address with another pointer: a temporary (std::ptr::eq)
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
	if _, ok := par.(*ast.ReturnStmt); ok && o.returnsBorrow(e, stack) {
		return // a borrow of the input lent back (returns.go)
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
				if o.localVar(l) || o.fieldPlace(l) || o.elemPlace(l) {
					return // a variable, a field or a Vec's element assigned a pointer, which moves there (RFC 0017, step 4)
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
		// Compared with nil, or with another pointer: a read of both
		// addresses, std::ptr::eq in Rust; two owners are never equal
		// (RFC 0017).
		return
	}
	if valueCopy(o.info, par, child) {
		// *p, or p.M() for a value receiver, of a type holding a slice: a
		// copy of the value shares the slice's elements with *p, not
		// supported yet, but no second pointer.
		o.share(el, e.Pos(), "a copy of its value shares its elements")
		return
	}
	notes := []string{"one owner per object (RFC 0017): a pointer kept elsewhere is moved there, and only read through or lent while its owner stays unchanged"}
	if l := keptAt(par, child); l != nil && o.localVar(l) {
		notes = append(notes, "a local variable that only reads through it is a borrow; one that changes through it, or is used once its owner changes, would be an exclusive borrow (&mut), not supported yet")
	}
	notes = append(notes, "use one name for the object from here on, read through it or pass it to a call, or give the copy an object of its own")
	o.alias(el, e, "a copy of it is kept",
		fmt.Sprintf("%s is copied and kept%s, so its object has two owners", types.ExprString(e), keptIn(keptAt(par, child))), notes...)
}

// heldAddrs returns, for each variable, the first &x of the program held
// in a variable, a field or a composite literal: kept, not lent to a call.
func (o *ownership) heldAddrs() map[*types.Var]*ast.UnaryExpr {
	held := map[*types.Var]*ast.UnaryExpr{}
	for _, f := range o.p.pkg.Files {
		var stack []ast.Node
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			u, ok := n.(*ast.UnaryExpr)
			if !ok || u.Op != token.AND {
				return true
			}
			id, ok := ast.Unparen(u.X).(*ast.Ident)
			if !ok {
				return true
			}
			v, ok := o.info.Uses[id].(*types.Var)
			if !ok || held[v] != nil {
				return true
			}
			switch par, _, _ := parent(stack); par.(type) {
			case *ast.AssignStmt, *ast.ValueSpec, *ast.CompositeLit, *ast.KeyValueExpr:
				held[v] = u
			}
			return true
		})
	}
	return held
}

// usedAfter reports whether v is used after pos.
func (o *ownership) usedAfter(v *types.Var, pos token.Pos) bool {
	for id, obj := range o.info.Uses {
		if obj == v && id.Pos() > pos {
			return true
		}
	}
	return false
}

// valueCopy reports whether par copies the value child points to: *child,
// or child.M() for a method with a value receiver.
func valueCopy(info *types.Info, par, child ast.Node) bool {
	switch p := par.(type) {
	case *ast.StarExpr:
		return true
	case *ast.SelectorExpr:
		s := info.Selections[p]
		return p.X == child && s != nil && s.Kind() == types.MethodVal && !isPointer(s.Obj().(*types.Func).Signature().Recv().Type())
	}
	return false
}

// keptAt returns the place par assigns child to, or nil.
func keptAt(par, child ast.Node) ast.Expr {
	if as, ok := par.(*ast.AssignStmt); ok && len(as.Lhs) == len(as.Rhs) {
		for i, r := range as.Rhs {
			if r == child {
				return as.Lhs[i]
			}
		}
	}
	return nil
}

// keptIn names place l for a message: " in l", or nothing for nil.
func keptIn(l ast.Expr) string {
	if l == nil {
		return ""
	}
	return " in " + types.ExprString(l)
}

// returnsBorrow reports whether e, returned, is a place rooted at the
// input whose borrow its function returns (returns.go).
func (o *ownership) returnsBorrow(e ast.Expr, stack []ast.Node) bool {
	for i := len(stack) - 1; ; i-- { // a return is inside a function
		if fd, ok := stack[i].(*ast.FuncDecl); ok {
			fn, _ := o.info.Defs[fd.Name].(*types.Func)
			v := o.p.retBorrow[fn]
			return v != nil && placeRoot(o.info, e) == v
		}
		if _, ok := stack[i].(*ast.FuncLit); ok {
			return false
		}
	}
}

// borrowUse checks a use of the result of a call that returns a borrow of
// its argument (returns.go): read through, lent to a call, given to a
// temporary borrow, or dropped, but never kept, which would give the
// argument's object a second name.
func (o *ownership) borrowUse(el types.Type, e ast.Expr, stack []ast.Node) {
	par, child, at := parent(stack)
	if _, ok := par.(*ast.ExprStmt); ok {
		return
	}
	if o.lentToBorrow(par, child) || o.readOrLend(par, child, stack[:at+1]) {
		return
	}
	if as, ok := par.(*ast.AssignStmt); ok && as.Tok == token.DEFINE {
		if id, ok := as.Lhs[0].(*ast.Ident); ok {
			if _, ref := o.p.refs[o.localOf(id)]; ref {
				return // p := f(x): a local borrow of x (escapes.go), checked through p
			}
		}
	}
	if _, ok := par.(*ast.ReturnStmt); ok && o.returnsBorrow(o.p.retBorrowCall(e.(*ast.CallExpr)), stack) {
		return // lent back on: return second(n) in a function lending n
	}
	if b, ok := par.(*ast.BinaryExpr); ok && (b.Op == token.EQL || b.Op == token.NEQ) {
		return // compared: a read of its address (std::ptr::eq)
	}
	l := keptAt(par, child)
	notes := []string{"in Rust the result borrows the argument, and lasts only while the argument is unchanged (RFC 0017)"}
	if as, ok := par.(*ast.AssignStmt); ok && as.Tok == token.DEFINE && o.localVar(l) {
		notes = append(notes, fmt.Sprintf("a new local is such a borrow when the argument is a pointer variable, p := f(x); %s lends another argument, not supported yet", types.ExprString(e)))
	}
	notes = append(notes, "read through the result or pass it to a call where it is returned, or keep the argument's own pointer instead")
	o.alias(el, e, "a borrow a call returns is kept",
		fmt.Sprintf("%s lends back a pointer its caller owns, and it is kept%s, so the object has two owners", types.ExprString(e), keptIn(l)), notes...)
}

// readOrLend reports whether child, a pointer, is read through by par (a
// field read or a method call with a pointer receiver), or lent to a
// call of a program function, neither of which keeps it.
func (o *ownership) readOrLend(par, child ast.Node, stack []ast.Node) bool {
	switch p := par.(type) {
	case *ast.StarExpr:
		// *p of value data: a copy out of the box, or the box's value
		// replaced, either sharing nothing (RFC 0017). Not &*p.
		above, _, _ := parent(stack)
		if u, ok := above.(*ast.UnaryExpr); ok && u.Op == token.AND {
			return false
		}
		return valueData(o.info.TypeOf(p))
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
			// A value method copies a value-data value out for the call
			// (RFC 0017), sharing nothing.
			return (ptrRecv || valueData(recv.Type())) && o.programCallee(call) != nil && !o.deferred(stack)
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
		return p.Value == child && (o.boxedLit(stack[:len(stack)-1]) || o.ownerLit(stack[:len(stack)-1]))
	case *ast.CompositeLit:
		o.chained[el] = true
		return o.boxedLit(stack) || o.ptrSlice(o.info.TypeOf(p)) || o.ownerStruct(o.info.TypeOf(p)) // a field, or an element of a new Vec
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
			if p.X == place && (o.ptrSlice(o.info.TypeOf(p.X)) || plainData(o.info.TypeOf(p.X))) {
				place = p // an element of a Vec of owned pointers or of data, changed through
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
			return basic || o.elemOf(t) != nil || plainData(t) // data the box owns, replaced in place
		case *ast.UnaryExpr:
			// &p.f of data lent to a call that keeps nothing: Rust lends
			// &mut p.f for the call (RFC 0017).
			t := o.info.TypeOf(place.(ast.Expr))
			v := o.p.lentTo(stack[:j+1])
			return p.Op == token.AND && p.X == place && v != nil && o.inputs[v] && !o.kept[v] && !o.deferred(stack[:j+1]) && plainData(t)
		}
		return false // p.s[i] = x, or &p.f kept
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
		if c, ok := stack[i].(*ast.CompositeLit); ok {
			if o.p.escapeInits[c] {
				return true // a local's value, moved into a box at its & (escapes.go)
			}
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
