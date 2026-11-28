// Package subset checks that a Go package stays inside the language patina
// mode supports (phase 1, see docs/patina-subset.md). Everything outside it
// is rejected with a Go-style error naming the construct, and the Go
// version that introduced it where that helps, so nothing is ever silently
// miscompiled.
package subset

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/pigfox/gopatina/internal/diag"
	"github.com/pigfox/gopatina/internal/load"
)

// seeCompat is the note attached to every "not supported" error.
const seeCompat = "see docs/compat.md for what patina mode supports"

type checker struct {
	pkg   *load.Package
	diags []diag.Diagnostic
	// body is the body of the function being checked, and inClosure the
	// function literal whose body is (RFC 0018), or nil.
	body      *ast.BlockStmt
	inClosure *ast.FuncLit
	mapMade   map[*types.Var]bool // local maps made where declared (RFC 0019)
	mapInits  map[ast.Expr]bool   // what they are made by
	// fusing is the function of the fused operation being matched
	// (checkFusion).
	fusing *ast.FuncDecl
	// inSwitch counts the switch statements enclosing the current node,
	// reset inside each for loop, so a break that leaves a switch can be
	// told from one that leaves a loop.
	loops []bool // for each enclosing loop or switch: true for a switch
	// trailing holds the break statements that end a case clause.
	trailing map[*ast.BranchStmt]bool
	// selfAppend holds the calls of append in s = append(s, ...).
	selfAppend map[*ast.CallExpr]bool
	// ifacePlan is the program's interface analysis (RFC 0012), made once.
	ifacePlan *IfacePlan
	// methodDecls are the program's method declarations, by method, and
	// printProblems the answers of methodPrintProblem, by method and nil
	// receiver: a program prints at many places, and each would otherwise
	// search every file again (see methodPrintProblem).
	methodDecls   map[*types.Func]*ast.FuncDecl
	printProblems map[printKey]string
	// nilRecv is the pointer receiver of the print method being checked
	// (RFC 0013), or nil.
	nilRecv *types.Var
}

// Check returns every construct in pkg that patina mode does not support,
// sorted by position. An empty result means the package is in the subset.
func Check(pkg *load.Package) []diag.Diagnostic {
	c := &checker{pkg: pkg, trailing: map[*ast.BranchStmt]bool{}, selfAppend: map[*ast.CallExpr]bool{}}
	checkMu.Lock()
	defer checkMu.Unlock()
	localPkgs = pkg.Local
	if obj, ok := pkg.Types.Scope().Lookup("main").(*types.Func); !Library(pkg) && (!ok || obj.Signature().Params().Len() != 0) {
		c.errorf(pkg.Files[0].Name, "package main has no func main()")
	}
	for _, f := range pkg.Files {
		c.file(f)
	}
	c.checkGlobals()
	c.checkIfaces()
	c.checkPtrs()
	c.checkFusion()
	c.checkNaNBits()
	if Library(pkg) {
		c.checkLibrary() // RFC 0014
	}
	_, errDiags := Errors(pkg)
	c.diags = append(c.diags, errDiags...)
	c.explainReach()
	diag.Sort(c.diags)
	return c.diags
}

// explainReach adds to each diagnostic in a package other than main how the
// program reaches the declaration it is in (RFC 0009).
func (c *checker) explainReach() {
	for i, d := range c.diags {
		for n := range c.pkg.Via {
			start, end := c.pkg.Fset.Position(n.Pos()), c.pkg.Fset.Position(n.End())
			if start.Filename == d.Pos.Filename && start.Offset <= d.Pos.Offset && d.Pos.Offset < end.Offset {
				if chain := load.Chain(c.pkg, n); chain != "" {
					c.diags[i] = d.With("reached from main through: " + chain)
				}
				break
			}
		}
	}
}

func (c *checker) errorf(n ast.Node, format string, args ...any) {
	c.diags = append(c.diags, diag.Errorf(c.pkg.Fset.Position(n.Pos()), format, args...))
}

// unsupported reports a construct that patina mode does not support yet.
func (c *checker) unsupported(n ast.Node, what string) {
	d := diag.Errorf(c.pkg.Fset.Position(n.Pos()), "%s is not supported in patina mode yet", what)
	c.diags = append(c.diags, d.With(seeCompat))
}

// localPkgs is the program's translated packages while Check runs, for
// unsupportedType, which has no checker.
var (
	checkMu   sync.Mutex
	localPkgs map[*types.Package]bool
)

var allowedImports = map[string]bool{
	"bytes": true, "errors": true, "fmt": true, "io": true, "os": true, "slices": true, "sort": true, "strings": true,
	"time": true, // RFC 0011: Duration (ProvidedTypes)
	"math": true, // RFC 0011: the functions exact by IEEE 754 (StdFuncs)
}

func (c *checker) file(f *ast.File) {
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		pn, _ := c.pkg.Info.Implicits[imp].(*types.PkgName)
		if imp.Name != nil {
			pn, _ = c.pkg.Info.Defs[imp.Name].(*types.PkgName)
		}
		switch {
		case pn != nil && c.pkg.Local[pn.Imported()] && imp.Name == nil:
			// A package of the program, translated from source (RFC 0009).
		case imp.Name != nil && imp.Name.Name == "_" && c.localPath(path):
			// A blank import of a translated package: its initialization is
			// translated and checked like the rest of the program.
		case !allowedImports[path]:
			c.unsupported(imp, fmt.Sprintf("package %s", path))
		case imp.Name != nil && imp.Name.Name == "_":
			// A blank import runs the package's initialization only, which
			// for the packages patina mode provides changes nothing a
			// program can observe: nothing to write.
		case imp.Name != nil:
			c.unsupported(imp.Name, "a renamed or dot import")
		}
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			c.funcDecl(d)
		case *ast.GenDecl:
			c.genDecl(d, true)
		}
	}
}

func (c *checker) genDecl(d *ast.GenDecl, top bool) {
	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			c.typeSpec(s, top)
		case *ast.ValueSpec:
			if d.Tok == token.VAR && top {
				c.globalSpec(s)
				continue
			}
			for _, name := range s.Names {
				obj := c.pkg.Info.Defs[name]
				if d.Tok == token.CONST {
					c.constValue(name, obj.(*types.Const))
					c.typ(name, obj.Type(), true)
				}
			}
			for _, v := range s.Values {
				c.expr(v)
			}
		}
	}
}

func (c *checker) constValue(n ast.Node, obj *types.Const) {
	if obj.Val().Kind() == constant.String && !utf8.ValidString(constant.StringVal(obj.Val())) {
		c.unsupported(n, "a string constant that is not valid UTF-8")
	}
}

// aliasSpec checks type A = B (RFC 0007): B must be a supported type, and
// not a pointer, error or io.Writer, whose uses the subset checks by their
// syntax.
func (c *checker) aliasSpec(s *ast.TypeSpec) {
	t := types.Unalias(c.pkg.Info.Defs[s.Name].Type())
	switch {
	case isError(t) || isWriter(t):
		c.unsupported(s, "a type alias of error or io.Writer")
	case isPointer(t):
		c.unsupported(s, "a type alias of a pointer type")
	default:
		if what := unsupportedType(t, false); what != "" {
			c.unsupported(s, fmt.Sprintf("type %s: an alias of %s", s.Name.Name, what))
		}
	}
}

