package subset

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"github.com/pigfox/gopatina/internal/diag"
	"github.com/pigfox/gopatina/internal/load"
)

// Errors are lowered to Rust's Result: a function whose last result is
// error returns Result<T, Error>, every return is Ok(...) or Err(...), and
// every call checks its error at once with if err != nil. This is exact
// only for the patterns below; anything else is rejected, so Go's meaning
// is never changed.

// An ErrCall is a call of a function that returns an error, and what the
// caller does with the error.
type ErrCall struct {
	Call   *ast.CallExpr
	Values []ast.Expr // the left-hand sides for the non-error results
	Define bool       // the values are declared with :=
	Err    *ast.Ident // the error variable, nil if it is ignored (_)
	// Check is the if err != nil statement that handles the error, nil if
	// the error is ignored. Init reports that the call is the statement's
	// init (if v, err := f(); err != nil).
	Check *ast.IfStmt
	Init  bool
	// Propagate: Check's body is exactly return <zero values>, err in a
	// function that also returns an error, so it becomes Rust's ? operator.
	Propagate bool
}

// An ErrPlan says how every error in a package is lowered.
type ErrPlan struct {
	// Funcs holds the functions whose last result is error.
	Funcs map[*types.Func]bool
	// Calls maps the statement holding each such call (an assignment,
	// expression statement or if statement) to the call.
	Calls map[ast.Stmt]*ErrCall
	// Checks holds the if statements that Calls already cover.
	Checks map[*ast.IfStmt]bool
	// Returns classifies each return statement of a function in Funcs.
	Returns map[*ast.ReturnStmt]ReturnKind
}

// A ReturnKind says how a return statement in a function returning an error
// is written.
type ReturnKind int

const (
	ReturnOk   ReturnKind = iota + 1 // return v..., nil: Ok(v...)
	ReturnErr                        // return zero..., err: Err(err)
	ReturnCall                       // return f(...): f's Result as it is
)

type errChecker struct {
	pkg     *load.Package
	plan    *ErrPlan
	diags   []diag.Diagnostic
	fn      *types.Func
	allowed map[ast.Node]bool   // error variables and calls in a supported place
	errs    []*types.Var        // the error variables of the enclosing checks
	values  map[*types.Var]bool // values that hold nothing in a check's body
}

// Errors plans the lowering of every error value in pkg, and reports the
// uses of errors it cannot lower exactly.
func Errors(pkg *load.Package) (*ErrPlan, []diag.Diagnostic) {
	c := &errChecker{pkg: pkg, plan: &ErrPlan{
		Funcs:   map[*types.Func]bool{},
		Calls:   map[ast.Stmt]*ErrCall{},
		Checks:  map[*ast.IfStmt]bool{},
		Returns: map[*ast.ReturnStmt]ReturnKind{},
	}, allowed: map[ast.Node]bool{}, values: map[*types.Var]bool{}}
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				obj := pkg.Info.Defs[fd.Name].(*types.Func)
				if returnsError(obj.Signature()) {
					c.plan.Funcs[obj] = true
				}
			}
		}
	}
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
				c.fn = pkg.Info.Defs[fd.Name].(*types.Func)
				c.block(fd.Body.List)
			}
		}
	}
	// Every other use of an error value is outside the patterns.
	for _, f := range pkg.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Ident:
				v, ok := pkg.Info.Uses[n].(*types.Var)
				if !ok {
					v, ok = pkg.Info.Defs[n].(*types.Var)
				}
				if ok && n.Name != "_" && isError(v.Type()) && !c.allowed[n] {
					c.errorf(n, "error %s is used here outside an if %s != nil check", n.Name, n.Name)
				}
			case *ast.CallExpr:
				if c.allowed[n] {
					return true
				}
				if fn := c.calledFunc(n); fn != nil {
					c.errorf(n, "the error from %s is not checked right after the call", fn.Name())
				} else if name := c.stdName(n); name == "errors.New" || name == "fmt.Errorf" {
					c.errorf(n, "%s is used outside a return statement", name)
				}
			}
			return true
		})
	}
	return c.plan, c.diags
}

func returnsError(sig *types.Signature) bool {
	r := sig.Results()
	return r.Len() > 0 && isError(r.At(r.Len()-1).Type())
}

func (c *errChecker) errorf(n ast.Node, format string, args ...any) {
	d := diag.Errorf(c.pkg.Fset.Position(n.Pos()), format, args...)
	c.diags = append(c.diags, d.With(
		"patina mode lowers Go errors to Rust's Result, which needs each error checked",
		"right after the call: v, err := f(); if err != nil { ... }"))
}

// calledFunc returns the function n calls if it returns an error, or nil.
func (c *errChecker) calledFunc(n *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch f := ast.Unparen(n.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	default:
		return nil
	}
	fn, ok := c.pkg.Info.Uses[id].(*types.Func)
	if !ok || fn.Pkg() != c.pkg.Types || !returnsError(fn.Signature()) {
		return nil
	}
	return fn
}

