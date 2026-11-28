package subset

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sync"

	"github.com/pigfox/gopatina/internal/load"
)

// Recursive closures (RFC 0018, step 4): var f func(...) R, then one
// f = func(...) R {...} whose literal uses nothing of its enclosing function
// but f itself. It is a nested Rust fn, which can call itself and cannot
// capture.

type recPlan struct {
	byVar  map[*types.Var]*Closure
	assign map[*types.Var]*ast.AssignStmt
}

var (
	recMu    sync.Mutex
	recCache = map[*load.Package]*recPlan{}
)

func recursive(pkg *load.Package) *recPlan {
	recMu.Lock()
	defer recMu.Unlock()
	if p, ok := recCache[pkg]; ok {
		return p
	}
	p := &recPlan{byVar: map[*types.Var]*Closure{}, assign: map[*types.Var]*ast.AssignStmt{}}
	info := pkg.Info
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			declared := map[*types.Var]bool{}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.ValueSpec:
					if len(n.Names) == 1 && len(n.Values) == 0 {
						if v, ok := info.Defs[n.Names[0]].(*types.Var); ok && isFuncType(v.Type()) {
							declared[v] = true
						}
					}
				case *ast.AssignStmt:
					if n.Tok != token.ASSIGN || len(n.Lhs) != 1 || len(n.Rhs) != 1 {
						return true
					}
					id, ok := ast.Unparen(n.Lhs[0]).(*ast.Ident)
					lit, isLit := ast.Unparen(n.Rhs[0]).(*ast.FuncLit)
					if !ok || !isLit {
						return true
					}
					v, _ := info.Uses[id].(*types.Var)
					if declared[v] && p.byVar[v] == nil {
						cl := newClosure(info, lit, v)
						// It calls itself; that is not a capture.
						var others []*types.Var
						for _, c := range cl.Captures {
							if c != v {
								others = append(others, c)
							}
						}
						cl.Captures = others
						p.byVar[v], p.assign[v] = cl, n
					}
				}
				return true
			})
		}
	}
	recCache[pkg] = p
	return p
}

// Recursive returns pkg's recursive closures, by the variable each is
// assigned to.
func Recursive(pkg *load.Package) map[*types.Var]*Closure { return recursive(pkg).byVar }

// recursiveDecl checks f = func(...) {...} at s for recursive closure cl,
// in the function whose body is body, and then the literal's body.
func (c *checker) recursiveDecl(s *ast.AssignStmt, cl *Closure, body *ast.BlockStmt) {
	if len(cl.Captures) > 0 {
		c.unsupported(cl.Lit, fmt.Sprintf("a recursive function literal that uses %s from its enclosing function (it is a nested fn, which cannot capture; step 4 of RFC 0018)", cl.Captures[0].Name()))
		return
	}
	if what := closureBodyProblem(c.pkg, cl); what != "" {
		c.unsupported(cl.Lit, what)
		return
	}
	if what := recursiveUseProblem(c.pkg.Info, cl, s, body); what != "" {
		c.unsupported(cl.Lit, what)
		return
	}
	saved, savedLoops := c.inClosure, c.loops
	c.inClosure, c.loops = cl.Lit, nil
	c.block(cl.Lit.Body)
	c.inClosure, c.loops = saved, savedLoops
}

// recursiveUseProblem checks the uses of cl's variable in body: its one
// assignment, then calls and arguments of parameters of function type,
// none before the assignment (Go would call a nil function).
func recursiveUseProblem(info *types.Info, cl *Closure, s *ast.AssignStmt, body *ast.BlockStmt) string {
	ok := callees(body)
	ast.Inspect(body, func(n ast.Node) bool {
		if call, isCall := n.(*ast.CallExpr); isCall {
			for i, a := range call.Args {
				if id, isID := ast.Unparen(a).(*ast.Ident); isID && funcParamAt(info, call, i) != nil {
					ok[id] = true
				}
			}
		}
		return true
	})
	lhs := ast.Unparen(s.Lhs[0]).(*ast.Ident)
	problem := ""
	ast.Inspect(body, func(n ast.Node) bool {
		id, isID := n.(*ast.Ident)
		if !isID || id == lhs || info.Uses[id] != cl.Var {
			return true
		}
		inLit := id.Pos() >= cl.Lit.Pos() && id.End() <= cl.Lit.End()
		switch {
		case !ok[id]:
			problem = fmt.Sprintf("using the recursive function %s other than by calling it or passing it to a parameter of function type (step 4 of RFC 0018)", cl.Var.Name())
		case !inLit && id.Pos() < s.Pos():
			problem = fmt.Sprintf("calling %s before it is assigned (a nil function in Go)", cl.Var.Name())
		}
		return problem == ""
	})
	return problem
}
