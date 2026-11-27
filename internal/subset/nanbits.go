package subset

import (
	"go/ast"
	"go/token"
	"go/types"
)

// NaNObservers are the provided functions that expose a float's bits, so
// that a NaN's payload, which Rust leaves unspecified (RFC 3514) and
// rustc folds to its own NaN where gc computes on the hardware, can be
// seen. TestNaNObservers fails when a provided function exposing float
// bits is missing here. fmt prints every NaN as "NaN".
var NaNObservers = map[string]bool{
	"math.Float64bits": true,
}

// checkNaNBits finds the float operations whose results a NaN observer
// can see (load.Package.NaNExact): those the emitter writes through the
// target's NaN rules (rt::nan_*), so a NaN has gc's bits on every target,
// also where rustc would fold it. It follows a value back through
// constants, locals, parameters (to every call's argument) and the
// program's own functions (to their returns); anything else it cannot
// follow (a field, an element, a global, a pointer, an interface, a
// channel) makes every float operation of the program one an observer may
// see. A + or * whose two operands may both be NaN is refused: gc may
// swap a commutative operation's operands (cmd/compile/internal/ssa,
// cse.go:105-110, orders them by equivalence class; regalloc.go:1731-1749
// swaps amd64's resultInArg0 ones by liveness), and the NaN it keeps is
// then the other's.
func (c *checker) checkNaNBits() {
	n := &nanTaint{c: c, seen: map[ast.Node]bool{}, params: map[*types.Var]paramOf{}, funcs: map[*types.Func]*ast.FuncDecl{}}
	n.index()
	for _, file := range c.pkg.Files {
		ast.Inspect(file, func(x ast.Node) bool {
			if call, ok := x.(*ast.CallExpr); ok && NaNObservers[c.stdName(call.Fun)] {
				for _, a := range call.Args {
					n.value(a)
				}
			}
			return true
		})
	}
	exact := map[ast.Node]bool{}
	if n.all {
		for _, file := range c.pkg.Files {
			ast.Inspect(file, func(x ast.Node) bool {
				if op := n.op(x); op != nil {
					exact[x] = true
				}
				return true
			})
		}
	} else {
		exact = n.ops
	}
	for _, f := range c.pkg.Fusions {
		delete(exact, f.Mul) // rounded once with its add (rt::fma_*), which keeps gc's NaN
	}
	c.pkg.NaNExact = exact
	for x := range exact {
		if e, ok := x.(*ast.BinaryExpr); ok && (e.Op == token.ADD || e.Op == token.MUL) && c.pkg.Fusions[e].Mul == nil && c.mayBeNaN(e.X) && c.mayBeNaN(e.Y) {
			c.unsupported(e, "a float "+e.Op.String()+" of two values that may both be NaN, whose bits math.Float64bits may read (gc may swap the operands, and keep the other NaN),")
		}
		if as, ok := x.(*ast.AssignStmt); ok && (as.Tok == token.ADD_ASSIGN || as.Tok == token.MUL_ASSIGN) && c.mayBeNaN(as.Rhs[0]) {
			c.unsupported(as, "a float "+as.Tok.String()+" of two values that may both be NaN, whose bits math.Float64bits may read (gc may swap the operands, and keep the other NaN),")
		}
	}
}

// mayBeNaN reports whether e may be NaN: not a constant (Go has no NaN
// constants) or an integer converted to a float.
func (c *checker) mayBeNaN(e ast.Expr) bool {
	e = ast.Unparen(e)
	if c.pkg.Info.Types[e].Value != nil {
		return false
	}
	if call, ok := e.(*ast.CallExpr); ok && c.pkg.Info.Types[call.Fun].IsType() && len(call.Args) == 1 {
		if b, ok := c.pkg.Info.Types[call.Args[0]].Type.Underlying().(*types.Basic); ok && b.Info()&types.IsInteger != 0 {
			return false
		}
	}
	return true
}

// stdName is the path.Name of fun when it names a standard-library
// function, or "".
func (c *checker) stdName(fun ast.Expr) string {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	pn, ok := c.pkg.Info.Uses[id].(*types.PkgName)
	if !ok || c.pkg.Local[pn.Imported()] {
		return ""
	}
	return pn.Imported().Path() + "." + sel.Sel.Name
}

// paramOf is a function's parameter: the function and its index.
type paramOf struct {
	fn  *types.Func
	idx int
}

// nanTaint follows the values a NaN observer sees.
type nanTaint struct {
	c      *checker
	seen   map[ast.Node]bool
	ops    map[ast.Node]bool // the float operations an observer may see
	all    bool              // a value it cannot follow: every float operation
	params map[*types.Var]paramOf
	funcs  map[*types.Func]*ast.FuncDecl
	calls  map[*types.Func][]*ast.CallExpr
}

// index records the program's functions, their parameters and calls.
func (n *nanTaint) index() {
	info := n.c.pkg.Info
	n.ops = map[ast.Node]bool{}
	n.calls = map[*types.Func][]*ast.CallExpr{}
	for _, file := range n.c.pkg.Files {
		for _, d := range file.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			obj := info.Defs[fd.Name].(*types.Func)
			n.funcs[obj] = fd
			sig := obj.Signature()
			for i := 0; i < sig.Params().Len(); i++ {
				n.params[sig.Params().At(i)] = paramOf{obj, i}
			}
		}
		ast.Inspect(file, func(x ast.Node) bool {
			if call, ok := x.(*ast.CallExpr); ok {
				if f := n.callee(call); f != nil {
					n.calls[f] = append(n.calls[f], call)
				}
			}
			return true
		})
	}
}

