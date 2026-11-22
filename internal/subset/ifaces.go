package subset

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"

	"github.com/pigfox/gopatina/internal/load"
)

// IfacePlan is the whole program's interface analysis (RFC 0012): every
// expression whose value is converted to an interface type, and the
// program's dynamic types, the types of those values. The emitter writes
// one enum with a variant per dynamic type; an interface value is that
// enum behind a reference count, or nothing for a nil interface.
type IfacePlan struct {
	// Boxed maps each converted expression to its concrete type.
	Boxed map[ast.Expr]types.Type
	// Dyn lists the dynamic types in the order the program first converts
	// them, so the output is deterministic.
	Dyn []types.Type
	// CommaOk marks the type assertions of v, ok := x.(T).
	CommaOk map[*ast.TypeAssertExpr]bool
}

// IsIface reports whether t is an interface type handled as an interface
// value (RFC 0012): error too, except where it is lowered to Result; not
// io.Writer, which phase 1 lowers on its own.
func IsIface(t types.Type) bool {
	return t != nil && types.IsInterface(t) && !isWriter(t)
}

// Ifaces analyses pkg, every package of the program, for RFC 0012.
func Ifaces(pkg *load.Package) *IfacePlan {
	p := &IfacePlan{Boxed: map[ast.Expr]types.Type{}, CommaOk: map[*ast.TypeAssertExpr]bool{}}
	info := pkg.Info
	conv := func(e ast.Expr, target types.Type) {
		if !IsIface(target) {
			return
		}
		tv := info.Types[e]
		if tv.Type == nil || tv.IsNil() || types.IsInterface(tv.Type) {
			return // nil, or already an interface: nothing to box
		}
		p.Boxed[e] = tv.Type
		for _, d := range p.Dyn {
			if types.Identical(d, tv.Type) {
				return
			}
		}
		p.Dyn = append(p.Dyn, tv.Type)
	}
	for _, f := range pkg.Files {
		var results []*types.Tuple // the enclosing functions' results
		var walk func(n ast.Node) bool
		walk = func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				fn, _ := info.Defs[n.Name].(*types.Func)
				if fn == nil || n.Body == nil {
					return false
				}
				results = append(results, fn.Signature().Results())
				ast.Inspect(n.Body, walk)
				results = results[:len(results)-1]
				return false
			case *ast.AssignStmt:
				if len(n.Lhs) == len(n.Rhs) && (n.Tok == token.ASSIGN || n.Tok == token.DEFINE) {
					for i, l := range n.Lhs {
						conv(n.Rhs[i], info.TypeOf(l))
					}
				}
				if len(n.Lhs) == 2 && len(n.Rhs) == 1 {
					if ta, ok := ast.Unparen(n.Rhs[0]).(*ast.TypeAssertExpr); ok {
						p.CommaOk[ta] = true
					}
				}
			case *ast.ValueSpec:
				if n.Type != nil && len(n.Values) == len(n.Names) {
					for _, v := range n.Values {
						conv(v, info.TypeOf(n.Type))
					}
				}
				if len(n.Names) == 2 && len(n.Values) == 1 {
					if ta, ok := ast.Unparen(n.Values[0]).(*ast.TypeAssertExpr); ok {
						p.CommaOk[ta] = true
					}
				}
			case *ast.ReturnStmt:
				if len(results) > 0 {
					res := results[len(results)-1]
					if res.Len() == len(n.Results) {
						for i, r := range n.Results {
							conv(r, res.At(i).Type())
						}
					}
				}
			case *ast.CallExpr:
				p.call(info, n, conv)
			case *ast.CompositeLit:
				switch u := info.TypeOf(n).Underlying().(type) {
				case *types.Slice:
					for _, e := range n.Elts {
						conv(e, u.Elem())
					}
				case *types.Struct:
					for i, e := range n.Elts {
						if kv, ok := e.(*ast.KeyValueExpr); ok {
							if v, ok := info.Uses[kv.Key.(*ast.Ident)].(*types.Var); ok {
								conv(kv.Value, v.Type())
							}
						} else if i < u.NumFields() {
							conv(e, u.Field(i).Type())
						}
					}
				}
			case *ast.BinaryExpr:
				if n.Op == token.EQL || n.Op == token.NEQ {
					conv(n.X, info.TypeOf(n.Y))
					conv(n.Y, info.TypeOf(n.X))
				}
			}
			return true
		}
		ast.Inspect(f, walk)
	}
	return p
}