// stdName returns "pkg.Name" for a call of a standard-library function.
func (c *errChecker) stdName(n *ast.CallExpr) string {
	sel, ok := ast.Unparen(n.Fun).(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	fn, ok := c.pkg.Info.Uses[sel.Sel].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg() == c.pkg.Types {
		return ""
	}
	return fn.Pkg().Path() + "." + fn.Name()
}

func (c *errChecker) block(list []ast.Stmt) {
	for i := 0; i < len(list); i++ {
		var next ast.Stmt
		if i+1 < len(list) {
			next = list[i+1]
		}
		if used := c.stmt(list[i], next); used {
			i++
		}
	}
}

// stmt handles s, whose next statement in the block is next (or nil). It
// reports whether it used next as the check of an error.
func (c *errChecker) stmt(s ast.Stmt, next ast.Stmt) bool {
	switch s := s.(type) {
	case *ast.AssignStmt:
		if call := c.errCall(s); call != nil {
			return c.assignCall(s, call, next)
		}
	case *ast.ExprStmt:
		if call, ok := ast.Unparen(s.X).(*ast.CallExpr); ok && c.calledFunc(call) != nil {
			c.allowed[call] = true
			c.plan.Calls[s] = &ErrCall{Call: call}
		}
	case *ast.ReturnStmt:
		c.ret(s)
	case *ast.BlockStmt:
		c.block(s.List)
	case *ast.IfStmt:
		c.ifStmt(s)
	case *ast.ForStmt:
		c.block(s.Body.List)
	case *ast.RangeStmt:
		c.block(s.Body.List)
	case *ast.SwitchStmt:
		for _, cc := range s.Body.List {
			c.block(cc.(*ast.CaseClause).Body)
		}
	}
	return false
}

// errCall returns the call in v..., err := f(...) or v..., err = f(...)
// if f returns an error.
func (c *errChecker) errCall(s *ast.AssignStmt) *ast.CallExpr {
	if len(s.Rhs) != 1 || (s.Tok != token.DEFINE && s.Tok != token.ASSIGN) {
		return nil
	}
	call, ok := ast.Unparen(s.Rhs[0]).(*ast.CallExpr)
	if !ok || c.calledFunc(call) == nil {
		return nil
	}
	return call
}

func (c *errChecker) assignCall(s *ast.AssignStmt, call *ast.CallExpr, next ast.Stmt) bool {
	c.allowed[call] = true
	ec := &ErrCall{Call: call, Values: s.Lhs[:len(s.Lhs)-1], Define: s.Tok == token.DEFINE}
	c.plan.Calls[s] = ec
	errID, _ := s.Lhs[len(s.Lhs)-1].(*ast.Ident)
	if errID == nil || errID.Name == "_" {
		return false // the error is ignored
	}
	c.allowed[errID] = true
	ec.Err = errID
	check, ok := next.(*ast.IfStmt)
	if !ok || !c.isCheck(check, errID) || check.Init != nil || check.Else != nil {
		c.errorf(errID, "error %s is not checked right after the call (if %s != nil { ... } with no else)", errID.Name, errID.Name)
		return false
	}
	ec.Check = check
	c.plan.Checks[check] = true
	c.checkBody(ec, check.Body)
	return true
}

// isCheck reports whether s's condition is err != nil for the error err.
func (c *errChecker) isCheck(s *ast.IfStmt, err *ast.Ident) bool {
	b, ok := ast.Unparen(s.Cond).(*ast.BinaryExpr)
	if !ok || b.Op != token.NEQ {
		return false
	}
	x, ok := ast.Unparen(b.X).(*ast.Ident)
	if !ok || !c.pkg.Info.Types[b.Y].IsNil() || c.obj(x) != c.obj(err) {
		return false
	}
	c.allowed[x] = true
	return true
}

func (c *errChecker) obj(id *ast.Ident) types.Object {
	if o := c.pkg.Info.Uses[id]; o != nil {
		return o
	}
	return c.pkg.Info.Defs[id]
}

// checkBody checks the body of if err != nil, where err is non-nil and the
// values hold nothing.
func (c *errChecker) checkBody(ec *ErrCall, body *ast.BlockStmt) {
	errVar := c.obj(ec.Err).(*types.Var)
	for _, v := range ec.Values {
		if id, ok := v.(*ast.Ident); ok {
			if o, ok := c.obj(id).(*types.Var); ok {
				c.values[o] = true
				defer delete(c.values, o)
			}
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			if v, ok := c.pkg.Info.Uses[n].(*types.Var); ok && c.values[v] {
				c.errorf(n, "%s holds no value when %s != nil", n.Name, ec.Err.Name)
			}
		case *ast.CallExpr:
			c.errArgs(n, errVar)
		case *ast.SelectorExpr:
			if id, ok := n.X.(*ast.Ident); ok && c.obj(id) == errVar && n.Sel.Name == "Error" {
				c.allowed[id] = true
			}
		}
		return true
	})
	c.errs = append(c.errs, errVar)
	c.block(body.List)
	c.errs = c.errs[:len(c.errs)-1]
	if len(body.List) == 1 && c.fnReturnsError() {
		if r, ok := body.List[0].(*ast.ReturnStmt); ok && c.plan.Returns[r] == ReturnErr && len(r.Results) > 0 {
			if id, ok := r.Results[len(r.Results)-1].(*ast.Ident); ok && c.obj(id) == errVar {
				ec.Propagate = true
			}
		}
	}
}

