package emit

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/pigfox/gopatina/internal/borrow"
	"github.com/pigfox/gopatina/internal/subset"
)

// Package-level variables (RFC 0010).

// globalOf returns the package-level variable e names (v or pkg.V), or nil.
func (g *gen) globalOf(e ast.Expr) *types.Var {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		return subset.Global(g.info.Uses[x])
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			if _, pkg := g.info.Uses[id].(*types.PkgName); pkg {
				return subset.Global(g.info.Uses[x.Sel])
			}
		}
	}
	return nil
}

// localStatic reports whether v, a static, holds an interface value and is
// a thread-local: an interface value's reference count (Rc) is not shared
// between threads, which a static must be. This is the one place that
// changes when RFC 0002 makes it shareable (Arc): the static becomes a
// LazyLock like any other (RFC 0012).
func (g *gen) localStatic(v *types.Var) bool {
	return g.globals[v] == subset.GlobalStatic && subset.IsIface(v.Type())
}

// constStatic reports whether v, a package-level variable that nothing
// changes, holds a number or a boolean given by a constant: a plain Rust
// static, as a person writes it, with no LazyLock and nothing to do at
// start-up (its initializer has no effect to order).
func (g *gen) constStatic(v *types.Var) bool {
	if g.globals[v] != subset.GlobalStatic || g.localStatic(v) {
		return false
	}
	b, ok := v.Type().Underlying().(*types.Basic)
	if !ok || b.Info()&(types.IsNumeric|types.IsBoolean) == 0 {
		return false
	}
	init := g.globalInit[v]
	return init != nil && g.info.Types[init].Value != nil
}

// globalName is v's Rust name, qualified for the package being written.
func (g *gen) globalName(v *types.Var) string { return g.qual(v.Pkg(), g.gnames[v]) }

// globalValue renders a read of v as a value.
func (g *gen) globalValue(v *types.Var) node {
	if a, ok := g.galias[v]; ok {
		if borrow.KindOf(v.Type()) == borrow.Copy {
			return text(a)
		}
		return &call{recv: text(a), head: ".clone", open: "(", close: ")"}
	}
	name := g.globalName(v)
	switch g.globals[v] {
	case subset.GlobalCell:
		return &call{recv: text(name), head: ".get", open: "(", close: ")"}
	case subset.GlobalString:
		return &call{recv: text(name), head: ".with_borrow", open: "(", close: ")", args: []node{text("|v| v.clone()")}}
	}
	if g.localStatic(v) {
		return &call{recv: text(name), head: ".with", open: "(", close: ")", args: []node{text("Clone::clone")}}
	}
	if g.constStatic(v) {
		return text(name) // a plain static, read as itself
	}
	// A static is read in place: a copy of a copied kind, else a place
	// that owned and the borrowing reads take as they need (isPlace). It
	// is parenthesized only where a deref needs it, as (*S).f.
	return atom{"*" + name, precUnary}
}

// globalPlace renders v as a place: a static, or the local a changed
// variable is read into for a statement that changes it.
func (g *gen) globalPlace(v *types.Var) (node, bool) {
	if a, ok := g.galias[v]; ok {
		return text(a), true
	}
	if g.constStatic(v) {
		return text(g.globalName(v)), true
	}
	if g.globals[v] == subset.GlobalStatic && !g.localStatic(v) {
		return atom{"*" + g.globalName(v), precUnary}, true
	}
	return nil, false
}

// constZero renders the zero value of t, a copied kind, as a constant
// expression, for a thread-local's const initializer.
func (g *gen) constZero(t types.Type) string {
	switch u := t.Underlying().(type) {
	case *types.Basic:
		switch {
		case u.Info()&types.IsBoolean != 0:
			return "false"
		case u.Info()&types.IsFloat != 0:
			return "0.0"
		}
		return "0"
	}
	st := t.Underlying().(*types.Struct)
	var fs []string
	for i := 0; i < st.NumFields(); i++ {
		fs = append(fs, g.fields[st.Field(i)]+": "+g.constZero(st.Field(i).Type()))
	}
	if len(fs) == 0 {
		return g.rustType(t) + " {}"
	}
	return g.rustType(t) + " { " + strings.Join(fs, ", ") + " }"
}