// call records the conversions of a call's arguments: to the parameters of
// a function or method of the program, or of an explicit conversion
// any(x); append's elements. The standard library's functions (fmt's
// ...any) take their arguments as they are.
func (p *IfacePlan) call(info *types.Info, e *ast.CallExpr, conv func(ast.Expr, types.Type)) {
	fun := ast.Unparen(e.Fun)
	if tv := info.Types[fun]; tv.IsType() {
		if len(e.Args) == 1 {
			conv(e.Args[0], tv.Type)
		}
		return
	}
	if id, ok := fun.(*ast.Ident); ok {
		if b, ok := info.Uses[id].(*types.Builtin); ok {
			if b.Name() == "append" && len(e.Args) > 1 && !e.Ellipsis.IsValid() {
				if s, ok := info.TypeOf(e.Args[0]).Underlying().(*types.Slice); ok {
					for _, a := range e.Args[1:] {
						conv(a, s.Elem())
					}
				}
			}
			return
		}
	}
	if sel, ok := fun.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok {
			if _, ok := info.Uses[id].(*types.PkgName); ok {
				if fn, ok := info.Uses[sel.Sel].(*types.Func); ok && fn.Pkg() != nil && !localPkgs[fn.Pkg()] {
					if fn.Pkg().Path() == "errors" && fn.Name() != "New" {
						// errors.Is, As and Unwrap take errors (As's target
						// is a pointer, not converted).
						conv(e.Args[0], fn.Signature().Params().At(0).Type())
						if fn.Name() == "Is" {
							conv(e.Args[1], fn.Signature().Params().At(1).Type())
						}
					}
					return // fmt and the others take their arguments as they are
				}
			}
		}
	}
	sig, ok := info.TypeOf(fun).Underlying().(*types.Signature)
	if !ok || e.Ellipsis.IsValid() {
		return
	}
	for i, a := range e.Args {
		var t types.Type
		switch {
		case sig.Variadic() && i >= sig.Params().Len()-1:
			t = sig.Params().At(sig.Params().Len() - 1).Type().(*types.Slice).Elem()
		case i < sig.Params().Len():
			t = sig.Params().At(i).Type()
		}
		conv(a, t)
	}
}

// interfaceProblem says why patina mode cannot represent interface type t
// yet, or returns "".
func interfaceProblem(t *types.Interface) string {
	if !t.IsMethodSet() {
		return "an interface with a type set (a generic constraint)"
	}
	for i := 0; i < t.NumMethods(); i++ {
		sig := t.Method(i).Signature()
		if sig.Variadic() {
			return "a variadic interface method"
		}
		for j := 0; j < sig.Params().Len(); j++ {
			pt := sig.Params().At(j).Type()
			switch {
			case isWriter(pt):
				return "an interface method with an io.Writer parameter"
			case isPointer(pt):
				return "an interface method with a pointer parameter"
			}
			if holdsSlice(pt) {
				// The callee's copy would share the slice in Go; the
				// dispatcher's copy would not.
				return "an interface method with a slice parameter"
			}
			if what := unsupportedType(pt, false); what != "" {
				return what
			}
		}
		for j := 0; j < sig.Results().Len(); j++ {
			rt := sig.Results().At(j).Type()
			switch {
			case isError(rt) || isWriter(rt):
				return "an interface method with an error result"
			case isPointer(rt):
				return "an interface method with a pointer result"
			}
			if what := unsupportedType(rt, false); what != "" {
				return what
			}
		}
	}
	return ""
}

