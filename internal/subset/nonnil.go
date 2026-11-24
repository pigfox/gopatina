package subset

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
)

// Non-nil pointers (RFC 0017, step 2). An owned pointer (RFC 0016) is an
// Option<Box<T>>, as Rust writes a pointer that may be absent. Where the
// program shows that one is never nil, Rust writes it without the Option,
// and so does GoPatina:
//
//   - a function whose every return is a new pointer (&T{...}, new(T), or
//     a call of such a function, or a local variable holding one) returns
//     a Box<T>;
//   - a local variable declared from such a value and never assigned
//     again, nor compared with nil, is a Box<T>;
//   - a parameter or receiver that the function only reads through, never
//     compares with nil or assigns, and reads through before anything else
//     the function does could be seen, is a &T (a method's receiver is
//     &self). A caller that holds an Option unwraps it at the call, which
//     panics exactly where Go's callee would: the callee's first act is
//     that read. So every call must pass it in a call whose other
//     arguments are evaluated before it or have nothing to see (constants
//     and variables).

// NonNilResult reports whether program function fn returns an owned
// pointer that is never nil: a Box<T>.
func (p *PtrPlan) NonNilResult(fn *types.Func) bool {
	p.nonNilOnce()
	return p.nonNil.results[fn]
}

// NonNilVar reports whether v, a local variable, parameter or receiver of
// an owned pointer type, is never nil: a Box<T> for a local variable, a
// &T for a parameter or receiver.
func (p *PtrPlan) NonNilVar(v *types.Var) bool {
	p.nonNilOnce()
	return p.nonNil.vars[v]
}

// nonNilPlan holds the results of the non-nil analysis.
type nonNilPlan struct {
	results map[*types.Func]bool
	vars    map[*types.Var]bool
}

// nonNilAnalysis finds the non-nil pointers: the greatest sets of results,
// locals and parameters that satisfy the rules, found by removing those
// that break them until none does.
type nonNilAnalysis struct {
	p     *PtrPlan
	info  *types.Info
	decls map[*types.Func]*ast.FuncDecl
	// params are the program's parameters and receivers, with the function
	// they belong to.
	params map[*types.Var]*types.Func
	// calls are the calls of each program function.
	calls map[*types.Func][]*ast.CallExpr
	// uses lists each variable's uses.
	uses map[*types.Var][]*ast.Ident
	// parents maps each node to the node above it.
	parents map[ast.Node]ast.Node
	// inits holds the initializer of each local variable declared with
	// one value, and assigned counts the other assignments to each.
	inits    map[*types.Var]ast.Expr
	assigned map[*types.Var]bool
	// escaped are the functions used other than by a call.
	escaped map[*types.Func]bool
}

func (p *PtrPlan) nonNilOnce() {
	if p.nonNil != nil {
		return
	}
	a := &nonNilAnalysis{
		p:        p,
		info:     p.pkg.Info,
		decls:    map[*types.Func]*ast.FuncDecl{},
		params:   map[*types.Var]*types.Func{},
		calls:    map[*types.Func][]*ast.CallExpr{},
		uses:     map[*types.Var][]*ast.Ident{},
		parents:  map[ast.Node]ast.Node{},
		inits:    map[*types.Var]ast.Expr{},
		assigned: map[*types.Var]bool{},
		escaped:  map[*types.Func]bool{},
	}
	p.nonNil = a.run()
}

// owned reports whether t is an owned pointer type.
func (a *nonNilAnalysis) owned(t types.Type) bool {
	return t != nil && a.p.HeapType(t) && a.p.Owned(t)
}

