package subset

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sync"

	"github.com/pigfox/gopatina/internal/load"
)

// A Closure is a function literal bound to a local variable with :=, and
// only called, in the function that makes it (RFC 0018, step 1). It
// borrows the variables it captures for as long as it is still to be
// called: they are local variables of a number or bool type.
type Closure struct {
	Lit *ast.FuncLit
	Var *types.Var // the variable it is bound to
	// Captures are the enclosing function's variables the body uses, in
	// the order of their first use; Writes are those it changes.
	Captures []*types.Var
	Writes   map[*types.Var]bool
}

var (
	closuresMu    sync.Mutex
	closuresCache = map[*load.Package]map[*types.Var]*Closure{}
)

// Closures returns pkg's closures, by the variable each is bound to: every
// f := func(...) {...} inside a function body. The checker decides which
// are supported; the borrow checker and the emitter see only those.
func Closures(pkg *load.Package) map[*types.Var]*Closure {
	closuresMu.Lock()
	defer closuresMu.Unlock()
	if cs, ok := closuresCache[pkg]; ok {
		return cs
	}
	cs := map[*types.Var]*Closure{}
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				as, ok := n.(*ast.AssignStmt)
				if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
					return true
				}
				lit, ok := ast.Unparen(as.Rhs[0]).(*ast.FuncLit)
				if !ok {
					return true
				}
				// := of one name defines a new variable.
				v := pkg.Info.Defs[as.Lhs[0].(*ast.Ident)].(*types.Var)
				cs[v] = newClosure(pkg.Info, lit, v)
				return true
			})
		}
	}
	closuresCache[pkg] = cs
	return cs
}

// newClosure finds what lit captures from its enclosing function, and
// what it changes of that.
func newClosure(info *types.Info, lit *ast.FuncLit, v *types.Var) *Closure {
	c := &Closure{Lit: lit, Var: v, Writes: map[*types.Var]bool{}}
	seen := map[*types.Var]bool{}
	captured := func(id *ast.Ident) *types.Var {
		u, ok := info.Uses[id].(*types.Var)
		if !ok || u.IsField() || u.Parent() == nil || u.Pkg() == nil || u.Parent() == u.Pkg().Scope() {
			return nil // not a local variable
		}
		if u.Pos() >= lit.Pos() && u.Pos() < lit.End() {
			return nil // the literal's own
		}
		return u
	}
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			if u := captured(n); u != nil && !seen[u] {
				seen[u] = true
				c.Captures = append(c.Captures, u)
			}
		case *ast.AssignStmt:
			for _, l := range n.Lhs {
				if id, ok := ast.Unparen(l).(*ast.Ident); ok {
					if u := captured(id); u != nil {
						c.Writes[u] = true
					}
				}
			}
		case *ast.IncDecStmt:
			if id, ok := ast.Unparen(n.X).(*ast.Ident); ok {
				if u := captured(id); u != nil {
					c.Writes[u] = true
				}
			}
		}
		return true
	})
	return c
}

// closureLit returns the function literal of f := func(...) {...}.
func closureLit(s *ast.AssignStmt) (*ast.FuncLit, bool) {
	if s.Tok != token.DEFINE || len(s.Lhs) != 1 || len(s.Rhs) != 1 {
		return nil, false
	}
	lit, ok := ast.Unparen(s.Rhs[0]).(*ast.FuncLit)
	return lit, ok
}

// closureDecl checks f := func(...) {...} at s, in the function whose body
// is body (RFC 0018, step 1), and then the literal's body.
func (c *checker) closureDecl(s *ast.AssignStmt, lit *ast.FuncLit, body *ast.BlockStmt) {
	v := c.pkg.Info.Defs[s.Lhs[0].(*ast.Ident)].(*types.Var)
	cl := Closures(c.pkg)[v]
	if what := closureProblem(c.pkg, cl, body); what != "" {
		c.unsupported(lit, what)
		return
	}
	saved, savedLoops := c.inClosure, c.loops
	c.inClosure, c.loops = lit, nil
	c.block(lit.Body)
	c.inClosure, c.loops = saved, savedLoops
}

// closureProblem says why closure cl, in the function whose body is body,
// is not supported yet, or returns "".
func closureProblem(pkg *load.Package, cl *Closure, body *ast.BlockStmt) string {
	if what := closureBodyProblem(pkg, cl); what != "" {
		return what
	}
	return closureUseProblem(pkg.Info, cl, body)
}