// dynamicProblem says why a value of type t cannot be converted to an
// interface yet, or returns "".
func (c *checker) dynamicProblem(t types.Type) string {
	switch {
	case isPointer(t):
		// *T for a named type T of the program (RFC 0013): a heap pointer.
		if n, ok := types.Unalias(pointee(t)).(*types.Named); !ok || isPointer(t.(*types.Pointer).Elem()) || n.Obj().Pkg() == nil || !localPkgs[n.Obj().Pkg()] {
			return "converting a pointer to a type other than a named type of the program to an interface"
		}
		return ""
	case isWriter(t): // a type switch case or an assertion
		return "io.Writer as a type switch case or assertion type"
	case providedName(t) != "":
		return "converting a value of type " + t.String() + " to an interface"
	}
	if m := PrintMethod(t); m != nil {
		if what := c.methodPrintProblem(m); what != "" {
			return "converting to an interface a value whose " + m.Name() + " method may panic"
		}
	}
	if obj, _, _ := types.LookupFieldOrMethod(t, false, nil, "Unwrap"); obj != nil {
		if f, ok := obj.(*types.Func); ok && f.Signature().Results().Len() == 1 {
			if _, ok := f.Signature().Results().At(0).Type().(*types.Slice); ok {
				return "an error type with Unwrap() []error (errors.Join's form)"
			}
		}
	}
	if st, ok := t.Underlying().(*types.Struct); ok && PrintMethod(t) == nil {
		// Printed field by field (a type with its own String or Error is
		// printed through it).
		for i := 0; i < st.NumFields(); i++ {
			if PrintMethod(st.Field(i).Type()) != nil || IsIface(st.Field(i).Type()) {
				return "converting to an interface a struct with a field fmt prints through a method"
			}
		}
	}
	return unsupportedType(t, false)
}

// checkIfaces reports the conversions the plan found that patina mode
// cannot represent; each type is reported once, where it is first
// converted.
func (c *checker) checkIfaces() {
	if c.ifacePlan == nil {
		c.ifacePlan = Ifaces(c.pkg)
	}
	plan := c.ifacePlan
	var es []ast.Expr
	for e := range plan.Boxed {
		es = append(es, e)
	}
	sort.Slice(es, func(i, j int) bool { return es[i].Pos() < es[j].Pos() })
	seen := map[string]bool{}
	for _, e := range es {
		t := plan.Boxed[e]
		if what := c.dynamicProblem(t); what != "" && !seen[t.String()] {
			seen[t.String()] = true
			c.unsupported(e, what)
		}
	}
}

// typeAssert checks x.(T).
func (c *checker) typeAssert(e *ast.TypeAssertExpr) {
	c.expr(e.X) // go/types allows x.(type) only in a type switch
	t := c.pkg.Info.TypeOf(e.Type)
	if what := c.assertProblem(t); what != "" {
		c.unsupported(e.Type, what)
	}
}

// assertProblem says why a type assertion or type switch case to t is not
// supported, or returns "".
func (c *checker) assertProblem(t types.Type) string {
	if IsIface(t) {
		return unsupportedType(t, false)
	}
	return c.dynamicProblem(t)
}

// typeSwitch checks switch [v :=] x.(type) { ... }.
func (c *checker) typeSwitch(s *ast.TypeSwitchStmt) {
	if s.Init != nil {
		c.unsupported(s.Init, "a type switch with an init statement")
	}
	var x ast.Expr
	switch a := s.Assign.(type) {
	case *ast.AssignStmt:
		x = a.Rhs[0].(*ast.TypeAssertExpr).X
	case *ast.ExprStmt:
		x = a.X.(*ast.TypeAssertExpr).X
	}
	c.expr(x)
	for _, cl := range s.Body.List {
		cc := cl.(*ast.CaseClause)
		for _, t := range cc.List {
			if tv := c.pkg.Info.Types[t]; !tv.IsNil() {
				if what := c.assertProblem(tv.Type); what != "" {
					c.unsupported(t, what)
				}
			}
		}
		if n := len(cc.Body); n > 0 {
			if b, ok := cc.Body[n-1].(*ast.BranchStmt); ok {
				c.trailing[b] = true
			}
		}
		c.loops = append(c.loops, true)
		for _, st := range cc.Body {
			c.stmt(st)
		}
		c.loops = c.loops[:len(c.loops)-1]
	}
}

