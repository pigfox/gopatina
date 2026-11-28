package emit

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"unicode/utf8"

	"github.com/pigfox/gopatina/internal/borrow"
	"github.com/pigfox/gopatina/internal/subset"
)

// closureDecl writes f := func(...) {...} (RFC 0018, step 1): a Rust
// closure, borrowing the variables it captures, mut when it changes one.
func (g *gen) closureDecl(s *ast.AssignStmt, lit *ast.FuncLit) {
	v := g.info.Defs[s.Lhs[0].(*ast.Ident)].(*types.Var)
	g.closureLet(g.varName(v), subset.Closures(g.pkg)[v], g.res.Closures[v])
}

// inlineClosures binds, just before statement s, each function literal s
// passes to a parameter of function type (RFC 0018, step 2), so the call
// lends it by name.
func (g *gen) inlineClosures(s ast.Stmt) {
	var lits []*ast.FuncLit
	ast.Inspect(s, func(n ast.Node) bool {
		if lit, ok := n.(*ast.FuncLit); ok {
			if _, done := g.litNames[lit]; !done && subset.Inline(g.pkg)[lit] != nil {
				lits = append(lits, lit) // not yet bound
			}
			return false
		}
		return true
	})
	for _, lit := range lits {
		name := g.lnames.name("", func(string) string { return "f" })
		if g.litNames == nil {
			g.litNames = map[*ast.FuncLit]string{}
		}
		g.litNames[lit] = name
		g.closureLet(name, subset.Inline(g.pkg)[lit], g.res.Lits[lit])
	}
}

// funcArg renders an argument of a parameter of function type: a function
// of the program by name; a closure that changes what it captures lent as
// &mut; any other closure by value, a copy (it holds only shared borrows,
// so it is Copy, and stays usable).
func (g *gen) funcArg(a ast.Expr) node {
	lend := func(name string, cl *subset.Closure) node {
		if len(cl.Writes) > 0 {
			return text("&mut " + name)
		}
		return text(name)
	}
	if lit, ok := ast.Unparen(a).(*ast.FuncLit); ok {
		return lend(g.litNames[lit], subset.Inline(g.pkg)[lit])
	}
	if sel, ok := ast.Unparen(a).(*ast.SelectorExpr); ok {
		return g.mathFuncValue(subset.MathFuncValue(g.info, sel))
	}
	// The checker allows only these and a function of the program.
	id := ast.Unparen(a).(*ast.Ident)
	if v, ok := g.info.Uses[id].(*types.Var); ok {
		if cl := subset.Closures(g.pkg)[v]; cl != nil {
			return lend(g.varName(v), cl)
		}
		if subset.Recursive(g.pkg)[v] != nil {
			return text(g.varName(v)) // a nested fn (step 4)
		}
		return text("&mut " + g.varName(v)) // a Box<dyn FnMut> (step 3)
	}
	fn := g.info.Uses[id].(*types.Func)
	return text(g.qual(fn.Pkg(), g.funcs[fn]))
}

// fnTrait renders sig as the trait of a generic parameter: impl Fn(T) -> R,
// or impl FnMut when mut.
func (g *gen) fnTrait(sig *types.Signature, mut bool) string {
	var ps []string
	for i := 0; i < sig.Params().Len(); i++ {
		t := sig.Params().At(i).Type()
		if isStringType(t) {
			ps = append(ps, "&str") // a closure borrows a string it is passed
			continue
		}
		ps = append(ps, g.rustType(t))
	}
	trait := "impl Fn("
	if mut {
		trait = "impl FnMut("
	}
	trait += strings.Join(ps, ", ") + ")"
	if sig.Results().Len() == 1 {
		trait += " -> " + g.rustType(sig.Results().At(0).Type())
	}
	return trait
}

// closureLet writes let name = |...| {...}; for closure cl, whose body the
// borrow checker checked as fn: mut when it changes what it captures.
func (g *gen) closureLet(name string, cl *subset.Closure, fn *borrow.Func) {
	head := "let "
	if len(cl.Writes) > 0 {
		head += "mut "
	}
	g.closureExpr(head+name+" = ", fn, ";")
}