func (a *nonNilAnalysis) run() *nonNilPlan {
	a.index()
	plan := &nonNilPlan{results: map[*types.Func]bool{}, vars: map[*types.Var]bool{}}
	// Every candidate starts in; the loop removes those that break a rule.
	for fn, fd := range a.decls {
		sig := fn.Signature()
		if sig.Results().Len() == 1 && sig.Results().At(0).Name() == "" && a.owned(sig.Results().At(0).Type()) && fd.Body != nil {
			plan.results[fn] = true
		}
	}
	for v, init := range a.inits {
		if a.owned(v.Type()) && init != nil && !a.assigned[v] && !a.nilCompared(v) && !a.p.Cells[v] {
			plan.vars[v] = true
		}
	}
	for v, fn := range a.params {
		if a.owned(v.Type()) && !a.escaped[fn] && !a.assigned[v] && a.onlyReadThrough(v) && a.firstRead(v, a.decls[fn]) {
			plan.vars[v] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for fn := range plan.results {
			if !a.returnsNew(a.decls[fn], plan) {
				delete(plan.results, fn)
				changed = true
			}
		}
		for v := range plan.vars {
			if fn, ok := a.params[v]; ok {
				if !a.callsSafe(fn, v, plan) {
					delete(plan.vars, v)
					changed = true
				}
				continue
			}
			if !a.isNew(a.inits[v], plan) {
				delete(plan.vars, v)
				changed = true
			}
		}
	}
	return plan
}

// index records the program's functions, their parameters and calls,
// every variable's uses and assignments, and each node's parent.
func (a *nonNilAnalysis) index() {
	for _, f := range a.p.pkg.Files {
		var stack []ast.Node
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if len(stack) > 0 {
				a.parents[n] = stack[len(stack)-1]
			}
			stack = append(stack, n)
			switch x := n.(type) {
			case *ast.FuncDecl:
				if fn, ok := a.info.Defs[x.Name].(*types.Func); ok {
					a.decls[fn] = x
					sig := fn.Signature()
					if r := sig.Recv(); r != nil {
						a.params[r] = fn
					}
					for i := 0; i < sig.Params().Len(); i++ {
						a.params[sig.Params().At(i)] = fn
					}
				}
			case *ast.Ident:
				switch obj := a.info.Uses[x].(type) {
				case *types.Var:
					a.uses[obj] = append(a.uses[obj], x)
				case *types.Func:
					if obj.Pkg() == a.p.pkg.Types && !a.calledAs(x, stack) {
						a.escaped[obj] = true
					}
				}
			case *ast.CallExpr:
				if fn := a.callee(x); fn != nil {
					a.calls[fn] = append(a.calls[fn], x)
				}
			case *ast.AssignStmt:
				for i, l := range x.Lhs {
					id, ok := ast.Unparen(l).(*ast.Ident)
					if !ok {
						continue
					}
					if v, ok := a.info.Defs[id].(*types.Var); ok {
						if x.Tok == token.DEFINE && len(x.Lhs) == len(x.Rhs) {
							a.inits[v] = x.Rhs[i]
						}
						continue
					}
					if v, ok := a.info.Uses[id].(*types.Var); ok {
						a.assigned[v] = true
					}
				}
			case *ast.ValueSpec:
				for i, id := range x.Names {
					if v, ok := a.info.Defs[id].(*types.Var); ok && len(x.Values) == len(x.Names) && v.Parent() != v.Pkg().Scope() {
						a.inits[v] = x.Values[i]
					}
				}
			case *ast.UnaryExpr:
				if id, ok := ast.Unparen(x.X).(*ast.Ident); ok && x.Op == token.AND {
					if v, ok := a.info.Uses[id].(*types.Var); ok {
						a.assigned[v] = true // its address is taken
					}
				}
			}
			return true
		})
	}
}

// calledAs reports whether id, naming a function, is the function of a
// call: directly, or as a method selected for the call.
func (a *nonNilAnalysis) calledAs(id *ast.Ident, stack []ast.Node) bool {
	child := ast.Node(id)
	for i := len(stack) - 2; ; i-- { // a name is never a file
		switch p := stack[i].(type) {
		case *ast.ParenExpr:
			child = p
			continue
		case *ast.SelectorExpr:
			if p.Sel == child {
				child = p
				continue
			}
		case *ast.CallExpr:
			return p.Fun == child
		}
		return false
	}
}