// namedSpec checks type T U where U is not a struct literal (RFC 0007): U's
// underlying type must be a supported basic or slice type, or a struct of
// this package (type B A, A a struct, whose fields A's declaration checks).
func (c *checker) namedSpec(s *ast.TypeSpec) {
	t := c.pkg.Info.Defs[s.Name].Type()
	if _, ok := t.Underlying().(*types.Struct); ok {
		from, ok := types.Unalias(c.pkg.Info.Types[s.Type].Type).(*types.Named)
		if !ok || !c.pkg.Local[from.Obj().Pkg()] {
			c.unsupported(s, fmt.Sprintf("type %s: a struct type from another package", s.Name.Name))
		}
		return
	}
	if what := unsupportedType(t.Underlying(), false); what != "" {
		c.unsupported(s, fmt.Sprintf("type %s: %s", s.Name.Name, what))
	}
}

func (c *checker) typeSpec(s *ast.TypeSpec, top bool) {
	if s.TypeParams != nil {
		c.unsupported(s, "a generic type (Go 1.18)")
		return
	}
	if !top {
		c.unsupported(s, "a type declared inside a function")
		return
	}
	if s.Assign.IsValid() {
		c.aliasSpec(s)
		return
	}
	st, ok := s.Type.(*ast.StructType)
	if !ok {
		c.namedSpec(s)
		return
	}
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			c.unsupported(f, "an embedded field")
			continue
		}
		t := c.pkg.Info.Types[f.Type].Type
		if isError(t) {
			continue // an interface value (RFC 0012)
		}
		if isWriter(t) {
			c.unsupported(f, "a struct field of type io.Writer")
			continue
		}
		c.typ(f.Type, pointee(t), false) // a pointer field is a heap pointer (RFC 0013)
	}
}

func (c *checker) funcDecl(d *ast.FuncDecl) {
	if d.Type.TypeParams != nil {
		c.unsupported(d.Name, "a generic function (Go 1.18)")
		return
	}
	if d.Body == nil {
		// Named, so the coverage measurement can say which assembly a
		// program needs.
		c.unsupported(d.Name, "a function declared without a body (assembly): "+funcPath(c.pkg.Info.Defs[d.Name].(*types.Func)))
		return
	}
	if d.Recv != nil {
		recv := d.Recv.List[0]
		t := c.pkg.Info.Types[recv.Type].Type
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		if n, ok := types.Unalias(t).(*types.Named); ok && n.TypeArgs().Len() > 0 {
			c.unsupported(d.Name, "a method on a generic type (Go 1.18)")
			return
		}
	}
	sig := c.pkg.Info.Defs[d.Name].(*types.Func).Signature()
	c.signature(d, sig)
	c.loops = nil
	c.body = d.Body
	if obj, ok := c.pkg.Info.Defs[d.Name].(*types.Func); ok && Recovering(c.pkg)[obj] {
		if what := recoveringFuncProblem(obj.Signature()); what != "" {
			c.unsupported(d.Name, what)
		}
	}
	c.mapMade, c.mapInits = mapMakes(c.pkg.Info, d.Body)
	c.funcParamUses(d)
	c.funcVarUses(d)
	if returnsFunc(sig) {
		c.funcResult(d, sig)
	}
	c.block(d.Body)
}

// funcPath names fn as path.Name, or path.Type.Name for a method.
func funcPath(fn *types.Func) string {
	name := fn.Name()
	if r := fn.Signature().Recv(); r != nil {
		t := r.Type()
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		name = types.Unalias(t).(*types.Named).Obj().Name() + "." + name
	}
	return fn.Pkg().Path() + "." + name
}

func (c *checker) signature(d *ast.FuncDecl, sig *types.Signature) {
	if sig.Variadic() {
		c.unsupported(d.Name, "a variadic function")
	}
	for i := 0; i < sig.Params().Len(); i++ {
		c.paramType(d.Name, sig.Params().At(i).Type())
	}
	for i := 0; i < sig.Results().Len(); i++ {
		t := sig.Results().At(i).Type()
		switch {
		case returnsFunc(sig):
			// Checked with the body (funcResult, RFC 0018 step 3).
		case isError(t) && i != sig.Results().Len()-1:
			c.unsupported(d.Name, "an error result that is not the last result")
		case isError(t):
		case isWriter(t):
			c.unsupported(d.Name, "an io.Writer result")
		default:
			c.typ(d.Name, pointee(t), false) // a pointer result is a heap pointer (RFC 0013)
		}
	}
}

func (c *checker) paramType(n ast.Node, t types.Type) {
	if sig, ok := t.Underlying().(*types.Signature); ok {
		if what := funcTypeProblem(sig); what != "" {
			c.unsupported(n, what) // RFC 0018, step 2
		}
		return
	}
	switch {
	case isWriter(t):
	case isError(t): // an interface value (RFC 0012)
	default:
		if isPointer(t) {
			c.typ(n, pointee(t), false)
			return
		}
		c.typ(n, t, false)
	}
}

// typ reports t if patina mode cannot represent it. constOK allows the
// untyped constant types.
func (c *checker) typ(n ast.Node, t types.Type, constOK bool) {
	if what := unsupportedType(t, constOK); what != "" {
		c.unsupported(n, what)
	}
}

// unsupportedType describes why t is outside the subset, or returns "".
func unsupportedType(t types.Type, constOK bool) string {
	switch t := t.(type) {
	case *types.Alias:
		return unsupportedType(types.Unalias(t), constOK)
	case *types.Basic:
		switch t.Kind() {
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
			types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64, types.Uintptr,
			types.Float32, types.Float64, types.Bool, types.String: // RFC 0008
			return ""
		case types.UntypedInt, types.UntypedFloat, types.UntypedRune, types.UntypedString, types.UntypedBool:
			if constOK {
				return ""
			}
		}
		return fmt.Sprintf("type %s", t) // unsafe.Pointer by its full name
	case *types.Slice:
		if isWriter(t.Elem()) {
			return fmt.Sprintf("type %s", t)
		}
		// A slice of pointers holds heap pointers (RFC 0013).
		return unsupportedType(pointee(t.Elem()), false)
	case *types.Named:
		if isError(t) {
			return "" // an interface value (RFC 0012)
		}
		if providedName(t) != "" {
			return "" // RFC 0011: its methods are checked where they are called
		}
		if t.Obj().Pkg() == nil || !localPkgs[t.Obj().Pkg()] {
			return fmt.Sprintf("type %s", t)
		}
		if t.TypeArgs().Len() > 0 {
			return "a generic type (Go 1.18)"
		}
		return "" // checked at its declaration
	case *types.Pointer:
		return "a pointer here (RFC 0013 supports pointers in variables, parameters, results and fields)"
	case *types.Map:
		return "a map type other than a variable's (RFC 0019)"
	case *types.Chan:
		return "a channel type (goroutines and channels are planned, RFC 0002)"
	case *types.Interface:
		return interfaceProblem(t) // RFC 0012
	case *types.Signature:
		return "a function type"
	case *types.Array:
		return "an array type"
	case *types.Struct:
		return "an anonymous struct type"
	}
	return fmt.Sprintf("type %s", t)
}

