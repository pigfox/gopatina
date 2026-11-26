package subset

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sync"

	"github.com/pigfox/gopatina/internal/diag"
	"github.com/pigfox/gopatina/internal/load"
)

// Pointers that escape (RFC 0013). A pointer type *T is a heap type when
// some *T of the program reaches a place a borrow cannot be: a field, a
// result, an interface, a slice or map element, a comparison, a variable
// assigned after its declaration or starting as nil, a package-level
// variable, a pointer to it. Every *T of a heap type is a heap pointer,
// Ptr<T>, and every variable of type T whose address is taken lives in a
// cell, so pointers of one type have one representation however they
// flow. Every other pointer is a borrow, as in phase 1, so programs
// accepted before emit the same crate.

// PtrPlan is the program's pointer plan.
type PtrPlan struct {
	pkg *load.Package
	// heap holds the element types T whose *T is a heap type.
	heap []types.Type
	// Cells are the variables whose address is a heap pointer.
	Cells map[*types.Var]bool
	// Interior are the &x.f and &s[i] that are heap pointers: pointers
	// into a struct or a slice that escape, not supported yet.
	Interior []ast.Expr
	// owned holds the element types T whose *T is owned (RFC 0016), and
	// shared why each other heap type is not; both are computed once, on
	// first use.
	owned     []types.Type
	shared    []sharing
	ownedDone bool
	// nonNil holds the owned pointers that are never nil (RFC 0017,
	// step 2), computed on first use.
	nonNil *nonNilPlan
	// ownerErrs are the uses of owned pointers refused (RFC 0017), and
	// chained the element types moved into fields (Chained).
	ownerErrs []ownerError
	chained   []types.Type
	// borrows are the temporary borrows (RFC 0017, step 5), and sources
	// the variables each borrows from.
	borrows map[*types.Var]bool
	sources map[*types.Var]map[*types.Var]bool
	// rangeElems are the values of ranges over slices (step 7).
	rangeElems map[*types.Var]bool
	// takes are the fields moved out and refilled at once (step 6).
	takes map[ast.Expr]*ast.AssignStmt
	// escapes are the &x that move a local value into a new box, and
	// escapeInits the literals declaring those variables (escapes.go).
	escapes     map[*ast.UnaryExpr]*types.Var
	escapeInits map[*ast.CompositeLit]bool
	// argEscapes are the &x given to a call that nothing uses after:
	// escapes when the callee keeps the parameter (keptArgs).
	argEscapes []argEscape
	// lends are the variables whose every address is lent to a call, with
	// the parameters lent to, and lentOK those no callee keeps.
	lends  map[*types.Var][]*types.Var
	lentOK map[*types.Var]bool
	// refs are the local borrows p := &x, with the parameters each is
	// lent to, and refSources the x each borrows (escapes.go).
	refs       map[*types.Var][]*types.Var
	refSources map[*types.Var]*types.Var
	refCalls   map[*types.Var]*ast.CallExpr
	refAsserts map[*types.Var]*ast.TypeAssertExpr
	refAs      map[*types.Var]*ast.CallExpr
	nilConv    map[*ast.Ident]bool
	// complete is set once the plan is made; from then on HeapType and
	// Owned keep their answers, by element type, in heapOf and ownedOf.
	complete bool
	heapOf   map[types.Type]bool
	ownedOf  map[types.Type]bool
	// retBorrow holds, by function, the input whose borrow it returns
	// (returns.go).
	retBorrow map[*types.Func]*types.Var
	// retMut holds the functions that change through the input they lend
	// back: an exclusive borrow returned.
	retMut map[*types.Func]bool
	// walked are the walkers of the functions lending an input back
	// (returns.go): temporary borrows that are returned.
	walked map[*types.Var]bool
}

// HeapType reports whether t, a pointer type, is a heap type.
func (p *PtrPlan) HeapType(t types.Type) bool {
	pt, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return false
	}
	if is, ok := p.heapOf[pt.Elem()]; ok {
		return is
	}
	is := false
	for _, h := range p.heap {
		if types.Identical(h, pt.Elem()) {
			is = true
			break
		}
	}
	if p.complete {
		p.heapOf[pt.Elem()] = is // the plan no longer changes
	}
	return is
}

// Heap reports whether the program has a heap pointer.
func (p *PtrPlan) Heap() bool { return len(p.heap) > 0 }

