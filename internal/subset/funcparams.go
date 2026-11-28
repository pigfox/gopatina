package subset

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sync"

	"github.com/pigfox/gopatina/internal/load"
)

// A FuncParam is a parameter of function type (RFC 0018, step 2): a
// generic impl Fn in Rust, or impl FnMut when a closure passed to it at
// some call changes what it captures (Mut).
type FuncParam struct {
	Var *types.Var
	Mut bool
}

// Inline returns the closure of a function literal written as an argument
// of a function-typed parameter, by its literal (RFC 0018, step 2): Var is
// nil.
func Inline(pkg *load.Package) map[*ast.FuncLit]*Closure {
	return plan(pkg).inline
}

// FuncParams returns the parameters of function type of pkg's functions
// and methods, by variable.
func FuncParams(pkg *load.Package) map[*types.Var]*FuncParam {
	return plan(pkg).params
}

type funcPlan struct {
	inline map[*ast.FuncLit]*Closure
	params map[*types.Var]*FuncParam
}

var (
	funcPlanMu    sync.Mutex
	funcPlanCache = map[*load.Package]*funcPlan{}
)

func plan(pkg *load.Package) *funcPlan {
	funcPlanMu.Lock()
	defer funcPlanMu.Unlock()
	if p, ok := funcPlanCache[pkg]; ok {
		return p
	}
	p := &funcPlan{inline: map[*ast.FuncLit]*Closure{}, params: map[*types.Var]*FuncParam{}}
	info := pkg.Info
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			sig := info.Defs[fd.Name].(*types.Func).Signature()
			for i := 0; i < sig.Params().Len(); i++ {
				if v := sig.Params().At(i); isFuncType(v.Type()) {
					p.params[v] = &FuncParam{Var: v}
				}
			}
		}
	}
	for _, f := range pkg.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			for i, a := range call.Args {
				param := funcParamAt(info, call, i)
				if param == nil {
					continue
				}
				var writes bool
				switch a := ast.Unparen(a).(type) {
				case *ast.FuncLit:
					cl := newClosure(info, a, nil)
					p.inline[a] = cl
					writes = len(cl.Writes) > 0
				case *ast.Ident:
					if v, ok := info.Uses[a].(*types.Var); ok {
						if cl := Closures(pkg)[v]; cl != nil {
							writes = len(cl.Writes) > 0
						}
					}
				}
				if fp := p.params[param]; fp != nil && writes {
					fp.Mut = true
				}
			}
			return true
		})
	}
	funcPlanCache[pkg] = p
	return p
}

// funcParamAt returns call's parameter i when it is of function type and
// the callee is a function or method of the program, or nil.
func funcParamAt(info *types.Info, call *ast.CallExpr, i int) *types.Var {
	var fn *types.Func
	switch f := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		fn, _ = info.Uses[f].(*types.Func)
	case *ast.SelectorExpr:
		fn, _ = info.Uses[f.Sel].(*types.Func)
	}
	if fn == nil {
		return nil
	}
	sig := fn.Signature()
	if i >= sig.Params().Len() || !isFuncType(sig.Params().At(i).Type()) {
		return nil
	}
	return sig.Params().At(i)
}

// isFuncType reports whether t is a function type.
func isFuncType(t types.Type) bool {
	_, ok := t.Underlying().(*types.Signature)
	return ok
}

// funcTypeProblem says why a parameter of function type sig is not
// supported yet (step 2 of RFC 0018 takes the types step 1 does).
func funcTypeProblem(sig *types.Signature) string {
	if sig.Variadic() {
		return "a parameter of function type with a ... parameter"
	}
	for i := 0; i < sig.Params().Len(); i++ {
		if !isCopyBasic(sig.Params().At(i).Type()) {
			return fmt.Sprintf("a parameter of function type taking %s (step 2 of RFC 0018 takes numbers and bools)", sig.Params().At(i).Type())
		}
	}
	if r := sig.Results(); r.Len() > 1 || r.Len() == 1 && !isCopyBasic(r.At(0).Type()) && !isString(r.At(0).Type()) {
		return fmt.Sprintf("a parameter of function type with results %s (step 2 of RFC 0018 takes one number, bool or string)", r)
	}
	return ""
}

// funcParamUses checks that the body of fd only calls its parameters of
// function type: it does not store, pass on, compare or return them.
func (c *checker) funcParamUses(fd *ast.FuncDecl) {
	calls := callees(fd.Body)
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		v, _ := c.pkg.Info.Uses[id].(*types.Var)
		if FuncParams(c.pkg)[v] != nil && !calls[id] {
			c.unsupported(id, fmt.Sprintf("using the function parameter %s other than by calling it (step 2 of RFC 0018)", v.Name()))
		}
		return true
	})
	// f(f(x)) with an impl FnMut borrows f twice at once in Rust.
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := ast.Unparen(call.Fun).(*ast.Ident)
		if !ok {
			return true
		}
		v, _ := c.pkg.Info.Uses[id].(*types.Var)
		if fp := FuncParams(c.pkg)[v]; fp != nil && fp.Mut {
			for _, a := range call.Args {
				if callees(a)[firstUse(c.pkg.Info, a, v)] {
					c.unsupported(call, fmt.Sprintf("calling %s inside the arguments of a call of %s, when it is passed a closure that changes what it captures", v.Name(), v.Name()))
					return false
				}
			}
		}
		return true
	})
}