func isError(t types.Type) bool {
	return types.Identical(t, types.Universe.Lookup("error").Type())
}

func isWriter(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	return ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "io" && n.Obj().Name() == "Writer"
}

func (c *checker) block(b *ast.BlockStmt) {
	for _, s := range b.List {
		c.stmt(s)
	}
}

func (c *checker) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.BlockStmt:
		c.block(s)
	case *ast.ExprStmt:
		call, ok := ast.Unparen(s.X).(*ast.CallExpr)
		if !ok {
			// go/types accepts no other expression statement.
			c.unsupported(s, "a channel receive as a statement")
			return
		}
		c.call(call, true)
	case *ast.AssignStmt:
		c.assign(s)
	case *ast.IncDecStmt:
		c.lvalue(s.X)
	case *ast.DeclStmt:
		c.genDecl(s.Decl.(*ast.GenDecl), false)
		c.localVars(s.Decl.(*ast.GenDecl))
	case *ast.IfStmt:
		if s.Init != nil {
			c.stmt(s.Init)
		}
		c.expr(s.Cond)
		c.block(s.Body)
		if s.Else != nil {
			c.stmt(s.Else)
		}
	case *ast.ForStmt:
		if s.Init != nil {
			c.stmt(s.Init)
		}
		if s.Cond != nil {
			c.expr(s.Cond)
		}
		if s.Post != nil {
			c.stmt(s.Post)
		}
		c.loopBody(s.Body, false)
	case *ast.RangeStmt:
		c.rangeStmt(s)
	case *ast.SwitchStmt:
		c.switchStmt(s)
	case *ast.ReturnStmt:
		for _, r := range s.Results {
			c.expr(r)
		}
	case *ast.BranchStmt:
		c.branch(s)
	case *ast.EmptyStmt:
	case *ast.GoStmt:
		c.unsupported(s, "the go statement (goroutines are planned, RFC 0002)")
	case *ast.DeferStmt:
		c.deferStmt(s)
	case *ast.SelectStmt:
		c.unsupported(s, "select (planned, RFC 0002)")
	case *ast.SendStmt:
		c.unsupported(s, "a channel send (planned, RFC 0002)")
	case *ast.LabeledStmt:
		c.unsupported(s, "a labeled statement")
	case *ast.TypeSwitchStmt:
		c.typeSwitch(s) // RFC 0012
	}
}

// localVars checks the types of variables declared with var inside a
// function.
func (c *checker) localVars(d *ast.GenDecl) {
	if d.Tok != token.VAR {
		return
	}
	for _, spec := range d.Specs {
		vs := spec.(*ast.ValueSpec)
		for _, name := range vs.Names {
			if obj := c.pkg.Info.Defs[name]; obj != nil && Recursive(c.pkg)[obj.(*types.Var)] == nil {
				c.localType(name, obj.Type())
			}
		}
	}
}

// pointee strips every level of pointer from t (RFC 0013: a pointer is a
// borrow or a heap pointer, at any depth).
func pointee(t types.Type) types.Type {
	for isPointer(t) {
		t = types.Unalias(t).(*types.Pointer).Elem()
	}
	return t
}

// localType checks the type of a local variable. It reports the type
// itself here; pointers are checked where they are created.
func (c *checker) localType(n ast.Node, t types.Type) {
	if m, ok := t.Underlying().(*types.Map); ok && c.body != nil && c.inClosure == nil {
		if id, ok := n.(*ast.Ident); ok {
			c.localMap(n, c.pkg.Info.Defs[id].(*types.Var), m) // RFC 0019, step 1
			return
		}
	}
	switch {
	case isError(t):
	case isWriter(t):
		c.unsupported(n, "a local variable of type io.Writer")
	default:
		c.typ(n, pointee(t), false)
	}
}

func (c *checker) loopBody(b *ast.BlockStmt, _ bool) {
	c.loops = append(c.loops, false)
	c.block(b)
	c.loops = c.loops[:len(c.loops)-1]
}

func (c *checker) branch(s *ast.BranchStmt) {
	switch {
	case s.Label != nil:
		c.unsupported(s, "a labeled "+s.Tok.String())
	case s.Tok == token.FALLTHROUGH:
		c.unsupported(s, "fallthrough")
	case s.Tok == token.BREAK && len(c.loops) > 0 && c.loops[len(c.loops)-1] && !c.trailing[s]:
		c.unsupported(s, "a break that leaves a switch before the end of its case")
	}
}

func (c *checker) rangeStmt(s *ast.RangeStmt) {
	c.expr(s.X)
	t := c.pkg.Info.Types[s.X].Type
	switch t.Underlying().(type) {
	case *types.Slice, *types.Basic: // a slice, a string or an integer or an integer; go/types rejects the rest
	case *types.Map:
		if what := c.mapRangeProblem(s); what != "" { // RFC 0019, step 3
			c.unsupported(s.X, what)
			return
		}
	case *types.Signature:
		c.unsupported(s.X, "range over a function (Go 1.23)")
		return
	default:
		kind := map[string]string{"*types.Map": "a map", "*types.Chan": "a channel", "*types.Array": "an array", "*types.Pointer": "a pointer to an array"}[fmt.Sprintf("%T", t.Underlying())]
		c.unsupported(s.X, fmt.Sprintf("range over %s (%s)", kind, t))
		return
	}
	if s.Tok == token.ASSIGN {
		c.unsupported(s, "a range loop that assigns to existing variables (use :=)")
		return
	}
	for _, e := range []ast.Expr{s.Key, s.Value} {
		if id, ok := e.(*ast.Ident); ok {
			if obj := c.pkg.Info.Defs[id]; obj != nil {
				c.localType(id, obj.Type())
			}
		}
	}
	c.loopBody(s.Body, false)
}

func (c *checker) switchStmt(s *ast.SwitchStmt) {
	if s.Init != nil {
		c.stmt(s.Init)
	}
	if s.Tag != nil {
		c.expr(s.Tag)
		if IsIface(c.pkg.Info.TypeOf(s.Tag)) {
			c.unsupported(s.Tag, "a switch on an interface value (use a type switch)")
		}
	}
	c.loops = append(c.loops, true)
	for _, cc := range s.Body.List {
		cc := cc.(*ast.CaseClause)
		for _, e := range cc.List {
			c.expr(e)
		}
		if n := len(cc.Body); n > 0 {
			if b, ok := cc.Body[n-1].(*ast.BranchStmt); ok {
				c.trailing[b] = true
			}
		}
		for _, st := range cc.Body {
			c.stmt(st)
		}
	}
	c.loops = c.loops[:len(c.loops)-1]
}