// closureBodyProblem says why the function literal of cl is not supported
// yet: its signature, what it captures, what its body does.
func closureBodyProblem(pkg *load.Package, cl *Closure) string {
	info := pkg.Info
	sig := info.Types[cl.Lit].Type.(*types.Signature)
	if sig.Variadic() {
		return "a function literal with a ... parameter"
	}
	for i := 0; i < sig.Params().Len(); i++ {
		if t := sig.Params().At(i).Type(); !isCopyBasic(t) && !isString(t) {
			return fmt.Sprintf("a function literal with a parameter of type %s (RFC 0018 takes numbers, bools and strings)", sig.Params().At(i).Type())
		}
	}
	switch r := sig.Results(); {
	case r.Len() > 1:
		return "a function literal with more than one result"
	case r.Len() == 1 && r.At(0).Name() != "":
		return "a function literal with a named result"
	case r.Len() == 1 && !isCopyBasic(r.At(0).Type()) && !isString(r.At(0).Type()):
		return fmt.Sprintf("a function literal with a result of type %s (step 1 of RFC 0018 takes numbers, bools and strings)", r.At(0).Type())
	}
	for _, v := range cl.Captures {
		if !isCopyBasic(v.Type()) && !isString(v.Type()) {
			return fmt.Sprintf("a function literal that uses %s, of type %s, from its enclosing function (RFC 0018 takes numbers, bools and strings)", v.Name(), v.Type())
		}
	}
	what := ""
	ast.Inspect(cl.Lit.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			what = "a function literal inside a function literal"
		case *ast.DeferStmt:
			what = "defer inside a function literal"
		case *ast.UnaryExpr:
			if id, ok := ast.Unparen(n.X).(*ast.Ident); ok && n.Op == token.AND && cl.captures(info, id) {
				what = "taking the address of a variable a function literal uses from its enclosing function"
			}
		case *ast.Ident:
			if g := Global(info.Uses[n]); g != nil {
				what = "a package-level variable inside a function literal"
			}
		case *ast.CallExpr:
			if programCall(pkg, n) {
				what = "a call of a function of the program inside a function literal"
			}
		}
		return what == ""
	})
	return what
}

// captures reports whether id names a variable cl captures.
func (cl *Closure) captures(info *types.Info, id *ast.Ident) bool {
	u, _ := info.Uses[id].(*types.Var)
	for _, v := range cl.Captures {
		if v == u {
			return true
		}
	}
	return false
}

// closureUseProblem checks the uses of cl's variable in body: each a call,
// after the literal; and that while the closure is still to be called, the
// function does not change a variable it reads, nor use one it changes,
// which would conflict with the closure's borrow of it.
func closureUseProblem(info *types.Info, cl *Closure, body *ast.BlockStmt) string {
	calls := callees(body)
	ast.Inspect(body, func(n ast.Node) bool {
		// An argument of a parameter of function type (step 2) is a use
		// too: the callee calls it.
		if call, ok := n.(*ast.CallExpr); ok {
			for i, a := range call.Args {
				if id, ok := ast.Unparen(a).(*ast.Ident); ok && funcParamAt(info, call, i) != nil {
					calls[id] = true
				}
			}
		}
		return true
	})
	end := cl.Lit.End()
	problem := ""
	ast.Inspect(body, func(n ast.Node) bool {
		if n == cl.Lit {
			return false
		}
		id, ok := n.(*ast.Ident)
		if !ok || info.Uses[id] != cl.Var {
			return true
		}
		if !calls[id] {
			problem = fmt.Sprintf("using the closure %s other than by calling it or passing it to a parameter of function type (RFC 0018, steps 1 and 2)", cl.Var.Name())
			return false
		}
		if last := liveUntil(body, cl.Lit, id); last > end {
			end = last
		}
		return true
	})
	if problem != "" {
		return problem
	}
	// The uses of captured variables while the closure is still to be
	// called, outside its body.
	written := writtenIdents(body)
	ast.Inspect(body, func(n ast.Node) bool {
		if n == cl.Lit {
			return false
		}
		id, ok := n.(*ast.Ident)
		if !ok || id.Pos() <= cl.Lit.End() || id.Pos() >= end {
			return true
		}
		v, _ := info.Uses[id].(*types.Var)
		switch {
		case v == nil || !cl.captures(info, id):
		case cl.Writes[v]:
			problem = fmt.Sprintf("%s is used while the closure %s, which changes it, is still to be called (it borrows %s until its last call)", v.Name(), cl.Var.Name(), v.Name())
		case written[id]:
			problem = fmt.Sprintf("%s is changed while the closure %s, which reads it, is still to be called (it borrows %s until its last call)", v.Name(), cl.Var.Name(), v.Name())
		}
		return problem == ""
	})
	return problem
}

// liveUntil returns how long the closure lit stays live for its use id:
// to the end of the outermost loop that holds id but not lit (a later
// iteration calls it again), or to id itself.
func liveUntil(body *ast.BlockStmt, lit *ast.FuncLit, id *ast.Ident) token.Pos {
	until := id.End()
	ast.Inspect(body, func(n ast.Node) bool {
		var loop ast.Node
		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			loop = n
		default:
			return true
		}
		holdsUse := id.Pos() >= loop.Pos() && id.End() <= loop.End()
		holdsLit := lit.Pos() >= loop.Pos() && lit.End() <= loop.End()
		if holdsUse && !holdsLit {
			if loop.End() > until {
				until = loop.End()
			}
			return false // the outermost such loop
		}
		return true
	})
	return until
}

// writtenIdents returns the identifiers body assigns, increments,
// decrements or takes the address of.
func writtenIdents(body *ast.BlockStmt) map[*ast.Ident]bool {
	out := map[*ast.Ident]bool{}
	mark := func(e ast.Expr) {
		if id, ok := ast.Unparen(e).(*ast.Ident); ok {
			out[id] = true
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, l := range n.Lhs {
				mark(l)
			}
		case *ast.IncDecStmt:
			mark(n.X)
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				mark(n.X)
			}
		}
		return true
	})
	return out
}

// isCopyBasic reports whether t is a number or bool type: a closure takes
// and captures those (RFC 0018, step 1).
func isCopyBasic(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&(types.IsNumeric|types.IsBoolean) != 0 && b.Info()&types.IsComplex == 0
}