// callee returns the program function or method c calls statically.
func (a *nonNilAnalysis) callee(c *ast.CallExpr) *types.Func {
	var fn *types.Func
	switch f := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		fn, _ = a.info.Uses[f].(*types.Func)
	case *ast.SelectorExpr:
		fn, _ = a.info.Uses[f.Sel].(*types.Func)
	}
	if fn == nil || fn.Pkg() != a.p.pkg.Types {
		return nil
	}
	return fn
}

// isNew reports whether e is a pointer that is never nil: a new one, a
// call of a function that returns one, or a local variable holding one.
func (a *nonNilAnalysis) isNew(e ast.Expr, plan *nonNilPlan) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.UnaryExpr:
		_, lit := ast.Unparen(x.X).(*ast.CompositeLit)
		return x.Op == token.AND && (lit || a.p.EscapeBox(x) != nil)
	case *ast.CallExpr:
		if id, ok := ast.Unparen(x.Fun).(*ast.Ident); ok {
			if b, ok := a.info.Uses[id].(*types.Builtin); ok {
				return b.Name() == "new"
			}
		}
		fn := a.callee(x)
		return fn != nil && plan.results[fn]
	case *ast.Ident:
		v, ok := a.info.Uses[x].(*types.Var)
		_, param := a.params[v]
		return ok && !param && plan.vars[v]
	}
	return false
}

// returnsNew reports whether every return of fd returns a pointer that is
// never nil.
func (a *nonNilAnalysis) returnsNew(fd *ast.FuncDecl, plan *nonNilPlan) bool {
	ok := true
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(x.Results) != 1 || !a.isNew(x.Results[0], plan) {
				ok = false
			}
		}
		return ok
	})
	return ok
}

// nilCompared reports whether v is compared with nil.
func (a *nonNilAnalysis) nilCompared(v *types.Var) bool {
	for _, id := range a.uses[v] {
		if b, ok := a.above(id).(*ast.BinaryExpr); ok && (b.Op == token.EQL || b.Op == token.NEQ) {
			return true
		}
	}
	return false
}

// above returns the node above n, skipping parentheses.
func (a *nonNilAnalysis) above(n ast.Node) ast.Node {
	p := a.parents[n]
	for {
		if _, ok := p.(*ast.ParenExpr); !ok {
			return p
		}
		p = a.parents[p]
	}
}

// onlyReadThrough reports whether every use of parameter v reads through
// it (v.f, v.m()) or passes it to a call.
func (a *nonNilAnalysis) onlyReadThrough(v *types.Var) bool {
	for _, id := range a.uses[v] {
		switch a.above(id).(type) {
		case *ast.SelectorExpr:
			// v.f or v.m: v is X, as a variable is never a selector's name.
		case *ast.CallExpr:
			// Lent to a call: of a program function, as any other call
			// would share v's type (RFC 0016).
		default:
			return false
		}
	}
	return true
}