func (c *checker) assign(s *ast.AssignStmt) {
	if s.Tok == token.ASSIGN && len(s.Lhs) == 1 && c.body != nil {
		if id, ok := ast.Unparen(s.Lhs[0]).(*ast.Ident); ok {
			v, _ := c.pkg.Info.Uses[id].(*types.Var)
			if cl := Recursive(c.pkg)[v]; cl != nil && ast.Unparen(s.Rhs[0]) == cl.Lit {
				c.recursiveDecl(s, cl, c.body) // var f func...; f = func... (RFC 0018, step 4)
				return
			}
		}
	}
	if lit, ok := closureLit(s); ok && c.inClosure == nil && c.body != nil {
		c.closureDecl(s, lit, c.body) // f := func(...) {...} (RFC 0018)
		return
	}
	if s.Tok == token.DEFINE {
		for _, l := range s.Lhs {
			if id := l.(*ast.Ident); c.pkg.Info.Defs[id] != nil && !FuncVars(c.pkg)[c.pkg.Info.Defs[id].(*types.Var)] {
				c.localType(id, c.pkg.Info.Defs[id].Type())
			}
		}
	} else {
		for _, l := range s.Lhs {
			c.lvalue(l)
		}
		if s.Tok != token.ASSIGN && len(s.Lhs) == 1 {
			if what := mapUpdateProblem(c.pkg.Info, s.Lhs[0], s.Tok); what != "" {
				c.unsupported(s, what)
				return
			}
		}
	}
	if len(s.Lhs) == 1 && len(s.Rhs) == 1 && s.Tok == token.ASSIGN {
		if call, ok := ast.Unparen(s.Rhs[0]).(*ast.CallExpr); ok && len(call.Args) > 0 && simplePlace(s.Lhs[0]) && types.ExprString(s.Lhs[0]) == types.ExprString(call.Args[0]) {
			c.selfAppend[call] = true
		}
	}
	for _, r := range s.Rhs {
		c.expr(r)
	}
}

// lvalue checks the left side of an assignment.
func (c *checker) lvalue(e ast.Expr) {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
	case *ast.SelectorExpr:
		c.lvalue(e.X)
	case *ast.IndexExpr: // go/types rejects assigning to a byte of a string
		c.lvalue(e.X)
		c.expr(e.Index)
	case *ast.StarExpr:
		c.expr(e.X)
	case *ast.TypeAssertExpr:
		c.unsupported(e, "a type assertion in an assignment target")
	default:
		c.unsupported(e, "this assignment target")
	}
}

func isPointer(t types.Type) bool {
	_, ok := t.(*types.Pointer)
	return ok
}

func isString(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsString != 0
}

func (c *checker) exprs(es []ast.Expr) {
	for _, e := range es {
		c.expr(e)
	}
}

func (c *checker) expr(e ast.Expr) {
	info := c.pkg.Info
	if tv, ok := info.Types[e]; ok && tv.Value != nil {
		if tv.Value.Kind() == constant.String && !utf8.ValidString(constant.StringVal(tv.Value)) {
			c.unsupported(e, "a string constant that is not valid UTF-8")
		}
		if tv.Value.Kind() == constant.Complex {
			c.unsupported(e, "a complex number")
		}
		return // constants are folded; their parts need no checking
	}
	switch e := e.(type) {
	case *ast.Ident:
		c.ident(e)
	case *ast.ParenExpr:
		c.expr(e.X)
	case *ast.CompositeLit:
		c.compositeLit(e)
	case *ast.SelectorExpr:
		c.selector(e, false)
	case *ast.IndexExpr:
		if _, ok := info.Types[e.X].Type.Underlying().(*types.Signature); ok {
			c.unsupported(e, "an instantiated generic function (Go 1.18)")
			return
		}
		c.expr(e.X)
		c.expr(e.Index)
	case *ast.CallExpr:
		c.call(e, false)
	case *ast.StarExpr:
		c.expr(e.X)
	case *ast.UnaryExpr:
		c.unary(e)
	case *ast.BinaryExpr:
		c.binary(e)
	case *ast.KeyValueExpr:
		c.expr(e.Value)
	case *ast.FuncLit:
		if Returned(c.pkg)[e] != nil {
			return // checked by funcResult (step 3)
		}
		// One inside a closure was refused with its closure (closureProblem).
		c.unsupported(e, "a function literal (closure) other than f := func(...) {...}, an argument or a returned value (RFC 0018)")
	case *ast.SliceExpr:
		c.unsupported(e, "a slice expression (s[i:j])")
	case *ast.TypeAssertExpr:
		c.typeAssert(e) // RFC 0012
	}
}

func (c *checker) ident(id *ast.Ident) {
	switch c.pkg.Info.Uses[id].(type) {
	case *types.Func:
		c.unsupported(id, fmt.Sprintf("using function %s as a value", id.Name))
	case *types.Nil:
		// nil is a nil error or an empty slice: pointers are never nil,
		// since they start as &x, and nil is never passed for one.
	case *types.Var:
		// A package-level variable (RFC 0010): its uses are checked by
		// checkGlobals, which knows what encloses them.
	}
}

// localSelector checks p.Name for a package p of the program: a function
// (called, not as a value), a constant or a type, not a package-level
// variable.
func (c *checker) localSelector(e *ast.SelectorExpr, name string, called bool) {
	switch c.pkg.Info.Uses[e.Sel].(type) {
	case *types.Func:
		if !called {
			c.unsupported(e, fmt.Sprintf("using %s as a value", name))
		}
	}
}

func (c *checker) selector(e *ast.SelectorExpr, called bool) {
	info := c.pkg.Info
	if id, ok := e.X.(*ast.Ident); ok {
		if pn, ok := info.Uses[id].(*types.PkgName); ok {
			name := pn.Imported().Path() + "." + e.Sel.Name
			if c.pkg.Local[pn.Imported()] {
				c.localSelector(e, name, called)
				return
			}
			switch info.Uses[e.Sel].(type) {
			case *types.Func:
				if !StdFuncs[name] {
					c.unsupported(e, name)
				} else if !called {
					c.unsupported(e, fmt.Sprintf("using %s as a value", name))
				}
			default: // a variable: constants are folded, types are not expressions
				if !StdVars[name] {
					c.unsupported(e, name)
				}
			}
			return
		}
	}
	sel := info.Selections[e]
	switch sel.Kind() {
	case types.MethodVal:
		if !called {
			c.unsupported(e, "a method value")
		}
		if fn, name := ProvidedMethod(sel.Obj().(*types.Func)); name != "" && fn == "" {
			c.unsupported(e, name)
		}
		if isWriter(sel.Recv()) {
			c.unsupported(e, "calling methods on an io.Writer (use fmt.Fprint, Fprintln or Fprintf)")
		}
	case types.MethodExpr:
		c.unsupported(e, "a method expression")
		return
	case types.FieldVal:
		if len(sel.Index()) > 1 {
			c.unsupported(e, "a promoted field")
		}
	}
	c.expr(e.X)
}