// returnClosure writes return func(...) {...} of a function whose result is
// of function type (RFC 0018, step 3): Box::new(move |...| ...), the
// closure owning what it captures, moved in. tail: the function's last
// statement. It reports whether r is one.
func (g *gen) returnClosure(r *ast.ReturnStmt, tail bool) bool {
	if len(r.Results) != 1 {
		return false
	}
	lit, ok := ast.Unparen(r.Results[0]).(*ast.FuncLit)
	if !ok || subset.Returned(g.pkg)[lit] == nil {
		return false
	}
	head, end := "Box::new(move ", ")"
	if !tail {
		head, end = "return "+head, ");"
	}
	g.closureExpr(head, g.res.Lits[lit], end)
	return true
}

// closureExpr writes head + |...| {...} + end, for the closure whose body
// the borrow checker checked as fn. Its body is written as that function,
// its returns its own.
func (g *gen) closureExpr(prefix string, fn *borrow.Func, end string) {
	sig := fn.Obj.Signature()
	params := g.closureParams(fn)
	head := prefix + "|" + strings.Join(params, ", ") + "|"
	if sig.Results().Len() == 1 {
		head += " -> " + g.rustType(sig.Results().At(0).Type())
	}
	savedFn, savedRecv, savedStart, savedBody, savedDeferred := g.fn, g.recv, g.fnStart, g.fnBody, g.deferred
	savedOut := g.out
	var body strings.Builder
	g.out = &body
	g.fn, g.recv = fn, nil
	g.ind++
	if list := fn.Decl.Body.List; len(list) == 1 {
		g.bareIf, _ = list[0].(*ast.IfStmt)
	}
	g.funcBody(fn.Decl, fn.Obj, sig, fn.Params)
	g.bareIf = nil
	g.ind--
	g.fn, g.recv, g.fnStart, g.fnBody, g.deferred = savedFn, savedRecv, savedStart, savedBody, savedDeferred
	g.out = savedOut
	// rustfmt writes an empty body as {}, and a body that is one
	// expression on the closure's own line when it fits.
	text := body.String()
	if text == "" {
		g.line(head + " {}" + end)
		return
	}
	if expr := strings.TrimSpace(text); !strings.Contains(expr, "\n") && !strings.HasSuffix(expr, ";") && !strings.Contains(expr, "//") {
		if one := head + " { " + expr + " }" + end; g.ind*indentWidth+utf8.RuneCountInString(one) <= maxWidth {
			g.line(one)
			return
		}
	}
	g.line(head + " {")
	g.out.WriteString(text)
	g.line("}" + end)
}

// recursiveFn writes f = func(...) {...} of a recursive closure (RFC 0018,
// step 4) as a nested fn, which can call itself: it captures nothing. It
// reports whether s is one.
func (g *gen) recursiveFn(s *ast.AssignStmt) bool {
	if s.Tok != token.ASSIGN || len(s.Lhs) != 1 {
		return false
	}
	id, ok := ast.Unparen(s.Lhs[0]).(*ast.Ident)
	if !ok {
		return false
	}
	v, _ := g.info.Uses[id].(*types.Var)
	if subset.Recursive(g.pkg)[v] == nil {
		return false
	}
	fn := g.res.Closures[v]
	sig := fn.Obj.Signature()
	params := g.closureParams(fn)
	head := "fn " + g.varName(v) + "(" + strings.Join(params, ", ") + ")"
	if sig.Results().Len() == 1 {
		head += " -> " + g.rustType(sig.Results().At(0).Type())
	}
	g.line(head + " {")
	savedFn, savedRecv, savedStart, savedBody, savedDeferred := g.fn, g.recv, g.fnStart, g.fnBody, g.deferred
	g.fn, g.recv = fn, nil
	g.ind++
	g.funcBody(fn.Decl, fn.Obj, sig, fn.Params)
	g.ind--
	g.fn, g.recv, g.fnStart, g.fnBody, g.deferred = savedFn, savedRecv, savedStart, savedBody, savedDeferred
	g.line("}")
	return true
}

// closureParams renders the parameters of the closure whose body the
// borrow checker checked as fn: name: type, mut where the body changes one.
func (g *gen) closureParams(fn *borrow.Func) []string {
	var params []string
	for _, p := range fn.Params {
		pname := "_"
		if p.Var.Name() != "_" && p.Var.Name() != "" {
			pname = g.varName(p.Var)
		}
		if p.Mut {
			pname = "mut " + pname
		}
		params = append(params, pname+": "+g.paramType(p.Var.Type(), p.Mode))
	}
	return params
}
