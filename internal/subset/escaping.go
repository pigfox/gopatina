package subset

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sync"

	"github.com/pigfox/gopatina/internal/load"
)

// Escaping closures and function-typed variables (RFC 0018, step 3). A
// function whose result is of function type returns function literals:
// each owns what it captures, moved in, as Box<dyn FnMut>. A local
// variable defined by a call of such a function holds that box; it is only
// called, or passed to a parameter of function type.

type escapePlan struct {
	returned map[*ast.FuncLit]*Closure // returned literals
	vars     map[*types.Var]bool       // function-typed local variables
}

var (
	escapeMu    sync.Mutex
	escapeCache = map[*load.Package]*escapePlan{}
)

func escapes(pkg *load.Package) *escapePlan {
	escapeMu.Lock()
	defer escapeMu.Unlock()
	if p, ok := escapeCache[pkg]; ok {
		return p
	}
	p := &escapePlan{returned: map[*ast.FuncLit]*Closure{}, vars: map[*types.Var]bool{}}
	info := pkg.Info
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			if returnsFunc(info.Defs[fd.Name].(*types.Func).Signature()) {
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					if _, ok := n.(*ast.FuncLit); ok {
						return false // its returns are its own
					}
					if r, ok := n.(*ast.ReturnStmt); ok && len(r.Results) == 1 {
						if lit, ok := ast.Unparen(r.Results[0]).(*ast.FuncLit); ok {
							p.returned[lit] = newClosure(info, lit, nil)
						}
					}
					return true
				})
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				as, ok := n.(*ast.AssignStmt)
				if !ok || as.Tok != token.DEFINE || len(as.Lhs) != len(as.Rhs) {
					return true
				}
				for i, l := range as.Lhs {
					v, _ := info.Defs[l.(*ast.Ident)].(*types.Var)
					if v != nil && isFuncType(v.Type()) && funcCall(info, as.Rhs[i]) {
						p.vars[v] = true
					}
				}
				return true
			})
		}
	}
	escapeCache[pkg] = p
	return p
}

// Returned returns the function literals functions of pkg return, each the
// closure it is.
func Returned(pkg *load.Package) map[*ast.FuncLit]*Closure { return escapes(pkg).returned }

// FuncVars returns pkg's local variables of function type, each defined by
// a call of a function of the program that returns a function.
func FuncVars(pkg *load.Package) map[*types.Var]bool { return escapes(pkg).vars }

// returnsFunc reports whether sig's only result is of function type.
func returnsFunc(sig *types.Signature) bool {
	return sig.Results().Len() == 1 && isFuncType(sig.Results().At(0).Type())
}

// funcCall reports whether e calls a function or method of the program.
func funcCall(info *types.Info, e ast.Expr) bool {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return false
	}
	var fn *types.Func
	switch f := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		fn, _ = info.Uses[f].(*types.Func)
	case *ast.SelectorExpr:
		fn, _ = info.Uses[f.Sel].(*types.Func)
	}
	return fn != nil && fn.Pkg() != nil
}

// funcResult checks a function with a result of function type: the result
// type is one step 2 takes, and every return returns a function literal,
// which owns what it captures (step 3).
func (c *checker) funcResult(fd *ast.FuncDecl, sig *types.Signature) {
	t := sig.Results().At(0).Type().Underlying().(*types.Signature)
	if what := funcTypeProblem(t); what != "" {
		c.unsupported(fd.Name, "a result of function type: "+what)
		return
	}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		r, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		lit, ok := ast.Unparen(r.Results[0]).(*ast.FuncLit)
		if !ok {
			c.unsupported(r, "returning a function other than a function literal (step 3 of RFC 0018)")
			return true
		}
		cl := Returned(c.pkg)[lit]
		if what := closureBodyProblem(c.pkg, cl); what != "" {
			c.unsupported(lit, what)
			return true
		}
		saved, savedLoops := c.inClosure, c.loops
		c.inClosure, c.loops = lit, nil
		c.block(lit.Body)
		c.inClosure, c.loops = saved, savedLoops
		return true
	})
}

// funcVarUses checks the uses of function-typed local variables in fd:
// each is called or passed to a parameter of function type, nothing else
// (one owner: no copy, no reassignment, RFC 0018).
func (c *checker) funcVarUses(fd *ast.FuncDecl) {
	info := c.pkg.Info
	ok := callees(fd.Body)
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if call, isCall := n.(*ast.CallExpr); isCall {
			for i, a := range call.Args {
				if id, isID := ast.Unparen(a).(*ast.Ident); isID && funcParamAt(info, call, i) != nil {
					ok[id] = true
				}
			}
		}
		return true
	})
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		id, isID := n.(*ast.Ident)
		if !isID {
			return true
		}
		if v, _ := info.Uses[id].(*types.Var); FuncVars(c.pkg)[v] && !ok[id] {
			c.unsupported(id, fmt.Sprintf("using the function value %s other than by calling it or passing it to a parameter of function type (step 3 of RFC 0018)", v.Name()))
		}
		return true
	})
}