// firstRead reports whether the first thing fd's body does that could be
// seen, in Go's order of evaluation, is to read a field through v: then a
// nil v panics there, before anything else, and a caller that unwraps it
// panics at the same point.
func (a *nonNilAnalysis) firstRead(v *types.Var, fd *ast.FuncDecl) bool {
	if fd == nil || fd.Body == nil || len(fd.Body.List) == 0 {
		return false
	}
	var exprs []ast.Expr
	switch s := fd.Body.List[0].(type) {
	case *ast.IfStmt:
		if s.Init != nil {
			return false
		}
		exprs = []ast.Expr{s.Cond}
	case *ast.ReturnStmt:
		exprs = s.Results
	case *ast.ExprStmt:
		exprs = []ast.Expr{s.X}
	case *ast.AssignStmt:
		if len(s.Lhs) == 1 && a.writesThrough(s.Lhs[0], v) && a.calm(s.Rhs[0]) {
			return true // v.f = x, v.f += x: nothing before the write through v
		}
		for _, l := range s.Lhs {
			if _, ok := ast.Unparen(l).(*ast.Ident); !ok && !(len(s.Lhs) == 1 && a.writesThrough(l, v)) {
				return false
			}
		}
		// v.f = g(v.f, ...): the value is evaluated first, and a read
		// through v in it panics before anything else is done.
		exprs = s.Rhs
	case *ast.IncDecStmt:
		return a.writesThrough(s.X, v) // v.f++
	default:
		return false
	}
	// gc makes the calls first, in order; a call's own arguments are
	// evaluated just before it. The first call with no call inside it is
	// therefore the first thing done, if there is a call at all.
	var first *ast.CallExpr
	for _, e := range exprs {
		ast.Inspect(e, func(n ast.Node) bool {
			if first != nil {
				return false
			}
			if c, ok := n.(*ast.CallExpr); ok && a.isCall(c) && !a.hasCall(c) {
				first = c
				return false
			}
			return true
		})
		if first != nil {
			break
		}
	}
	if first != nil {
		var parts []ast.Expr
		if sel, ok := ast.Unparen(first.Fun).(*ast.SelectorExpr); ok {
			if s := a.info.Selections[sel]; s != nil && s.Kind() == types.MethodVal {
				if id, ok := ast.Unparen(sel.X).(*ast.Ident); ok && a.info.Uses[id] == v && !isPointer(s.Obj().(*types.Func).Signature().Recv().Type()) {
					return true // v.M() of a value method copies *v first
				}
				parts = append(parts, sel.X)
			}
		}
		exprs = append(parts, first.Args...)
	}
	for _, e := range exprs {
		if r, ok := a.firstPanic(e); ok {
			return a.readsThrough(r, v)
		}
	}
	return false
}

// writesThrough reports whether e, a place written, is a field of v
// itself: v.f. Go dereferences v to write it, after evaluating the value
// (for v.f = x) or to read it first (for v.f += x).
func (a *nonNilAnalysis) writesThrough(e ast.Expr, v *types.Var) bool {
	sel, ok := ast.Unparen(e).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := ast.Unparen(sel.X).(*ast.Ident)
	return ok && a.info.Uses[id] == v
}

// calm reports whether evaluating e can neither be seen nor panic: no
// call, and nothing that panics.
func (a *nonNilAnalysis) calm(e ast.Expr) bool {
	calls := false
	ast.Inspect(e, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok && a.isCall(c) {
			calls = true
		}
		return !calls
	})
	_, panics := a.firstPanic(e)
	return !calls && !panics
}

// isCall reports whether c is a call that does something: not a
// conversion, and not a built-in that only computes (len, cap, min, max).
func (a *nonNilAnalysis) isCall(c *ast.CallExpr) bool {
	if a.info.Types[c.Fun].IsType() {
		return false
	}
	if id, ok := ast.Unparen(c.Fun).(*ast.Ident); ok {
		if b, ok := a.info.Uses[id].(*types.Builtin); ok {
			switch b.Name() {
			case "len", "cap", "min", "max":
				return false
			}
		}
	}
	return true
}

// hasCall reports whether c's function or arguments hold a call.
func (a *nonNilAnalysis) hasCall(c *ast.CallExpr) bool {
	found := false
	for _, e := range append([]ast.Expr{c.Fun}, c.Args...) {
		ast.Inspect(e, func(n ast.Node) bool {
			if x, ok := n.(*ast.CallExpr); ok && a.isCall(x) {
				found = true
			}
			return !found
		})
	}
	return found
}