// globalDecl writes the package-level variables of one spec.
func (g *gen) globalDecl(s *ast.ValueSpec) {
	for i, n := range s.Names {
		if n.Name == "_" {
			continue // its initializer runs in initVars
		}
		v := g.info.Defs[n].(*types.Var)
		name, vis, rt := g.gnames[v], g.vis(v.Exported()), g.rustType(v.Type())
		sentinel := g.api != nil && g.api.sentinel[v] != ""
		if sentinel {
			vis = "" // its pub function is the API (RFC 0014)
		}
		switch {
		case g.localStatic(v):
			g.fnContext()
			init := g.zero(v.Type())
			if i < len(s.Values) {
				init = g.owned(s.Values[i])
			}
			g.line("thread_local! {")
			g.ind++
			g.stmtLine(vis+"static "+name+": "+rt+" = ", init, ";")
			g.ind--
			g.line("}")
			if sentinel {
				g.sentinelFn(v)
			}
		case g.constStatic(v):
			g.fnContext()
			g.stmtLine(vis+"static "+name+": "+rt+" = ", g.value(s.Values[i]), ";")
		case g.globals[v] == subset.GlobalStatic:
			g.fnContext()
			var init node
			if i < len(s.Values) {
				init = &addrOf{"|| ", g.owned(s.Values[i])}
				if f := init.(*addrOf).x.flat(); f == "rt::Map::default()" {
					init = text("rt::Map::default") // an empty map literal (RFC 0019), not || rt::Map::default()
				}
			} else if z := g.zero(v.Type()).flat(); strings.HasSuffix(z, "::new()") || strings.HasSuffix(z, "::default()") {
				init = text(strings.TrimSuffix(z, "()")) // Vec::new, not || Vec::new()
			} else {
				init = &addrOf{"|| ", text(z)}
			}
			lazy := &call{head: "std::sync::LazyLock::new", open: "(", close: ")", args: []node{init}}
			g.stmtLine(vis+"static "+name+": std::sync::LazyLock<"+rt+"> = ", lazy, ";")
		case g.globals[v] == subset.GlobalCell:
			g.line("thread_local! {")
			g.line("    " + vis + "static " + name + ": std::cell::Cell<" + rt + "> = const { std::cell::Cell::new(" + g.constZero(v.Type()) + ") };")
			g.line("}")
		default: // GlobalString
			g.line("thread_local! {")
			g.line("    " + vis + "static " + name + ": std::cell::RefCell<String> = const { std::cell::RefCell::new(String::new()) };")
			g.line("}")
		}
	}
}

// fnContext starts the per-function state for code outside a function: a
// static's initializer, initVars.
func (g *gen) fnContext() {
	g.fn = nil
	g.recv = nil
	g.locals = map[types.Object]string{}
	g.lnames = newNamer()
	g.usedNames = map[string]int{}
}

// initsOf returns package p's initializers that the program translates
// (go/types lists every one, also those of variables nothing reaches), in
// Go's order.
func (g *gen) initsOf(p *types.Package) []*types.Initializer {
	declared := map[*types.Var]bool{}
	for _, f := range g.pkg.Files {
		if g.pkg.PkgOf[f] != p {
			continue
		}
		for _, d := range f.Decls {
			if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.VAR {
				for _, s := range gd.Specs {
					for _, n := range s.(*ast.ValueSpec).Names {
						declared[g.info.Defs[n].(*types.Var)] = true // _ too: go/types defines it
					}
				}
			}
		}
	}
	var out []*types.Initializer
	for _, ini := range g.info.InitOrder {
		if ini.Lhs[0].Pkg() == p && declared[ini.Lhs[0]] && !g.constStatic(ini.Lhs[0]) {
			out = append(out, ini)
		}
	}
	return out
}

// initVars writes package p's initVars: its variables' initializers, in
// Go's initialization order, or nothing when it has none.
func (g *gen) initVars(p *types.Package) {
	inits := g.initsOf(p)
	if len(inits) == 0 {
		return
	}
	g.fnContext()
	g.line(g.vis(false) + "fn " + g.initVarsName[p] + "() {")
	g.ind++
	for _, ini := range inits {
		v := ini.Lhs[0]
		switch {
		case v.Name() == "_":
			g.stmtLine("let _ = ", g.value(ini.Rhs), ";")
		case g.localStatic(v):
			g.line(g.gnames[v] + ".with(|_| {});")
		case g.globals[v] == subset.GlobalStatic:
			g.line("std::sync::LazyLock::force(&" + g.gnames[v] + ");")
		default:
			// As an assignment, so that Go's order of calls and reads holds.
			id := ast.NewIdent(v.Name())
			g.info.Uses[id] = v
			g.stmt(&ast.AssignStmt{Lhs: []ast.Expr{id}, Tok: token.ASSIGN, Rhs: []ast.Expr{ini.Rhs}})
		}
	}
	g.ind--
	g.line("}")
}