var (
	ptrMu    sync.Mutex
	ptrPlans = map[*load.Package]*PtrPlan{}
)

// Ptrs returns pkg's pointer plan, made once.
func Ptrs(pkg *load.Package) *PtrPlan {
	ptrMu.Lock()
	defer ptrMu.Unlock()
	if p, ok := ptrPlans[pkg]; ok {
		return p
	}
	p := &PtrPlan{pkg: pkg, Cells: map[*types.Var]bool{}, nilConv: map[*ast.Ident]bool{}, heapOf: map[types.Type]bool{}, ownedOf: map[types.Type]bool{}}
	p.build()
	p.retBorrows()
	p.localRefs()
	p.escapeBoxes()
	p.ownedOnce()
	for _, v := range p.escapes {
		if p.Owned(types.NewPointer(v.Type())) {
			delete(p.Cells, v) // a plain local, moved into its box (escapes.go)
		}
	}
	for v := range p.lentOK {
		if p.Owned(types.NewPointer(v.Type())) {
			delete(p.Cells, v) // a plain local, lent (escapes.go)
		}
	}
	p.complete = true
	ptrPlans[pkg] = p
	return p
}

// mark makes the pointer type t a heap type, and so a pointer to a
// pointer type its element too.
func (p *PtrPlan) mark(t types.Type) {
	pt, ok := types.Unalias(t).(*types.Pointer)
	if !ok || p.HeapType(t) {
		return
	}
	p.heap = append(p.heap, pt.Elem())
	p.mark(pt.Elem()) // **T: *T is held in a cell, as a heap pointer too
}

// build finds the heap types, then the cells.
func (p *PtrPlan) build() {
	info := p.pkg.Info
	markExpr := func(e ast.Expr) {
		if t := info.TypeOf(e); t != nil {
			p.mark(t)
		}
	}
	var addrs []ast.Expr // the operands of & and of implicit &x.M()
	for _, f := range p.pkg.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if fn, ok := info.Defs[n.Name].(*types.Func); ok {
					res := fn.Signature().Results()
					for i := 0; i < res.Len(); i++ {
						p.mark(res.At(i).Type()) // a result escapes
					}
				}
			case *ast.TypeSpec:
				if st, ok := info.TypeOf(n.Type).Underlying().(*types.Struct); ok {
					for i := 0; i < st.NumFields(); i++ {
						p.mark(st.Field(i).Type()) // a field
					}
				}
				p.elems(info.TypeOf(n.Type))
			case *ast.ValueSpec:
				for _, name := range n.Names {
					if v, ok := info.Defs[name].(*types.Var); ok && (v.Parent() == v.Pkg().Scope() || len(n.Values) == 0) {
						p.mark(v.Type()) // package-level, or starting nil
					}
				}
				if n.Type != nil {
					p.elems(info.TypeOf(n.Type))
				}
			case *ast.AssignStmt:
				for _, l := range n.Lhs {
					id, isID := ast.Unparen(l).(*ast.Ident)
					if n.Tok != token.DEFINE || isID && info.Defs[id] == nil {
						markExpr(l) // assigned after its declaration
					}
				}
				for _, r := range n.Rhs {
					if !addressOf(r) {
						markExpr(r) // not starting as &x or &T{...}
					}
				}
			case *ast.BinaryExpr:
				if n.Op == token.EQL || n.Op == token.NEQ {
					markExpr(n.X) // identity needs a heap address
					markExpr(n.Y)
				}
			case *ast.CallExpr:
				if tv := info.Types[n]; tv.Type != nil {
					if _, ok := tv.Type.(*types.Tuple); !ok {
						markExpr(n) // a call's pointer result
					}
				}
				for _, a := range n.Args {
					if info.Types[a].IsNil() {
						if sig, ok := info.TypeOf(n.Fun).Underlying().(*types.Signature); ok {
							for i := 0; i < sig.Params().Len(); i++ {
								p.mark(sig.Params().At(i).Type()) // nil passed for a pointer
							}
						}
					}
				}
				if sel, ok := ast.Unparen(n.Fun).(*ast.SelectorExpr); ok {
					if s := info.Selections[sel]; s != nil && s.Kind() == types.MethodVal {
						if recv := s.Obj().(*types.Func).Signature().Recv(); recv != nil && isPointer(recv.Type()) && !isPointer(info.TypeOf(sel.X)) {
							addrs = append(addrs, sel.X) // x.M() takes &x
						}
					}
				}
			case *ast.UnaryExpr:
				if n.Op == token.AND {
					addrs = append(addrs, n.X)
				}
			case *ast.StarExpr:
				if t := info.TypeOf(n.X); t != nil && isPointer(t) {
					if pt := types.Unalias(t).(*types.Pointer); isPointer(pt.Elem()) {
						p.mark(t) // **T
					}
				}
			case *ast.ReturnStmt:
				for _, r := range n.Results {
					markExpr(r)
				}
			case *ast.SendStmt:
				markExpr(n.Value)
			case *ast.CompositeLit:
				p.elems(info.TypeOf(n))
			}
			return true
		})
	}
	for e := range Ifaces(p.pkg).Boxed {
		markExpr(e) // in an interface
	}
	for _, tv := range info.Types {
		if tv.Type != nil {
			p.elems(tv.Type) // the elements of a slice of pointers, however made
		}
	}
	// A variable whose address is a heap pointer lives in a cell; a
	// pointer into a struct or a slice that escapes is not supported.
	for _, x := range addrs {
		t := info.TypeOf(x)
		if t == nil || !p.HeapType(types.NewPointer(t)) {
			continue
		}
		switch x := ast.Unparen(x).(type) {
		case *ast.Ident:
			if v, ok := info.Uses[x].(*types.Var); ok {
				p.Cells[v] = true
			}
		case *ast.CompositeLit:
			// &T{...}: a new cell
		default:
			p.Interior = append(p.Interior, x)
		}
	}
}