// firstUse returns the first identifier in n that names v, or nil.
func firstUse(info *types.Info, n ast.Node, v *types.Var) *ast.Ident {
	var out *ast.Ident
	ast.Inspect(n, func(x ast.Node) bool {
		if id, ok := x.(*ast.Ident); ok && out == nil && info.Uses[id] == v {
			out = id
		}
		return out == nil
	})
	return out
}

// callees returns the identifiers called in n.
func callees(n ast.Node) map[*ast.Ident]bool {
	out := map[*ast.Ident]bool{}
	ast.Inspect(n, func(x ast.Node) bool {
		if call, ok := x.(*ast.CallExpr); ok {
			if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
				out[id] = true
			}
		}
		return true
	})
	return out
}

// programArgs checks the arguments of a call of a function of the program:
// one for a parameter of function type is a function of the program, a
// closure, or a function literal (RFC 0018, step 2).
func (c *checker) programArgs(e *ast.CallExpr) {
	info := c.pkg.Info
	for i, a := range e.Args {
		if funcParamAt(info, e, i) == nil {
			c.expr(a)
			continue
		}
		switch a := ast.Unparen(a).(type) {
		case *ast.Ident:
			switch u := info.Uses[a].(type) {
			case *types.Func:
				if u.Signature().Recv() == nil && c.pkg.Local[u.Pkg()] {
					continue // a function of the program
				}
			case *types.Var:
				if Closures(c.pkg)[u] != nil {
					continue // a closure (its uses are checked with it)
				}
				if FuncParams(c.pkg)[u] != nil {
					continue // refused where it is used (funcParamUses)
				}
			}
		case *ast.FuncLit:
			c.inlineClosure(e, a)
			continue
		}
		c.unsupported(a, "a function argument other than a function of the program, a closure or a function literal (step 2 of RFC 0018)")
	}
}

// inlineClosure checks a function literal passed to call: as a closure of
// step 1, alive for the statement it is in, which must be the call itself,
// an assignment of it or a return of it, in a block (the crate binds the
// closure just before the statement).
func (c *checker) inlineClosure(call *ast.CallExpr, lit *ast.FuncLit) {
	stmt := enclosingStmt(c.body, call)
	if stmt == nil {
		c.unsupported(lit, "a function literal passed in a call that is not a statement of its own, an assignment or a return (step 2 of RFC 0018)")
		return
	}
	cl := Inline(c.pkg)[lit]
	if what := closureBodyProblem(c.pkg, cl); what != "" {
		c.unsupported(lit, what)
		return
	}
	if what := inlineUseProblem(c.pkg.Info, cl, stmt); what != "" {
		c.unsupported(lit, what)
		return
	}
	saved, savedLoops := c.inClosure, c.loops
	c.inClosure, c.loops = lit, nil
	c.block(lit.Body)
	c.inClosure, c.loops = saved, savedLoops
}

// enclosingStmt returns the statement of body, directly in a block, whose
// whole expression is call, an assignment's value, or a returned value; or
// nil.
func enclosingStmt(body *ast.BlockStmt, call *ast.CallExpr) ast.Stmt {
	var found ast.Stmt
	ast.Inspect(body, func(n ast.Node) bool {
		b, ok := n.(*ast.BlockStmt)
		if !ok || found != nil {
			return found == nil
		}
		for _, s := range b.List {
			var es []ast.Expr
			switch s := s.(type) {
			case *ast.ExprStmt:
				es = []ast.Expr{s.X}
			case *ast.AssignStmt:
				es = s.Rhs
			case *ast.ReturnStmt:
				es = s.Results
			}
			for _, e := range es {
				if ast.Unparen(e) == call {
					found = s
				}
			}
		}
		return true
	})
	return found
}

// inlineUseProblem checks the statement a passed function literal is alive
// in: it may not use, outside the literal, a variable the literal changes,
// nor change one it reads.
func inlineUseProblem(info *types.Info, cl *Closure, stmt ast.Stmt) string {
	written := map[*ast.Ident]bool{}
	ast.Inspect(stmt, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, l := range n.Lhs {
				if id, ok := ast.Unparen(l).(*ast.Ident); ok {
					written[id] = true
				}
			}
		case *ast.UnaryExpr:
			if id, ok := ast.Unparen(n.X).(*ast.Ident); ok && n.Op == token.AND {
				written[id] = true
			}
		}
		return true
	})
	problem := ""
	ast.Inspect(stmt, func(n ast.Node) bool {
		if n == cl.Lit {
			return false
		}
		id, ok := n.(*ast.Ident)
		if !ok || !cl.captures(info, id) {
			return true
		}
		v := info.Uses[id].(*types.Var)
		switch {
		case cl.Writes[v]:
			problem = fmt.Sprintf("%s is used in the statement that passes a function literal which changes it", v.Name())
		case written[id]:
			problem = fmt.Sprintf("%s is changed in the statement that passes a function literal which reads it", v.Name())
		}
		return problem == ""
	})
	return problem
}