// holdsSlice reports whether a value of type t holds a slice, directly or
// in a field.
func holdsSlice(t types.Type) bool {
	switch u := t.Underlying().(type) {
	case *types.Slice:
		return true
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if holdsSlice(u.Field(i).Type()) {
				return true
			}
		}
	}
	return false
}

// holdsIface reports whether a value of type t holds an interface value,
// directly, in a field or in a slice.
func holdsIface(t types.Type) bool {
	if IsIface(t) {
		return true
	}
	switch u := t.Underlying().(type) {
	case *types.Slice:
		return holdsIface(u.Elem())
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if holdsIface(u.Field(i).Type()) {
				return true
			}
		}
	}
	return false
}

// stringErrors reports whether the program converts a type of string kind
// to an interface: fmt.Print would space such an error as a string.
// (printProblem, called first for every operand, has made the plan.)
func (c *checker) stringErrors() bool {
	for _, t := range c.ifacePlan.Dyn {
		if b, ok := t.Underlying().(*types.Basic); ok && b.Info()&types.IsString != 0 {
			return true
		}
	}
	return false
}

// wrapProblem says why %w at operand a of a call of fn is not supported,
// or returns "": only in fmt.Errorf, once, plain, of a variable of type
// error.
func (c *checker) wrapProblem(v Verb, a ast.Expr, fn string, n int) string {
	_, isVar := ast.Unparen(a).(*ast.Ident)
	switch {
	case fn != "fmt.Errorf":
		return "%w outside fmt.Errorf"
	case n > 1:
		return "more than one %w in a format (errors.Join's form)"
	case !isError(c.pkg.Info.TypeOf(a)):
		return "%w of a value that is not of type error"
	case v.Width >= 0 || v.Prec >= 0 || v.Plus || v.Minus || v.Zero || v.Space || v.Sharp:
		return fmt.Sprintf("the format %s", v.Spec)
	case !isVar:
		return "%w of an expression other than a variable (compute it into one first)"
	}
	return ""
}

// errorsAs checks errors.As(err, &target): target a local variable whose
// type is an interface or a type that implements error (Go panics on any
// other), and no type of the program with an As method, which errors.As
// would call.
func (c *checker) errorsAs(e *ast.CallExpr) {
	info := c.pkg.Info
	u, ok := ast.Unparen(e.Args[1]).(*ast.UnaryExpr)
	var id *ast.Ident
	if ok && u.Op == token.AND {
		id, _ = ast.Unparen(u.X).(*ast.Ident)
	}
	v, _ := info.Uses[id].(*types.Var)
	if id == nil || v == nil || v.Parent() == v.Pkg().Scope() {
		c.unsupported(e.Args[1], "an errors.As target other than &v for a local variable v")
		return
	}
	t := v.Type()
	if !types.IsInterface(t) && !types.Implements(t, types.Universe.Lookup("error").Type().Underlying().(*types.Interface)) {
		c.unsupported(e.Args[1], "an errors.As target whose type is neither an interface nor an error type (Go panics)")
		return
	}
	if what := c.assertProblem(t); what != "" {
		c.unsupported(e.Args[1], what)
		return
	}
	if c.ifacePlan == nil {
		c.ifacePlan = Ifaces(c.pkg)
	}
	for _, d := range c.ifacePlan.Dyn {
		if obj, _, _ := types.LookupFieldOrMethod(d, false, nil, "As"); obj != nil {
			c.unsupported(e, "errors.As in a program with an error type that has an As method")
			return
		}
	}
}