func (c *checker) compositeLit(e *ast.CompositeLit) {
	t := pointee(c.pkg.Info.Types[e].Type) // {...} for &T{...} in a literal of pointers
	switch t.Underlying().(type) {
	case *types.Struct:
		if _, ok := types.Unalias(t).(*types.Named); !ok {
			c.unsupported(e, "an anonymous struct type")
			return
		}
	case *types.Map:
		if what := mapTypeProblem(t.Underlying().(*types.Map)); what != "" {
			c.unsupported(e, what)
			return
		}
		if !c.mapInit(e) {
			return
		}
	case *types.Slice:
		c.typ(e, t, false)
		for _, el := range e.Elts {
			if _, ok := el.(*ast.KeyValueExpr); ok {
				c.unsupported(el, "an indexed slice literal element")
				return
			}
		}
	default:
		c.typ(e, t, false)
		return
	}
	c.exprs(e.Elts)
}

func (c *checker) unary(e *ast.UnaryExpr) {
	switch e.Op {
	case token.SUB, token.NOT, token.XOR, token.ADD:
		c.expr(e.X)
	case token.AND:
		switch x := ast.Unparen(e.X).(type) {
		case *ast.Ident:
			c.expr(x)
		case *ast.CompositeLit:
			c.expr(x)
		case *ast.SelectorExpr:
			c.expr(x)
		default:
			c.unsupported(e, "taking the address of this expression")
		}
	default:
		c.unsupported(e, "the "+e.Op.String()+" operator")
	}
}

func (c *checker) binary(e *ast.BinaryExpr) {
	c.expr(e.X)
	c.expr(e.Y)
	switch e.Op {
	case token.EQL, token.NEQ:
		xt, yt := c.pkg.Info.Types[e.X].Type, c.pkg.Info.Types[e.Y].Type
		for _, t := range []types.Type{xt, yt} {
			if _, ok := t.Underlying().(*types.Slice); ok {
				c.unsupported(e, "comparing a slice with nil")
				return
			}
		}

	}
}

func (c *checker) call(e *ast.CallExpr, standalone bool) {
	info := c.pkg.Info
	fun := ast.Unparen(e.Fun)
	if tv := info.Types[fun]; tv.IsType() {
		c.conversion(e, tv.Type)
		return
	}
	if e.Ellipsis.IsValid() {
		if id, ok := fun.(*ast.Ident); !ok || id.Name != "append" {
			c.unsupported(e, "a call with ... arguments")
			return
		}
	}
	switch f := fun.(type) {
	case *ast.Ident:
		if b, ok := info.Uses[f].(*types.Builtin); ok {
			c.builtin(e, b.Name())
			return
		}
		if _, ok := info.Uses[f].(*types.Func); !ok {
			if v, ok := info.Uses[f].(*types.Var); ok && (Closures(c.pkg)[v] != nil || FuncParams(c.pkg)[v] != nil || FuncVars(c.pkg)[v] || Recursive(c.pkg)[v] != nil) {
				c.exprs(e.Args) // a closure's or a function parameter's call (RFC 0018)
				return
			}
			c.unsupported(e, "calling a function value")
			return
		}
	case *ast.SelectorExpr:
		c.selector(f, true)
		if pn, ok := f.X.(*ast.Ident); ok {
			if p, ok := info.Uses[pn].(*types.PkgName); ok && !c.pkg.Local[p.Imported()] {
				c.stdCall(e, p.Imported().Path()+"."+f.Sel.Name)
				return
			}
		}
		if _, ok := info.Uses[f.Sel].(*types.Func); ok && info.Selections[f] == nil {
			// A function of another package of the program (RFC 0009).
		} else if sel := info.Selections[f]; sel == nil || sel.Kind() != types.MethodVal {
			c.unsupported(e, "calling a function value")
			return
		}
	case *ast.IndexExpr, *ast.IndexListExpr:
		c.unsupported(e, "an instantiated generic function (Go 1.18)")
		return
	default:
		c.unsupported(e, "calling a function value")
		return
	}
	sig := info.Types[fun].Type.(*types.Signature)
	if standalone && c.sliceSuffix(e, fun, sig) {
		return
	}
	// nil passed for a pointer is a heap pointer (RFC 0013).
	c.programArgs(e)
}

func (c *checker) stdCall(e *ast.CallExpr, name string) {
	if name == "errors.As" {
		c.expr(e.Args[0])
		c.errorsAs(e)
		return
	}
	c.exprs(e.Args)
	switch {
	case strings.HasPrefix(name, "fmt."):
		c.fmtCall(e, name)
	case strings.HasPrefix(name, "slices.Sort"), strings.HasPrefix(name, "slices.IsSorted"):
		if what := sortProblem(c.pkg.Info, e, name); what != "" {
			c.unsupported(e, what)
		}
	}
}

// sortProblem checks slices.Sort(s) or slices.IsSorted(s): s holds
// integers or strings, whose order Rust's sort gives as Go's does. Floats
// are refused: Go orders a NaN before every number, Rust's sort needs a
// total order.
func sortProblem(info *types.Info, e *ast.CallExpr, name string) string {
	elem := info.Types[e.Args[0]].Type.Underlying().(*types.Slice).Elem()
	if b, ok := elem.Underlying().(*types.Basic); ok && b.Info()&(types.IsInteger|types.IsString) != 0 {
		return ""
	}
	return fmt.Sprintf("%s of a slice of %s (patina mode sorts integers and strings)", name, elem)
}

func (c *checker) conversion(e *ast.CallExpr, to types.Type) {
	c.exprs(e.Args)
	from := c.pkg.Info.Types[e.Args[0]].Type
	switch {
	case isNumeric(to) && isNumeric(from):
		c.typ(e, to, false)
	case isByteSlice(to) && isString(from):
	case isString(to) && isByteSlice(from):
		c.unsupported(e, "converting a []byte to a string (patina strings must be valid UTF-8)")
	case isString(to) && isNumeric(from):
		c.unsupported(e, "converting an integer to a string")
	case IsIface(to):
		// To an interface (RFC 0012): any(x), Shape(c), any(nil); the
		// value's dynamic type is checked with every other conversion.
		c.typ(e, to, false)
	case types.IdenticalIgnoreTags(to.Underlying(), from.Underlying()):
		// Between a named type and its underlying type, or two named types
		// with the same underlying type (RFC 0007): the same value.
	default:
		c.unsupported(e, fmt.Sprintf("converting %s to %s", from, to))
	}
}

func isNumeric(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsNumeric != 0
}

func isInteger(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsInteger != 0
}

func isByteSlice(t types.Type) bool {
	s, ok := t.Underlying().(*types.Slice)
	if !ok {
		return false
	}
	b, ok := s.Elem().(*types.Basic)
	return ok && b.Kind() == types.Uint8
}