// callee is the program's function call calls, or nil.
func (n *nanTaint) callee(call *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch f := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	default:
		return nil
	}
	f, ok := n.c.pkg.Info.Uses[id].(*types.Func)
	if !ok || n.funcs[f] == nil {
		return nil
	}
	return f
}

// op returns x when it is a float operation that may make a NaN: + - *
// / (also as an assignment), a conversion between float types, or a
// provided math function that computes (Sqrt); nil otherwise.
func (n *nanTaint) op(x ast.Node) ast.Node {
	info := n.c.pkg.Info
	isFloat := func(e ast.Expr) bool {
		b, ok := info.Types[e].Type.Underlying().(*types.Basic)
		return ok && b.Info()&types.IsFloat != 0
	}
	switch e := x.(type) {
	case *ast.BinaryExpr:
		switch e.Op {
		case token.ADD, token.SUB, token.MUL, token.QUO:
			if isFloat(e) && info.Types[e].Value == nil {
				return e
			}
		}
	case *ast.AssignStmt:
		switch e.Tok {
		case token.ADD_ASSIGN, token.SUB_ASSIGN, token.MUL_ASSIGN, token.QUO_ASSIGN:
			if isFloat(e.Lhs[0]) {
				return e
			}
		}
	case *ast.CallExpr:
		if info.Types[e.Fun].IsType() && len(e.Args) == 1 && isFloat(e) && isFloat(e.Args[0]) && info.Types[e].Value == nil {
			return e
		}
		if n.c.stdName(e.Fun) == "math.Sqrt" {
			return e
		}
	}
	return nil
}

// value follows e, a value an observer sees, to the operations that make
// it.
func (n *nanTaint) value(e ast.Expr) {
	if n.all || n.seen[e] {
		return
	}
	n.seen[e] = true
	info := n.c.pkg.Info
	if tv := info.Types[e]; tv.Value != nil {
		return // a constant: never NaN
	}
	if b, ok := info.Types[e].Type.Underlying().(*types.Basic); ok && b.Info()&types.IsFloat == 0 {
		return // an integer's bits are its own: Float64bits(Float64frombits(u)) needs no float operation
	}
	if op := n.op(e); op != nil {
		n.ops[op] = true
	}
	switch x := e.(type) {
	case *ast.ParenExpr:
		n.value(x.X)
	case *ast.BinaryExpr:
		n.value(x.X)
		n.value(x.Y)
	case *ast.UnaryExpr:
		n.value(x.X) // negation flips the sign bit, NaN or not
	case *ast.CallExpr:
		switch {
		case info.Types[x.Fun].IsType():
			n.value(x.Args[0])
		case n.c.stdName(x.Fun) != "":
			for _, a := range x.Args {
				n.value(a) // math's: Sqrt, and Abs, Copysign, Floor... which keep a NaN's payload
			}
		case n.callee(x) != nil:
			n.returns(n.callee(x))
		default:
			n.all = true
		}
	case *ast.Ident:
		n.ident(x)
	default:
		n.all = true // a field, an element, a pointer, an interface, a receive
	}
}

// ident follows a variable: a parameter to every call's argument, a local
// to every value assigned it, anything else (a global) to all.
func (n *nanTaint) ident(id *ast.Ident) {
	v, _ := n.c.pkg.Info.Uses[id].(*types.Var) // a constant has a value: value stopped
	if p, ok := n.params[v]; ok {
		for _, call := range n.calls[p.fn] {
			if p.idx < len(call.Args) {
				n.value(call.Args[p.idx])
			}
		}
		return
	}
	fd := n.funcOf(id)
	if v.Parent() == v.Pkg().Scope() || fd == nil {
		n.all = true // a package-level variable, or one of a function literal there
		return
	}
	is := func(e ast.Expr) bool {
		x, ok := ast.Unparen(e).(*ast.Ident)
		return ok && (n.c.pkg.Info.Defs[x] == v || n.c.pkg.Info.Uses[x] == v)
	}
	ast.Inspect(fd, func(x ast.Node) bool {
		switch s := x.(type) {
		case *ast.AssignStmt:
			for i, l := range s.Lhs {
				if !is(l) {
					continue
				}
				if op := n.op(s); op != nil {
					n.ops[op] = true
					n.value(s.Rhs[0])
				} else if len(s.Lhs) == len(s.Rhs) {
					n.value(s.Rhs[i])
				} else {
					n.all = true // v, err := f(): a result of several
				}
			}
		case *ast.ValueSpec:
			for i, name := range s.Names {
				if is(name) && i < len(s.Values) {
					n.value(s.Values[i])
				}
			}
		case *ast.RangeStmt:
			if (s.Key != nil && is(s.Key)) || (s.Value != nil && is(s.Value)) {
				n.all = true // an element
			}
		case *ast.UnaryExpr:
			if s.Op == token.AND && is(s.X) {
				n.all = true // it may change through its address
			}
		}
		return true
	})
}

// returns follows the values function f returns.
func (n *nanTaint) returns(f *types.Func) {
	fd := n.funcs[f]
	if n.seen[fd] {
		return
	}
	n.seen[fd] = true
	ast.Inspect(fd.Body, func(x ast.Node) bool {
		if _, ok := x.(*ast.FuncLit); ok {
			return false
		}
		if r, ok := x.(*ast.ReturnStmt); ok {
			if len(r.Results) == 0 {
				n.all = true // a bare return: its named results
			}
			for _, e := range r.Results {
				n.value(e)
			}
		}
		return true
	})
}

// funcOf is the function declaring id's use.
func (n *nanTaint) funcOf(id *ast.Ident) *ast.FuncDecl {
	for _, fd := range n.funcs {
		if fd.Pos() <= id.Pos() && id.Pos() < fd.End() {
			return fd
		}
	}
	return nil
}
