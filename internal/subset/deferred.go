package subset

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sync"

	"github.com/pigfox/gopatina/internal/load"
)

// Deferred closures (RFC 0018, step 5): defer func() {...}() runs the
// literal when its function returns, or as a panic unwinds through it. It
// is moved into the function's rt::Defers, a copy of each variable it uses,
// so what it uses must not change after the defer statement, and its body
// must not panic: a second panic while one unwinds aborts a Rust program.

var (
	deferMu    sync.Mutex
	deferCache = map[*load.Package]map[*ast.FuncLit]*Closure{}
)

// Deferred returns pkg's deferred function literals, each the closure it
// is.
func Deferred(pkg *load.Package) map[*ast.FuncLit]*Closure {
	deferMu.Lock()
	defer deferMu.Unlock()
	if m, ok := deferCache[pkg]; ok {
		return m
	}
	m := map[*ast.FuncLit]*Closure{}
	for _, f := range pkg.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if d, ok := n.(*ast.DeferStmt); ok {
				if lit, ok := ast.Unparen(d.Call.Fun).(*ast.FuncLit); ok {
					m[lit] = newClosure(pkg.Info, lit, nil)
				}
			}
			return true
		})
	}
	deferCache[pkg] = m
	return m
}

// deferredClosure checks defer func() {...}() at s, for closure cl, in the
// function whose body is body; then the literal's body.
func (c *checker) deferredClosure(s *ast.DeferStmt, cl *Closure, body *ast.BlockStmt) {
	sig := c.pkg.Info.Types[cl.Lit].Type.(*types.Signature)
	what := ""
	switch {
	case sig.Params().Len() > 0 || sig.Results().Len() > 0:
		what = "deferring a function literal with parameters or results (step 5 of RFC 0018 takes func() {...}())"
	case inLoop(body, s):
		what = "deferring a function literal inside a loop (step 5 of RFC 0018)"
	default:
		what = closureBodyProblem(c.pkg, cl)
	}
	if what == "" {
		what = deferredUseProblem(c.pkg.Info, cl, s, body)
	}
	if what == "" {
		what = mayPanic(c.pkg.Info, cl.Lit.Body)
	}
	if what == "" {
		what = recoverUseProblem(c.pkg, cl.Lit) // RFC 0020
	}
	if what != "" {
		c.unsupported(cl.Lit, what)
		return
	}
	saved, savedLoops := c.inClosure, c.loops
	c.inClosure, c.loops = cl.Lit, nil
	c.block(cl.Lit.Body)
	c.inClosure, c.loops = saved, savedLoops
}

// inLoop reports whether s is inside a loop of body.
func inLoop(body *ast.BlockStmt, s ast.Stmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			if n.Pos() <= s.Pos() && s.End() <= n.End() {
				found = true
			}
		}
		return !found
	})
	return found
}

// deferredUseProblem checks that the deferred closure cl does not change
// what it uses, and that nothing changes it after the defer statement s:
// the closure runs with copies made at s.
func deferredUseProblem(info *types.Info, cl *Closure, s *ast.DeferStmt, body *ast.BlockStmt) string {
	for _, v := range cl.Captures {
		if cl.Writes[v] {
			return fmt.Sprintf("a deferred function literal that changes %s, of its enclosing function (step 5 of RFC 0018)", v.Name())
		}
	}
	captured := map[*types.Var]bool{}
	for _, v := range cl.Captures {
		captured[v] = true
	}
	problem := ""
	changed := func(e ast.Expr) {
		if id, ok := ast.Unparen(e).(*ast.Ident); ok && id.Pos() > s.End() {
			if v, _ := info.Uses[id].(*types.Var); captured[v] {
				problem = fmt.Sprintf("changing %s after deferring a function literal that uses it (step 5 of RFC 0018)", v.Name())
			}
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, l := range n.Lhs {
				changed(l)
			}
		case *ast.IncDecStmt:
			changed(n.X)
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				changed(n.X)
			}
		}
		return problem == ""
	})
	return problem
}

// mayPanic names what in a deferred closure's body could panic, or is "".
// The body takes numbers, bools and strings (closureBodyProblem), so what
// can panic is an index or slice, a division or shift by a variable, a
// conversion or assertion, and a call other than a print or math.
func mayPanic(info *types.Info, body *ast.BlockStmt) string {
	what := ""
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.IndexExpr, *ast.SliceExpr:
			what = "an index or slice expression in a deferred function literal (it may panic while a panic unwinds)"
		case *ast.BinaryExpr:
			if divides(info, n.Op, n.Y) {
				what = "a division, remainder or shift by a variable in a deferred function literal (it may panic while a panic unwinds)"
			}
		case *ast.AssignStmt:
			if len(n.Rhs) == 1 && divides(info, divOp[n.Tok], n.Rhs[0]) {
				what = "a division, remainder or shift by a variable in a deferred function literal (it may panic while a panic unwinds)"
			}
		case *ast.CallExpr:
			if !safeCall(info, n) {
				what = "a call other than fmt.Print, Println, Printf or a math function in a deferred function literal (it may panic while a panic unwinds)"
			}
		}
		return what == ""
	})
	return what
}

// divOp maps an assignment operator to its binary operator, for the ones
// that may panic.
var divOp = map[token.Token]token.Token{token.QUO_ASSIGN: token.QUO, token.REM_ASSIGN: token.REM, token.SHL_ASSIGN: token.SHL, token.SHR_ASSIGN: token.SHR}

// divides reports whether op y may panic: an integer division or remainder,
// or a shift, by a value that is not a constant.
func divides(info *types.Info, op token.Token, y ast.Expr) bool {
	switch op {
	case token.QUO, token.REM, token.SHL, token.SHR:
	default:
		return false
	}
	tv := info.Types[y]
	if tv.Value != nil {
		return false
	}
	b, ok := tv.Type.Underlying().(*types.Basic)
	return op == token.SHL || op == token.SHR || ok && b.Info()&types.IsInteger != 0
}

// safeCall reports whether call, in a deferred closure, cannot panic: a
// print, or a function of math, or a conversion between number types.
func safeCall(info *types.Info, call *ast.CallExpr) bool {
	if isRecover(info, call) {
		return true // RFC 0020
	}
	if tv := info.Types[call.Fun]; tv.IsType() {
		b, ok := tv.Type.Underlying().(*types.Basic)
		return ok && b.Info()&types.IsNumeric != 0
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	pn, ok := info.Uses[id].(*types.PkgName)
	if !ok {
		return false
	}
	switch pn.Imported().Path() {
	case "math":
		return true
	case "fmt":
		switch sel.Sel.Name {
		case "Print", "Println", "Printf", "Sprint", "Sprintln", "Sprintf":
			return true
		}
	}
	return false
}