func (c *checker) builtin(e *ast.CallExpr, name string) {
	info := c.pkg.Info
	switch name {
	case "len":
		c.exprs(e.Args)
	case "append":
		if !c.selfAppend[e] {
			c.unsupported(e, "append whose result is not assigned back to its first operand (s = append(s, ...))")
			return
		}
		c.exprs(e.Args)
	case "make":
		if m, ok := info.Types[e.Args[0]].Type.Underlying().(*types.Map); ok {
			if len(e.Args) > 1 && info.Types[e.Args[1]].Value == nil {
				c.unsupported(e, "make of a map with a size that is not a constant (Go panics on a negative one)")
				return
			}
			if what := mapTypeProblem(m); what != "" {
				c.unsupported(e, what)
				return
			}
			c.mapInit(e)
			return
		}
		if _, ok := info.Types[e.Args[0]].Type.Underlying().(*types.Slice); !ok {
			c.unsupported(e, "make of "+info.Types[e.Args[0]].Type.String())
			return
		}
		c.typ(e, info.Types[e.Args[0]].Type, false)
		c.exprs(e.Args[1:])
	case "recover":
		if c.inClosure == nil || Deferred(c.pkg)[c.inClosure] == nil {
			c.unsupported(e, "recover other than directly in a deferred function literal (it returns nil there; RFC 0020)")
		}
	case "panic":
		if len(Recovering(c.pkg)) > 0 {
			if what := recoveredPanicProblem(info, e); what != "" {
				c.unsupported(e, what)
				return
			}
		}
		// Go prints a string; an error's Error() (Go's PanicNilError text
		// for a nil error); a Stringer's String(); a named string type as
		// T("s"). Other values print by the runtime's own rules, not yet.
		t := info.Types[e.Args[0]].Type
		switch {
		case isString(t):
		case isError(t):
			if what := c.printProblem(t); what != "" {
				c.unsupported(e, "panic with an error value: "+what)
				return
			}
		case PrintMethod(t) != nil:
			if what := c.methodPrintProblem(PrintMethod(t)); what != "" {
				c.unsupported(e, what)
				return
			}
		default:
			c.unsupported(e, "panic with a value that is not a string or an error")
			return
		}
		c.exprs(e.Args)
	case "min", "max":
		if len(e.Args) < 2 {
			c.unsupported(e, fmt.Sprintf("the %s built-in with fewer than two arguments", name))
			return
		}
		for _, a := range e.Args {
			if !isInteger(info.Types[a].Type) {
				c.unsupported(e, fmt.Sprintf("the %s built-in with a non-integer argument", name))
				return
			}
		}
		c.exprs(e.Args)
	case "delete":
		c.exprs(e.Args) // of a map (RFC 0019)
	case "clear":
		c.unsupported(e, fmt.Sprintf("the %s built-in (Go 1.21)", name))
	default:
		c.unsupported(e, fmt.Sprintf("the %s built-in", name))
	}
}

// fmtCall checks a call to a fmt function: its format string, if it has
// one, and that every operand can be printed.
func (c *checker) fmtCall(e *ast.CallExpr, name string) {
	info := c.pkg.Info
	args := e.Args
	if strings.HasPrefix(name, "fmt.F") {
		args = args[1:]
	}
	if strings.HasSuffix(name, "f") {
		tv := info.Types[args[0]]
		if tv.Value == nil {
			c.unsupported(args[0], "a format string that is not a constant")
			return
		}
		verbs, err := ParseFormat(constant.StringVal(tv.Value))
		if err != nil {
			c.errorf(args[0], "format string: %v", err)
			return
		}
		args = args[1:]
		n, wraps := 0, 0
		for _, v := range verbs {
			if v.Verb == 0 {
				continue
			}
			if n >= len(args) {
				c.errorf(e, "format string: %%%c has no operand", v.Verb)
				return
			}
			if v.Verb == 'w' {
				// RFC 0012: one wrapped error, a variable, since it is both
				// formatted and kept.
				wraps++
				if what := c.wrapProblem(v, args[n], name, wraps); what != "" {
					c.unsupported(args[n], what)
				}
				n++
				continue
			}
			if what := c.verbProblem(v, info.Types[args[n]].Type, name); what != "" {
				c.unsupported(args[n], what)
			}
			n++
		}
		if n != len(args) {
			c.errorf(e, "format string has %d verbs but %d operands are given", n, len(args))
		}
		return
	}
	for _, a := range args {
		if what := c.printProblem(info.Types[a].Type); what != "" {
			c.unsupported(a, what)
		} else if t := info.Types[a].Type; IsIface(t) && !strings.HasSuffix(name, "ln") && (!isError(t) || c.stringErrors()) {
			// Print and Sprint put a space between operands unless one is a
			// string, which for an interface depends on its dynamic type.
			c.unsupported(a, "an interface value printed with "+name+" (use "+name+"ln or Printf)")
		}
	}
}

// printProblem says why a value of type t cannot be printed with %v.
func (c *checker) printProblem(t types.Type) string {
	if IsIface(t) {
		return c.dynPointerPrintProblem(t) // through its dynamic type (RFC 0012)
	}
	if isPointer(t) {
		return c.ptrPrintProblem(t)
	}
	if s, ok := t.Underlying().(*types.Slice); ok && isPointer(s.Elem()) {
		return c.ptrPrintProblem(s.Elem()) // each element through its method
	}
	if m := PrintMethod(t); m != nil {
		return c.methodPrintProblem(m)
	}
	if isError(t) || isPrintableBasic(t) {
		return ""
	}
	if s, ok := t.Underlying().(*types.Slice); ok {
		if m := PrintMethod(s.Elem()); m != nil {
			return c.methodPrintProblem(m) // each element through its method
		}
		if isPrintableBasic(s.Elem()) {
			return ""
		}
	}
	if _, ok := t.Underlying().(*types.Struct); ok {
		return structPrintProblem(t)
	}
	if m, ok := t.Underlying().(*types.Map); ok && isPrintableBasic(m.Key()) && (isPrintableBasic(m.Elem()) || copyStruct(m.Elem()) && structPrintProblem(m.Elem()) == "") {
		return "" // keys sorted, as fmt prints them (RFC 0019)
	}
	return fmt.Sprintf("printing a value of type %s", t)
}

// structPrintProblem says why fmt's %v of a struct of type t, which has no
// String or Error method, cannot be printed yet. fmt prints {f1 f2 ...},
// each field by its own %v. The subset prints fields of the printable
// basic types, slices of them, and structs printed the same way. A field
// whose type has a String or Error method is not printed yet: fmt calls it
// for an exported field only, and Go's reflection decides which. Nor is a
// pointer field (fmt prints its address), an interface, or anything else.
func structPrintProblem(t types.Type) string {
	st := t.Underlying().(*types.Struct)
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i).Type()
		switch {
		case PrintMethod(f) != nil:
			return fmt.Sprintf("printing a struct of type %s, whose field %s fmt prints through its %s method", t, st.Field(i).Name(), PrintMethod(f).Name())
		case isPrintableBasic(f):
		case isStruct(f):
			if what := structPrintProblem(f); what != "" {
				return what
			}
		default:
			if s, ok := f.Underlying().(*types.Slice); ok && isPrintableBasic(s.Elem()) && PrintMethod(s.Elem()) == nil {
				continue
			}
			return fmt.Sprintf("printing a struct of type %s, whose field %s is of type %s", t, st.Field(i).Name(), f)
		}
	}
	return ""
}

// plainQuote reports whether %q directive v has only a width and the -
// flag, which pad the quoted string as Go does.
func plainQuote(v Verb) bool {
	return v.Prec < 0 && !v.Plus && !v.Sharp && !v.Zero && !v.Space
}