// firstPanic returns the first operation of e, in evaluation order, that
// can panic: a read through a pointer, an index, a slice, a division by a
// value not known, an explicit dereference or a type assertion.
func (a *nonNilAnalysis) firstPanic(e ast.Expr) (ast.Expr, bool) {
	var found ast.Expr
	var walk func(n ast.Expr)
	walk = func(n ast.Expr) {
		if found != nil || n == nil {
			return
		}
		switch x := n.(type) {
		case *ast.ParenExpr:
			walk(x.X)
		case *ast.SelectorExpr:
			walk(x.X)
			if s := a.info.Selections[x]; found == nil && s != nil && s.Kind() == types.FieldVal && isPtr(a.info.TypeOf(x.X)) {
				found = x
			}
		case *ast.IndexExpr:
			walk(x.X)
			walk(x.Index)
			if found == nil {
				found = x
			}
		case *ast.SliceExpr:
			walk(x.X)
			walk(x.Low)
			walk(x.High)
			walk(x.Max)
			if found == nil {
				found = x
			}
		case *ast.StarExpr:
			walk(x.X)
			if found == nil {
				found = x
			}
		case *ast.TypeAssertExpr:
			walk(x.X)
			if found == nil {
				found = x
			}
		case *ast.UnaryExpr:
			walk(x.X)
		case *ast.BinaryExpr:
			walk(x.X)
			if x.Op == token.LAND || x.Op == token.LOR {
				return // the right side may not run
			}
			walk(x.Y)
			if found == nil && (x.Op == token.QUO || x.Op == token.REM) {
				if tv := a.info.Types[x.Y]; tv.Value == nil || constant.Sign(tv.Value) == 0 {
					found = x
				}
			}
		case *ast.CallExpr:
			for _, arg := range x.Args {
				walk(arg)
			}
		case *ast.CompositeLit:
			for _, el := range x.Elts {
				if kv, ok := el.(*ast.KeyValueExpr); ok {
					walk(kv.Value)
					continue
				}
				walk(el)
			}
		}
	}
	walk(e)
	return found, found != nil
}

// readsThrough reports whether e is a field read through v itself.
func (a *nonNilAnalysis) readsThrough(e ast.Expr, v *types.Var) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := ast.Unparen(sel.X).(*ast.Ident)
	return ok && a.info.Uses[id] == v
}

// callsSafe reports whether every call of fn passes its parameter v so
// that unwrapping it at the call panics where Go's callee would: with
// every argument after it (every argument, for a receiver) a constant or
// a variable, unless the value passed is itself never nil.
func (a *nonNilAnalysis) callsSafe(fn *types.Func, v *types.Var, plan *nonNilPlan) bool {
	sig := fn.Signature()
	for _, c := range a.calls[fn] {
		// An owned pointer is never lent to a deferred call, nor through a
		// method expression: either shares its type (RFC 0016).
		var passed ast.Expr
		var after []ast.Expr
		if sig.Recv() == v {
			sel := ast.Unparen(c.Fun).(*ast.SelectorExpr) // x.m(...)
			passed, after = sel.X, c.Args
		} else {
			i := 0
			for sig.Params().At(i) != v {
				i++
			}
			passed, after = c.Args[i], c.Args[i+1:] // v is not variadic: a pointer
		}
		if a.info.Types[passed].IsNil() {
			return false // f(nil): Rust would need the None's type spelled out
		}
		if a.isNew(passed, plan) || a.nonNilParam(passed, plan) {
			continue
		}
		for _, e := range after {
			if !a.quiet(e, plan) {
				return false
			}
		}
	}
	return true
}

// nonNilParam reports whether e names a parameter that is never nil.
func (a *nonNilAnalysis) nonNilParam(e ast.Expr, plan *nonNilPlan) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	v, ok := a.info.Uses[id].(*types.Var)
	_, param := a.params[v]
	return ok && param && plan.vars[v]
}

// quiet reports whether evaluating e can neither be seen nor panic: a
// constant, a variable, or a field of a variable that is a struct value or
// a pointer never nil.
func (a *nonNilAnalysis) quiet(e ast.Expr, plan *nonNilPlan) bool {
	if a.info.Types[e].Value != nil {
		return true
	}
	if sel, ok := ast.Unparen(e).(*ast.SelectorExpr); ok {
		s := a.info.Selections[sel]
		base, ok := ast.Unparen(sel.X).(*ast.Ident)
		if s == nil || s.Kind() != types.FieldVal || !ok {
			return false
		}
		v, _ := a.info.Uses[base].(*types.Var)
		return v != nil && (!isPtr(v.Type()) || plan.vars[v])
	}
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	_, isVar := a.info.Uses[id].(*types.Var)
	return isVar || a.info.Types[id].IsNil()
}

// isPtr reports whether t is a pointer type.
func isPtr(t types.Type) bool {
	_, ok := types.Unalias(t).(*types.Pointer)
	return ok
}