func (c *errChecker) fnReturnsError() bool { return c.plan.Funcs[c.fn] }

// errArgs allows the error variable as an operand of a fmt function.
func (c *errChecker) errArgs(n *ast.CallExpr, errVar *types.Var) {
	name := c.stdName(n)
	switch name {
	case "fmt.Print", "fmt.Println", "fmt.Printf", "fmt.Sprint", "fmt.Sprintln", "fmt.Sprintf",
		"fmt.Fprint", "fmt.Fprintln", "fmt.Fprintf", "fmt.Errorf":
	default:
		return
	}
	for _, a := range n.Args {
		if id, ok := ast.Unparen(a).(*ast.Ident); ok && c.obj(id) == errVar {
			c.allowed[id] = true
		}
	}
}

func (c *errChecker) ifStmt(s *ast.IfStmt) {
	init, _ := s.Init.(*ast.AssignStmt)
	if init != nil {
		if call := c.errCall(init); call != nil {
			c.allowed[call] = true
			ec := &ErrCall{Call: call, Values: init.Lhs[:len(init.Lhs)-1], Define: init.Tok == token.DEFINE, Init: true}
			c.plan.Calls[s] = ec
			errID, _ := init.Lhs[len(init.Lhs)-1].(*ast.Ident)
			if errID == nil || errID.Name == "_" || !c.isCheck(s, errID) || init.Tok != token.DEFINE {
				c.errorf(init, "an if statement that calls a function returning an error must check it: if v, err := f(); err != nil")
				return
			}
			c.allowed[errID] = true
			ec.Err = errID
			ec.Check = s
			c.plan.Checks[s] = true
			c.checkBody(ec, s.Body)
			if s.Else != nil {
				c.stmt(s.Else, nil)
			}
			return
		}
	}
	c.block(s.Body.List)
	if s.Else != nil {
		c.stmt(s.Else, nil)
	}
}

// ret classifies a return statement.
func (c *errChecker) ret(s *ast.ReturnStmt) {
	if !c.plan.Funcs[c.fn] {
		return
	}
	n := c.fn.Signature().Results().Len()
	switch {
	case len(s.Results) == 1 && n > 1:
		call, _ := ast.Unparen(s.Results[0]).(*ast.CallExpr)
		c.allowed[call] = true
		c.plan.Returns[s] = ReturnCall
		return
	case len(s.Results) == 0:
		c.errorf(s, "a bare return in a function that returns an error")
		return
	}
	last := ast.Unparen(s.Results[n-1])
	if c.pkg.Info.Types[last].IsNil() {
		c.plan.Returns[s] = ReturnOk
		return
	}
	if n == 1 {
		if call, ok := last.(*ast.CallExpr); ok && c.calledFunc(call) != nil {
			c.allowed[call] = true
			c.plan.Returns[s] = ReturnCall
			return
		}
	}
	ok := false
	switch e := last.(type) {
	case *ast.CallExpr:
		if name := c.stdName(e); name == "errors.New" || name == "fmt.Errorf" {
			c.allowed[e] = true
			ok = true
		}
	case *ast.Ident:
		for _, v := range c.errs {
			if c.obj(e) == v {
				c.allowed[e] = true
				ok = true
			}
		}
	}
	if !ok {
		c.errorf(last, "this error may be nil; return nil, or a non-nil error such as errors.New(...) or the err of an if err != nil check")
		return
	}
	for _, r := range s.Results[:n-1] {
		if !c.isZero(r) {
			c.errorf(r, "a value returned together with a non-nil error must be the zero value")
			return
		}
	}
	c.plan.Returns[s] = ReturnErr
}

// isZero reports whether e is written as the zero value of its type.
func (c *errChecker) isZero(e ast.Expr) bool {
	tv := c.pkg.Info.Types[e]
	if tv.Value != nil {
		switch tv.Value.Kind() {
		case constant.Bool:
			return !constant.BoolVal(tv.Value)
		case constant.String:
			return constant.StringVal(tv.Value) == ""
		default:
			return constant.Sign(tv.Value) == 0
		}
	}
	if tv.IsNil() {
		c.allowed[e] = true
		return true
	}
	lit, ok := ast.Unparen(e).(*ast.CompositeLit)
	return ok && len(lit.Elts) == 0
}