// PrintMethod returns the method fmt calls to print a value of type t with
// %v or %s: Error() string, else String() string, from the value's method
// set (fmt receives a copy, so a pointer-receiver method is not used), or
// nil. Only named types of the package have methods in the subset.
func PrintMethod(t types.Type) *types.Func {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok || n.Obj().Pkg() == nil || n.Obj().Pkg().Path() == "io" {
		return nil
	}
	ms := types.NewMethodSet(n)
	for _, name := range []string{"Error", "String"} {
		sel := ms.Lookup(n.Obj().Pkg(), name)
		if sel == nil {
			continue
		}
		f := sel.Obj().(*types.Func)
		sig := f.Signature()
		if sig.Params().Len() == 0 && sig.Results().Len() == 1 && isString(sig.Results().At(0).Type()) {
			return f
		}
	}
	return nil
}

// PtrPrintMethod returns the method fmt calls to print a pointer of type
// t, *T for a named type T of the program: Error() string, else String()
// string, from *T's method set, which holds T's methods too; or nil.
func PtrPrintMethod(t types.Type) *types.Func {
	p := types.Unalias(t).(*types.Pointer) // the callers pass only pointers
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	if !ok || n.Obj().Pkg() == nil {
		return nil
	}
	ms := types.NewMethodSet(p)
	for _, name := range []string{"Error", "String"} {
		if sel := ms.Lookup(n.Obj().Pkg(), name); sel != nil {
			f := sel.Obj().(*types.Func)
			sig := f.Signature()
			if sig.Params().Len() == 0 && sig.Results().Len() == 1 && isString(sig.Results().At(0).Type()) {
				return f
			}
		}
	}
	return nil
}

// dynPointerPrintProblem says why a value of interface type t cannot be
// printed in this program: a dynamic type it can hold is a pointer fmt
// would print by its address, or prints through a method that may panic.
func (c *checker) dynPointerPrintProblem(t types.Type) string {
	if c.ifacePlan == nil {
		c.ifacePlan = Ifaces(c.pkg)
	}
	for _, d := range c.ifacePlan.Dyn {
		if isPointer(d) && types.AssignableTo(d, t) { // a value of t can hold it
			if what := c.ptrPrintProblem(d); what != "" {
				return "printing an interface value in a program that converts " + d.String() + " to one: " + what
			}
		}
	}
	return ""
}

// ptrPrintProblem says why a pointer of type t cannot be printed: only
// through its print method, which may dereference the pointer (fmt prints
// <nil> when that panics for a nil pointer, as patina mode does too).
func (c *checker) ptrPrintProblem(t types.Type) string {
	m := PtrPrintMethod(t)
	if m == nil {
		return fmt.Sprintf("printing a pointer of type %s, which has no String or Error method (fmt prints &{...} for a pointer to a struct, and an address for any other or inside another value)", t)
	}
	saved := c.nilRecv
	c.nilRecv = m.Signature().Recv()
	defer func() { c.nilRecv = saved }()
	return c.methodPrintProblem(m)
}

// methodPrintProblem says why printing through method m is not supported
// yet: Go's fmt recovers a panic in it and prints %!v(PANIC=...), which
// patina mode does not reproduce yet, so m must be unable to panic.
func (c *checker) methodPrintProblem(m *types.Func) string {
	k := printKey{m, c.nilRecv}
	if what, ok := c.printProblems[k]; ok {
		return what
	}
	if c.printProblems == nil {
		c.printProblems = map[printKey]string{}
		c.methodDecls = map[*types.Func]*ast.FuncDecl{}
		for _, f := range c.pkg.Files {
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv != nil {
					if fn, ok := c.pkg.Info.Defs[fd.Name].(*types.Func); ok {
						c.methodDecls[fn] = fd
					}
				}
			}
		}
	}
	what := c.methodPrintProblemOf(m)
	c.printProblems[k] = what
	return what
}

// A printKey is what methodPrintProblem's answer depends on.
type printKey struct {
	m       *types.Func
	nilRecv *types.Var
}

func (c *checker) methodPrintProblemOf(m *types.Func) string {
	if fn, _ := ProvidedMethod(m); fn != "" {
		return "" // a provided method cannot panic
	}
	// A method without a body is reported where it is declared.
	if fd := c.methodDecls[m]; fd != nil && fd.Body != nil && c.cannotPanic(fd.Body) {
		return ""
	}
	return fmt.Sprintf("printing a value whose %s method may panic", m.Name())
}

// cannotPanic reports whether n is built only from constructs that cannot
// panic: if, switch, return and blocks; constants, variables, conversions
// between basic types, len, and operators other than / and %; and
// fmt.Sprint, Sprintf and Sprintln of such values, none of which has a
// String or Error method itself. It is deliberately narrow.
func (c *checker) cannotPanic(n ast.Node) bool {
	ok := true
	ast.Inspect(n, func(n ast.Node) bool {
		if !ok || n == nil {
			return false
		}
		switch n := n.(type) {
		case *ast.BlockStmt, *ast.ReturnStmt, *ast.IfStmt, *ast.SwitchStmt, *ast.CaseClause,
			*ast.ExprStmt, *ast.BasicLit, *ast.Ident, *ast.ParenExpr, *ast.UnaryExpr:
			return true
		case *ast.BinaryExpr:
			ok = n.Op != token.QUO && n.Op != token.REM && n.Op != token.SHL && n.Op != token.SHR
			return ok
		case *ast.SelectorExpr:
			// Reading a field of a value cannot panic; through a pointer
			// it could.
			sel := c.pkg.Info.Selections[n]
			ok = sel != nil && sel.Kind() == types.FieldVal && (!isPointer(c.pkg.Info.Types[n.X].Type) || c.isNilRecv(n.X))
			return ok
		case *ast.CallExpr:
			// The callee is a type, a built-in or fmt's function, not a
			// value; only the arguments are visited.
			ok = c.cannotPanicCall(n)
			for _, a := range n.Args {
				ok = ok && c.cannotPanic(a)
			}
			return false
		}
		ok = false
		return false
	})
	return ok
}

// isNilRecv reports whether e is the pointer receiver of the print method
// being checked, whose nil dereference fmt prints as <nil>.
func (c *checker) isNilRecv(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && c.nilRecv != nil && c.pkg.Info.Uses[id] == c.nilRecv
}

func (c *checker) cannotPanicCall(e *ast.CallExpr) bool {
	info := c.pkg.Info
	if tv := info.Types[e.Fun]; tv.IsType() {
		_, basic := tv.Type.Underlying().(*types.Basic)
		_, fromBasic := info.Types[e.Args[0]].Type.Underlying().(*types.Basic)
		return basic && fromBasic
	}
	switch f := ast.Unparen(e.Fun).(type) {
	case *ast.Ident:
		if b, ok := info.Uses[f].(*types.Builtin); ok && b.Name() == "len" {
			return true
		}
	case *ast.SelectorExpr:
		if pn, ok := f.X.(*ast.Ident); ok {
			if p, ok := info.Uses[pn].(*types.PkgName); ok && p.Imported().Path() == "fmt" &&
				(f.Sel.Name == "Sprint" || f.Sel.Name == "Sprintf" || f.Sel.Name == "Sprintln") {
				for _, a := range e.Args {
					if PrintMethod(info.Types[a].Type) != nil {
						return false
					}
				}
				return true
			}
		}
	}
	return false
}