// changedRoots returns the changed package-level variables (Cell or
// String) at the root of the targets of an assignment.
func (g *gen) changedRoots(lhs []ast.Expr) []*types.Var {
	var out []*types.Var
	seen := map[*types.Var]bool{}
	for _, l := range lhs {
		for e := l; e != nil; {
			if v := g.globalOf(e); v != nil {
				if k := g.globals[v]; (k == subset.GlobalCell || k == subset.GlobalString) && !seen[v] {
					seen[v] = true
					out = append(out, v)
				}
				break
			}
			switch x := ast.Unparen(e).(type) {
			case *ast.SelectorExpr:
				e = x.X
			case *ast.IndexExpr:
				e = x.X
			default:
				e = nil
			}
		}
	}
	return out
}

// globalStmt writes a statement that reads or changes a changed
// package-level variable, in gc's order: first every call of the program
// in it, in order, into a local; then the reads; a changed variable is read
// into a local, which the statement changes, and set again after it. It
// reports false, writing nothing, for a statement with none.
func (g *gen) globalStmt(s ast.Stmt, lhs []ast.Expr, write func()) bool {
	if !subset.ReadsChangedGlobal(g.pkg, s, g.globals) || g.inGlobalStmt {
		return false
	}
	g.inGlobalStmt = true
	defer func() { g.inGlobalStmt = false }()
	roots := g.changedRoots(lhs)
	if as, ok := s.(*ast.AssignStmt); ok && as.Tok == token.ASSIGN && len(as.Lhs) == 1 && len(roots) == 1 && g.globalOf(as.Lhs[0]) == roots[0] {
		g.hoistCalls(s, false)
		g.stmtLine(g.globalName(roots[0])+".set(", g.owned(as.Rhs[0]), ");")
		return true
	}
	// A changed variable is read before the statement, so every call of
	// it, the outermost ones too, comes first; so does every call of a
	// statement of several values, which Rust would make in turn with the
	// reads beside them.
	g.hoistCalls(s, len(roots) > 0 || severalValues(s))
	for _, v := range roots {
		a := g.lnames.name(v.Name(), snake)
		g.stmtLine("let mut "+a+" = ", g.globalValue(v), ";")
		g.galias[v] = a
	}
	write()
	for _, v := range roots {
		g.line(g.globalName(v) + ".set(" + g.galias[v] + ");")
		delete(g.galias, v)
	}
	return true
}

// severalValues reports whether s returns or assigns several values, each
// its own expression: return a, f() or a, b = x, f().
func severalValues(s ast.Stmt) bool {
	switch s := s.(type) {
	case *ast.ReturnStmt:
		return len(s.Results) > 1
	case *ast.AssignStmt:
		return len(s.Rhs) > 1
	}
	return false
}

// hoistCalls computes the calls of the program in statement s into locals
// first, in the order Go makes them: all of them, or, unless all is set,
// those that are not the statement's outermost ones.
func (g *gen) hoistCalls(s ast.Stmt, all bool) {
	roots := map[ast.Node]bool{}
	switch s := s.(type) {
	case *ast.ExprStmt:
		roots[ast.Unparen(s.X)] = true
	case *ast.AssignStmt:
		for _, r := range s.Rhs {
			roots[ast.Unparen(r)] = true
		}
	case *ast.ReturnStmt:
		for _, r := range s.Results {
			roots[ast.Unparen(r)] = true
		}
	}
	mut := g.mutTemps(s)
	for _, c := range subset.HoistableCalls(g.pkg, s) {
		if roots[c] && !all {
			continue
		}
		name := g.lnames.name("", func(string) string { return "call" })
		decl := "let "
		if mut[c] {
			decl = "let mut "
		}
		g.boxSlot[c] = true // a call's local holds what it returns: a Box stays one
		g.hoisting = s
		g.stmtLine(decl+name+" = ", g.value(c), ";")
		g.hoisting = nil
		delete(g.boxSlot, c) // read from its local, it is wrapped where an Option is wanted
		g.callTemps[c] = name
	}
}

// programFunc returns the function of the program c calls, or nil (a
// built-in, a conversion, a bridged function).
func (g *gen) programFunc(c *ast.CallExpr) *borrow.Func {
	var id *ast.Ident
	switch f := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	default:
		return nil
	}
	fn, _ := g.info.Uses[id].(*types.Func)
	return g.res.Funcs[fn]
}

// mutTemps returns the calls in s whose results are borrowed exclusively:
// arguments for a RefMut parameter.
func (g *gen) mutTemps(s ast.Stmt) map[*ast.CallExpr]bool {
	out := map[*ast.CallExpr]bool{}
	ast.Inspect(s, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn := g.programFunc(c)
		if fn == nil {
			return true
		}
		for i, a := range c.Args {
			if inner, ok := ast.Unparen(a).(*ast.CallExpr); ok && i < len(fn.Params) && fn.Params[i].Mode == borrow.RefMut {
				out[inner] = true
			}
		}
		// A call's result is never the receiver of a method taking &mut
		// self: Go does not take the address of a call's result.
		return true
	})
	return out
}