// elems marks the pointer element types of a slice or map type.
func (p *PtrPlan) elems(t types.Type) {
	switch u := t.Underlying().(type) {
	case *types.Slice:
		p.mark(u.Elem())
	case *types.Map:
		p.mark(u.Elem())
	}
}

// addressOf reports whether e is &x or &T{...}.
func addressOf(e ast.Expr) bool {
	u, ok := ast.Unparen(e).(*ast.UnaryExpr)
	return ok && u.Op == token.AND
}

// checkPtrs reports what RFC 0013's first step leaves out: a pointer into
// a struct or a slice that escapes, and a range variable, a package-level
// variable or a variable of a pointer type that would live in a cell.
func (c *checker) checkPtrs() {
	p := Ptrs(c.pkg)
	for _, e := range p.ownerErrors() {
		c.diags = append(c.diags, diag.Errorf(c.pkg.Fset.Position(e.pos), "%s", e.msg).With(e.notes...))
	}
	reported := len(p.ownerErrors()) > 0 || len(p.Interior) > 0
	for _, sh := range p.shared {
		// One owner per object is the only representation of a pointer
		// (RFC 0017): a type it cannot be inferred for is not supported,
		// unless an error above already says why.
		if reported {
			break
		}
		c.unsupported(&ast.Ident{NamePos: sh.pos, Name: "_"}, fmt.Sprintf("a *%s with more than one owner (%s)", types.TypeString(sh.t, types.RelativeTo(c.pkg.Types)), sh.why))
	}
	for v := range p.Cells {
		if !isPointer(v.Type()) && !p.Owned(types.NewPointer(v.Type())) {
			c.unsupported(&ast.Ident{NamePos: v.Pos(), Name: v.Name()}, fmt.Sprintf("the address of %s kept where its object would have more than one owner", v.Name()))
		}
	}
	for _, x := range p.Interior {
		c.unsupported(x, "a pointer into a struct field or a slice element that escapes (RFC 0013)")
	}
	for v := range p.Cells {
		if isPointer(v.Type()) {
			c.unsupported(&ast.Ident{NamePos: v.Pos(), Name: v.Name()}, "a pointer to a pointer variable that escapes")
		}
	}
	for _, f := range c.pkg.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if r, ok := n.(*ast.RangeStmt); ok {
				for _, e := range []ast.Expr{r.Key, r.Value} {
					if id, ok := e.(*ast.Ident); ok {
						if v, ok := c.pkg.Info.Defs[id].(*types.Var); ok && p.Cells[v] {
							c.unsupported(id, "the address of a range variable that escapes")
						}
					}
				}
			}
			return true
		})
	}
}
