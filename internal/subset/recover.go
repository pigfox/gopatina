package subset

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sync"

	"github.com/pigfox/gopatina/internal/load"
)

// recover (RFC 0020): called directly by a deferred function literal, it
// returns the value of the panic being recovered, as %v prints it, and stops
// the panic. Its value is an Option<String> in Rust, so it is only compared
// with nil, printed, or held in a variable that is.

type recoverPlan struct {
	funcs map[*types.Func]bool // the functions that defer a literal calling recover
	vars  map[*types.Var]bool  // the variables r := recover() defines
}

var (
	recoverMu    sync.Mutex
	recoverCache = map[*load.Package]*recoverPlan{}
)

func recovers(pkg *load.Package) *recoverPlan {
	recoverMu.Lock()
	defer recoverMu.Unlock()
	if p, ok := recoverCache[pkg]; ok {
		return p
	}
	p := &recoverPlan{funcs: map[*types.Func]bool{}, vars: map[*types.Var]bool{}}
	info := pkg.Info
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				ds, ok := n.(*ast.DeferStmt)
				if !ok {
					return true
				}
				lit, ok := ast.Unparen(ds.Call.Fun).(*ast.FuncLit)
				if !ok {
					return true
				}
				ast.Inspect(lit.Body, func(m ast.Node) bool {
					switch m := m.(type) {
					case *ast.CallExpr:
						if isRecover(info, m) {
							p.funcs[info.Defs[fd.Name].(*types.Func)] = true
						}
					case *ast.AssignStmt:
						if m.Tok == token.DEFINE && len(m.Lhs) == 1 && len(m.Rhs) == 1 && isRecover(info, m.Rhs[0]) {
							if v, ok := info.Defs[m.Lhs[0].(*ast.Ident)].(*types.Var); ok {
								p.vars[v] = true
							}
						}
					}
					return true
				})
				return true
			})
		}
	}
	recoverCache[pkg] = p
	return p
}

// Recovering returns the functions of pkg that defer a function literal
// calling recover: each runs its body under rt::recovering.
func Recovering(pkg *load.Package) map[*types.Func]bool { return recovers(pkg).funcs }

// RecoverVars returns the variables r := recover() defines.
func RecoverVars(pkg *load.Package) map[*types.Var]bool { return recovers(pkg).vars }

// isRecover reports whether e is a call of the recover built-in.
func isRecover(info *types.Info, e ast.Expr) bool {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	b, ok := info.Uses[id].(*types.Builtin)
	return ok && b.Name() == "recover"
}

// IsRecoverValue reports whether e is recover() or a variable holding its
// value: an Option<String>.
func IsRecoverValue(pkg *load.Package, e ast.Expr) bool {
	if isRecover(pkg.Info, e) {
		return true
	}
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	v, _ := pkg.Info.Uses[id].(*types.Var)
	return RecoverVars(pkg)[v]
}

// recoverUseProblem checks the recover calls and recover values in the
// body of deferred literal lit: each is compared with nil, printed, or (a
// call) the value of r := recover() or a statement of its own.
func recoverUseProblem(pkg *load.Package, lit *ast.FuncLit) string {
	info := pkg.Info
	var stack []ast.Node
	problem := ""
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		var parent ast.Node
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		stack = append(stack, n)
		e, ok := n.(ast.Expr)
		if !ok || !IsRecoverValue(pkg, e) {
			return problem == ""
		}
		switch p := parent.(type) {
		case *ast.ExprStmt:
			if isRecover(info, e) {
				return true
			}
		case *ast.AssignStmt:
			if p.Tok == token.DEFINE && len(p.Rhs) == 1 && p.Rhs[0] == e && isRecover(info, e) {
				return true
			}
		case *ast.BinaryExpr:
			if (p.Op == token.EQL || p.Op == token.NEQ) && (info.Types[p.X].IsNil() || info.Types[p.Y].IsNil()) {
				return true
			}
		case *ast.CallExpr:
			if p.Fun != e && printCall(info, p) {
				return true
			}
		}
		problem = "using recover's value other than by comparing it with nil or printing it (RFC 0020)"
		return false
	})
	return problem
}

// printCall reports whether call is a print of fmt: Print, Println,
// Printf, Sprint, Sprintln or Sprintf.
func printCall(info *types.Info, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	pn, ok := info.Uses[id].(*types.PkgName)
	if !ok || pn.Imported().Path() != "fmt" {
		return false
	}
	switch sel.Sel.Name {
	case "Print", "Println", "Printf", "Sprint", "Sprintln", "Sprintf":
		return true
	}
	return false
}

// recoveringFuncProblem checks function fd that defers a literal calling
// recover: what it returns after a recovered panic is Go's, its named
// results as they are and the zero value of the others, so its results are
// values Rust copies or has a zero value for (RFC 0020, decision 3).
func recoveringFuncProblem(sig *types.Signature) string {
	r := sig.Results()
	for i := 0; i < r.Len(); i++ {
		t := r.At(i).Type()
		named := r.At(i).Name() != "" && r.At(i).Name() != "_"
		switch {
		case isCopyBasic(t) || copyStruct(t):
		case isString(t) && !named:
		default:
			return fmt.Sprintf("a function that recovers with a result of type %s (RFC 0020 takes numbers, bools and structs of them, and unnamed strings)", t)
		}
	}
	return ""
}

// recoveredPanicProblem checks panic(v) in a program that recovers: Go's
// recover returns v, which prints as %v does, and patina mode's recover
// returns the panic's text, which is the same for a string and an error,
// and not for a named string type (T("s")) or a Stringer.
func recoveredPanicProblem(info *types.Info, call *ast.CallExpr) string {
	t := info.Types[call.Args[0]].Type
	if _, basic := t.(*types.Basic); isError(t) || basic {
		return ""
	}
	return fmt.Sprintf("panic with a value of type %s in a program that recovers (recover's value would print differently; RFC 0020)", t)
}