func isPrintableBasic(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&(types.IsInteger|types.IsFloat|types.IsString|types.IsBoolean) != 0
}

// verbProblem says why verb v cannot format a value of type t, or returns
// "". fn is the fmt function being called.
func (c *checker) verbProblem(v Verb, t types.Type, fn string) string {
	if isError(t) && v.Verb == 'T' && v.Width < 0 && v.Prec < 0 && !v.Plus && !v.Minus && !v.Zero && !v.Space && !v.Sharp {
		return ""
	}
	if isError(t) && v.Prec >= 0 {
		// Go truncates an error's text but not <nil>.
		return fmt.Sprintf("the format %s with an error value", v.Spec)
	}
	if IsIface(t) && !isError(t) {
		if (v.Verb == 'v' || v.Verb == 'T') && v.Width < 0 && v.Prec < 0 && !v.Plus && !v.Minus && !v.Zero && !v.Space && !v.Sharp {
			if v.Verb == 'v' {
				return c.printProblem(t) // through its dynamic types, as Println's operands are
			}
			return ""
		}
		return fmt.Sprintf("the format %s with an interface value", v.Spec)
	}
	if m := PrintMethod(t); m != nil {
		switch v.Verb {
		case 'v', 's':
			if v.Plus || v.Zero || v.Space || v.Sharp {
				return fmt.Sprintf("the format %s with a value of type %s", v.Spec, t)
			}
			return c.methodPrintProblem(m)
		case 'x', 'X':
			// fmt prints the method's result in hex.
			if v.Plus || v.Space || v.Prec >= 0 {
				return fmt.Sprintf("the format %s with a value of type %s", v.Spec, t)
			}
			return c.methodPrintProblem(m)
		case 'q':
			// fmt quotes the method's result.
			if !plainQuote(v) {
				return fmt.Sprintf("the format %s with a value of type %s", v.Spec, t)
			}
			return c.methodPrintProblem(m)
		}
	}
	b, _ := t.Underlying().(*types.Basic)
	kind := func(mask types.BasicInfo) bool { return b != nil && b.Info()&mask != 0 }
	ok := false
	switch v.Verb {
	case 'v':
		ok = c.printProblem(t) == "" && !v.Zero && v.Prec < 0
		if isStruct(t) && (v.Width >= 0 || v.Plus || v.Minus || v.Sharp) {
			// fmt pads each field, and %+v and %#v print names and Go syntax.
			ok = false
		}
	case 'd':
		ok = kind(types.IsInteger) && v.Prec < 0
	case 'x', 'X':
		// Integers, and the bytes of a string or []byte, with the - 0 and #
		// flags and a width.
		ok = (kind(types.IsInteger) || kind(types.IsString) || isByteSlice(t)) && v.Prec < 0 && !v.Plus
	case 'c':
		ok = kind(types.IsInteger) && v.Prec < 0 && !v.Plus && !v.Zero
	case 's':
		ok = (kind(types.IsString) || isError(t) || c.methodSlice(t) || (isByteSlice(t) && v.Width < 0 && v.Prec < 0 && fn != "fmt.Sprintf" && fn != "fmt.Errorf")) && !v.Plus && !v.Zero
	case 't':
		ok = kind(types.IsBoolean) && v.Prec < 0 && !v.Plus && !v.Zero
	case 'q':
		// strconv.Quote of a string, padded to a width as Go pads it, by
		// runes. Not yet %q of a rune or a []byte, nor %+q (ASCII only),
		// %#q (backquoted), %.Nq (truncated first) or 0 (zero padding).
		ok = kind(types.IsString) && plainQuote(v)
	case 'f', 'F':
		ok = kind(types.IsFloat)
	case 'g':
		// Go's %g of a float is its %v: the shortest digits, an exponent
		// below 1e-4 and from 1e+06 up. Not yet a precision, which counts
		// digits there, nor the + flag, which %g honours (+1e+06, +NaN)
		// and %v does not.
		ok = kind(types.IsFloat) && !v.Zero && !v.Plus && v.Prec < 0
	}
	if !ok || v.Space || (v.Sharp && v.Verb != 'x' && v.Verb != 'X') {
		return fmt.Sprintf("the format %s with a value of type %s", v.Spec, t)
	}
	return ""
}

// methodSlice reports whether t is a slice whose elements fmt prints
// through a String or Error method that cannot panic.
func (c *checker) methodSlice(t types.Type) bool {
	s, ok := t.Underlying().(*types.Slice)
	if !ok {
		return false
	}
	m := PrintMethod(s.Elem())
	return m != nil && c.methodPrintProblem(m) == ""
}

// simplePlace reports whether e is a variable or a field of one, possibly
// through a pointer: a place append can grow in place.
func simplePlace(e ast.Expr) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return simplePlace(x.X)
	case *ast.StarExpr:
		return simplePlace(x.X)
	}
	return false
}

// localPath reports whether the program translates the package at path
// from source (RFC 0009).
func (c *checker) localPath(path string) bool {
	for p := range c.pkg.Local {
		if p.Path() == path {
			return true
		}
	}
	return false
}

// deferStmt checks a defer statement. Patina mode defers fmt.Print,
// Println and Printf of values whose text cannot change by the time the
// call runs (numbers, strings and booleans, without a String method): Go
// evaluates a deferred call's arguments at the defer statement, so their
// text is made then, and those calls cannot panic, which would be a
// second panic while one unwinds.
func (c *checker) deferStmt(s *ast.DeferStmt) {
	if lit, ok := ast.Unparen(s.Call.Fun).(*ast.FuncLit); ok {
		if len(s.Call.Args) > 0 {
			c.unsupported(s, "deferring a function literal with arguments (step 5 of RFC 0018 takes func() {...}())")
			return
		}
		c.deferredClosure(s, Deferred(c.pkg)[lit], c.body) // RFC 0018, step 5
		return
	}
	sel, ok := s.Call.Fun.(*ast.SelectorExpr)
	name := ""
	if ok {
		if id, ok := sel.X.(*ast.Ident); ok {
			if pn, ok := c.pkg.Info.Uses[id].(*types.PkgName); ok {
				name = pn.Imported().Path() + "." + sel.Sel.Name
			}
		}
	}
	switch name {
	case "fmt.Print", "fmt.Println", "fmt.Printf":
	default:
		c.unsupported(s, "defer of a call other than fmt.Print, Println and Printf")
		return
	}
	args := s.Call.Args
	if name == "fmt.Printf" && len(args) > 0 {
		args = args[1:]
	}
	for _, a := range args {
		t := c.pkg.Info.Types[a].Type
		if b, ok := t.Underlying().(*types.Basic); !ok || b.Info()&(types.IsNumeric|types.IsString|types.IsBoolean) == 0 || PrintMethod(t) != nil || b.Info()&types.IsComplex != 0 {
			c.unsupported(a, fmt.Sprintf("deferring the print of a value of type %s, whose text is made when the call runs,", t))
			return
		}
	}
	c.expr(s.Call)
}
