package emit

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/pigfox/gopatina/internal/borrow"
	"github.com/pigfox/gopatina/internal/subset"
)

// freeFuncNames returns the Rust names of the functions and free methods,
// which a function's locals must not shadow. They are found once: a batch
// of many programs has thousands of functions.
func (g *gen) freeFuncNames() map[string]string {
	if g.funcNames == nil {
		g.funcNames = map[string]string{}
		for f, n := range g.funcs {
			if f.Signature().Recv() == nil || g.freeMethods(g.recvTypeName(f)) {
				g.funcNames[n] = f.Name()
			}
		}
	}
	return g.funcNames
}

// funcDecl writes a function or method.
func (g *gen) funcDecl(fd *ast.FuncDecl) {
	obj := g.info.Defs[fd.Name].(*types.Func)
	g.fn = g.res.Funcs[obj]
	g.locals = map[types.Object]string{}
	g.lnames = newNamer()
	g.lnames.outer = g.freeFuncNames()
	g.recv = nil
	g.temps = map[ast.Expr]string{}
	g.usedNames = map[string]int{}
	g.countNames(fd)
	g.docComment(fd.Doc)
	sig := obj.Signature()
	var params []string
	ps := g.fn.Params
	if g.fn.Recv != nil {
		if g.freeMethods(g.recvTypeName(obj)) || g.heapRecv(obj) {
			// The receiver is the function's first parameter (RFC 0007),
			// or a heap pointer, which the method takes itself (RFC 0013).
			ps = append([]*borrow.Param{g.fn.Recv}, ps...)
		} else {
			g.recv = g.fn.Recv.Var
			params = append(params, g.selfParam(g.fn.Recv))
		}
	}
	for _, p := range ps {
		name := "_"
		if p.Var.Name() != "_" && p.Var.Name() != "" {
			name = g.varName(p.Var)
		}
		if p.Mut && (p.Mode == borrow.ByValue || p.Mode == borrow.Owned) {
			name = "mut " + name
		}
		if g.ownPtr(p.Var.Type()) && g.paramRep(p.Var) == repOptRefMut {
			name = "\x00" + name // mut if the body reborrows it (below)
		}
		t := g.paramType(p.Var.Type(), p.Mode)
		if g.ownPtr(p.Var.Type()) {
			// A borrowed owned pointer: &T, &mut T, or in an Option where
			// it may be nil (RFC 0017, steps 2 and 3).
			elem := g.rustType(types.Unalias(p.Var.Type()).(*types.Pointer).Elem())
			t = map[ptrRep]string{repRef: "&", repOptRef: "Option<&", repRefMut: "&mut ", repOptRefMut: "Option<&mut ", repOptBox: "Option<Box<"}[g.paramRep(p.Var)] + elem
			if g.paramRep(p.Var) == repOptBox {
				t = "Option<" + g.boxOf(p.Var.Type()) // kept: Option<Box<T>> (RFC 0017, step 8)
			}
			if g.paramRep(p.Var).optional() {
				t += ">"
			}
		}
		params = append(params, name+": "+t)
	}
	g.life = ""
	generics := ""
	if g.ptrs != nil && g.namedLifetime(obj, params) {
		// The result borrows from one input: its lifetime named, the
		// others elided (RFC 0017, decision 4).
		g.life, generics = "'a ", "<'a>"
		in := g.ptrs.RetBorrow(obj)
		off := len(params) - len(ps) // a self parameter comes first
		for i, p := range ps {
			if p.Var == in {
				name, typ, _ := strings.Cut(params[off+i], ": ")
				params[off+i] = name + ": " + strings.Replace(typ, "&", "&'a ", 1)
			}
		}
	}
	vis := g.vis(obj.Exported() && obj.Name() != "main")
	if g.apiName(obj) != "" {
		vis = "" // its pub wrapper is the API (RFC 0014)
	}
	ret := g.resultType(obj)
	if ret != "" {
		ret = " -> " + ret
	}
	head := vis + "fn " + g.funcs[obj] + generics + "("
	// The body first, into its own buffer: an Option<&mut T> parameter is
	// mut only if the body reborrows it (as_mut, as_deref_mut).
	saved := g.out
	var body strings.Builder
	g.out = &body
	g.ind++
	g.funcBody(fd, obj, sig, ps)
	g.ind--
	g.out = saved
	for i, p := range params {
		if name, ok := strings.CutPrefix(p, "\x00"); ok {
			v := strings.SplitN(name, ":", 2)[0]
			if strings.Contains(body.String(), v+".as_mut()") || strings.Contains(body.String(), v+".as_deref_mut()") {
				name = "mut " + name
			}
			params[i] = name
		}
	}
	open := " {"
	if body.Len() == 0 {
		open = " {}" // an empty body, as rustfmt writes it
	}
	sigLine := head + strings.Join(params, ", ") + ")" + ret + open
	if g.ind*indentWidth+len(sigLine) <= maxWidth {
		g.line(sigLine)
	} else {
		g.line(head)
		g.ind++
		for _, p := range params {
			g.line(p + ",")
		}
		g.ind--
		g.line(")" + ret + open)
	}
	if body.Len() > 0 {
		g.out.WriteString(reborrowedMut(body.String()))
		g.line("}")
	}
	if g.inAPI(obj) {
		g.apiTail(fd, obj, params, ret)
	}
}

// funcBody writes the body of function fd, already indented.
func (g *gen) funcBody(fd *ast.FuncDecl, obj *types.Func, sig *types.Signature, ps []*borrow.Param) {
	main := obj.Name() == "main" && g.fn.Recv == nil
	if main && !g.defers() {
		g.rt["exit_on_panic"] = true
		g.line("rt::exit_on_panic();")
	}
	if main {
		// Each package: its variables, then its init functions (Go's
		// order, RFC 0009 and RFC 0010).
		for _, p := range g.pkg.Pkgs {
			if len(g.initsOf(p)) > 0 {
				g.line(g.qual(p, g.initVarsName[p]) + "();")
			}
			for _, f := range g.inits {
				if f.Pkg() == p {
					g.line(g.qual(f.Pkg(), g.funcs[f]) + "();")
				}
			}
		}
	}
	res := sig.Results()
	for i := 0; i < res.Len(); i++ {
		r := res.At(i)
		switch {
		case r.Name() == "" || r.Name() == "_":
		case isPointerType(r.Type()) && g.ptrs.RetBorrow(obj) != nil:
			// The named result of a function lending back a borrow
			// (decision 4): held as the result is, nil until assigned.
			g.line("let " + g.mutPrefix(r) + g.varName(r) + ": " + g.borrowType(r.Type(), g.retRep(obj), g.life) + " = None;")
		default:
			g.letZero(r)
		}
	}
	g.fnStart = fd.Body.Lbrace
	g.fnBody = fd.Body
	g.deferred = ""
	if main && g.defers() {
		// A panic unwinds, running the deferred calls, before Go's
		// message and exit status (rt::run_main).
		g.rt["run_main"] = true
		g.line("rt::run_main(|| {")
		g.ind++
		defer func() { g.ind--; g.line("});") }()
	}
	if hasDefer(fd.Body) {
		g.rt["Defers"] = true
		g.deferred = g.lnames.name("", func(string) string { return "deferred" })
		g.line("let mut " + g.deferred + " = rt::Defers::default();")
	}
	g.bodyTo(fd.Body.List, true, fd.Body.Rbrace)
}

// hasDefer reports whether body defers a call.
func hasDefer(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		_, ok := n.(*ast.DeferStmt)
		found = found || ok
		return !found
	})
	return found
}

// defers reports whether a function of the program defers a call: then a
// panic must unwind, so the deferred calls run.
func (g *gen) defers() bool {
	for _, f := range g.pkg.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil && hasDefer(fd.Body) {
				return true
			}
		}
	}
	return false
}

// deferStmt writes defer fmt.Print(...), Println or Printf (subset
// checks it): Go evaluates the arguments now, values whose text cannot
// change by the time the call runs, so the text is made now and printed
// then (rt::Defers).
func (g *gen) deferStmt(s *ast.DeferStmt) {
	sel := s.Call.Fun.(*ast.SelectorExpr)
	text := g.fmtCall(s.Call, "fmt.Sp"+strings.TrimPrefix(sel.Sel.Name, "P")) // Println: Sprintln
	g.stmtLine(g.deferred+".print(", text, ");")
}

// apiTail writes, for a function of a library's API (RFC 0014), its pub
// wrapper and its notes.
func (g *gen) apiTail(fd *ast.FuncDecl, obj *types.Func, params []string, ret string) {
	name := g.funcs[obj]
	if n := g.apiName(obj); n != "" {
		name = n
		g.apiWrapper(fd, obj, params, ret)
	}
	if g.fn.Recv != nil && !g.freeMethods(g.recvTypeName(obj)) {
		name = g.types[g.recvTypeName(obj)] + "::" + name
	}
	g.apiNotes(obj, name)
}

// countNames counts the local objects that get each Rust name, so a loop
// whose variable shares its name with another is put in its own block.
func (g *gen) countNames(fd *ast.FuncDecl) {
	seen := map[types.Object]bool{}
	ast.Inspect(fd, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if v, ok := g.info.Defs[id].(*types.Var); ok && !seen[v] {
				seen[v] = true
				g.usedNames[snake(v.Name())]++
			}
		}
		return true
	})
}

func (g *gen) selfParam(p *borrow.Param) string {
	switch p.Mode {
	case borrow.Ref:
		return "&self"
	case borrow.RefMut:
		return "&mut self"
	}
	if p.Mut {
		return "mut self"
	}
	return "self"
}

// resultType returns the Rust return type of fn, or "" for none.
func (g *gen) resultType(fn *types.Func) string {
	res := fn.Signature().Results()
	var ts []string
	n := res.Len()
	lowered := g.errs.Funcs[fn]
	if lowered {
		n--
	}
	for i := 0; i < n; i++ {
		t := g.rustType(res.At(i).Type())
		if g.ownPtr(res.At(i).Type()) && g.ptrs.NonNilResult(fn) {
			t = g.boxOf(res.At(i).Type()) // RFC 0017, step 2
		}
		if g.ptrs != nil && g.ptrs.RetBorrow(fn) != nil {
			t = g.borrowType(res.At(i).Type(), g.retRep(fn), g.life) // a borrow lent back (decision 4)
		}
		ts = append(ts, t)
	}
	t := strings.Join(ts, ", ")
	if len(ts) != 1 {
		t = "(" + t + ")"
	}
	if lowered {
		return "Result<" + t + ", " + g.root("Iface") + ">"
	}
	if len(ts) == 0 {
		return ""
	}
	return t
}

// letZero declares v with its zero value.
func (g *gen) letZero(v *types.Var) {
	t := g.rustType(v.Type())
	if g.isBorrow(v) {
		t = "Option<&" + g.rustType(types.Unalias(v.Type()).(*types.Pointer).Elem()) + ">" // a temporary borrow, nil for now
	}
	if g.ptrs.RefAs(v) != nil && g.ptrs.LocalRef(v) {
		// errors.As's target, a borrow of the pointer in err's chain.
		ref := map[bool]string{true: "&mut ", false: "&"}[g.localRefRep(v) == repOptRefMut]
		g.line("let mut " + g.varName(v) + ": Option<" + ref + g.rustType(types.Unalias(v.Type()).(*types.Pointer).Elem()) + "> = None;")
		return
	}
	g.line("let " + g.mutPrefix(v) + g.varName(v) + ": " + t + " = " + g.zero(v.Type()).flat() + ";")
}

// isBorrow reports whether v is a temporary borrow of an owned pointer
// (RFC 0017, step 5): an Option<&T>.
func (g *gen) isBorrow(v *types.Var) bool {
	return v != nil && g.ownPtr(v.Type()) && g.ptrs.BorrowVar(v)
}

func (g *gen) mutPrefix(v *types.Var) string {
	if g.res.Mut[v] {
		return "mut "
	}
	return ""
}

// body writes the statements of a block. At the end of a function body a
// final return becomes the block's tail expression.
func (g *gen) body(list []ast.Stmt, fnBody bool) {
	g.bodyTo(list, fnBody, token.NoPos)
}

// bodyTo writes the statements of a block whose closing brace is at end
// (or NoPos), with the Go comments and blank lines between them.
func (g *gen) bodyTo(list []ast.Stmt, fnBody bool, end token.Pos) {
	saved := g.lastLine
	g.lastLine = 0
	for i := 0; i < len(list); i++ {
		s := list[i]
		g.commentsBefore(s.Pos())
		g.blankBefore(g.pkg.Fset.Position(s.Pos()).Line)
		if fnBody && i == len(list)-2 {
			if ret, ok := list[i+1].(*ast.ReturnStmt); ok {
				if !g.hasLineCommentsBetween(s.End(), ret.Pos()) {
					if e, folded, ok := g.foldTailLetReturn(s, ret); ok {
						var b strings.Builder
						out := g.out
						g.out = &b
						g.stmtLine("", e, "")
						g.out = out
						endLine := g.pkg.Fset.Position(folded.End()).Line
						text := b.String()
						if c := g.trailingComment(folded.End(), endLine); c != "" {
							text = strings.TrimSuffix(text, "\n") + " " + c + "\n"
						}
						g.out.WriteString(text)
						g.lastLine = endLine
						i++ // the tail let+return was folded
						continue
					}
				}
			}
		}
		last := s
		var b strings.Builder
		out := g.out
		g.out = &b
		switch n := g.zeroThenFields(list, i); {
		case n > 0:
			// var p T; p.f = x: the fields given at once, as clippy asks
			// (field_reassign_with_default).
			g.letWithFields(list[i:i+n+1], list[i+n+1:])
			i += n
			last = list[i]
		case i+1 < len(list) && g.fieldStoreNext(list[i], list[i+1]):
			// var p T; p.f = p.g: every field zero, spelled out, since a
			// store after T::default() is clippy's field_reassign_with_default.
			g.letZeroFields(g.zeroStruct(s))
		case g.errs.Calls[s] != nil:
			ec := g.errs.Calls[s]
			g.errCall(s, ec)
			if ec.Check != nil && !ec.Init {
				i++ // the check is written with the call
				last = list[i]
			}
		case isReturn(s) && fnBody && i == len(list)-1:
			if subset.ReadsChangedGlobal(g.pkg, s, g.globals) {
				g.hoistCalls(s, severalValues(s)) // gc's order (RFC 0010)
			} else {
				g.heapOrder(s) // and through heap pointers (RFC 0013)
			}
			if e := g.returnValue(s.(*ast.ReturnStmt)); e != nil {
				g.stmtLine("", e, "")
			}
		default:
			g.stmt(s)
		}
		g.out = out
		endLine := g.pkg.Fset.Position(last.End()).Line
		text := b.String()
		if c := g.trailingComment(last.End(), endLine); c != "" {
			text = strings.TrimSuffix(text, "\n") + " " + c + "\n"
		}
		g.out.WriteString(text)
		g.lastLine = endLine
	}
	if end.IsValid() {
		g.commentsBefore(end)
	}
	g.lastLine = saved
}

// zeroThenFields returns how many statements after list[i], a var
// declaration of one struct variable with no value, assign its fields
// directly, each a different field, from values that do not read the
// variable, with no comment between: Rust gives them in the initializer.
func (g *gen) zeroThenFields(list []ast.Stmt, i int) int {
	v := g.zeroStruct(list[i])
	if v == nil {
		return 0
	}
	seen := map[string]bool{}
	n := 0
	for j := i + 1; j < len(list); j++ {
		as, ok := list[j].(*ast.AssignStmt)
		if !ok || as.Tok != token.ASSIGN || len(as.Lhs) != 1 || len(as.Rhs) != 1 || g.hasLineCommentsBetween(list[j-1].End(), as.Pos()) {
			break
		}
		sel, ok := ast.Unparen(as.Lhs[0]).(*ast.SelectorExpr)
		if !ok {
			break
		}
		id, ok := ast.Unparen(sel.X).(*ast.Ident)
		if !ok || g.localVar(id) != v || seen[sel.Sel.Name] || g.mentions(as.Rhs[0], v) {
			break
		}
		seen[sel.Sel.Name] = true
		n++
	}
	return n
}

// zeroStruct returns the variable s declares, when s is var v T with no
// value, T a struct of the program held as a plain value, or nil.
func (g *gen) zeroStruct(s ast.Stmt) *types.Var {
	d, ok := s.(*ast.DeclStmt)
	if !ok {
		return nil
	}
	gd := d.Decl.(*ast.GenDecl)
	if gd.Tok != token.VAR || len(gd.Specs) != 1 {
		return nil
	}
	vs := gd.Specs[0].(*ast.ValueSpec)
	if len(vs.Names) != 1 || len(vs.Values) != 0 {
		return nil
	}
	v := g.localVar(vs.Names[0]) // a local: the statement is in a function
	if _, ok := v.Type().Underlying().(*types.Struct); !ok {
		return nil
	}
	return v
}

// fieldStoreNext reports whether s is var v T for a struct (zeroStruct)
// and next assigns a field of v.
func (g *gen) fieldStoreNext(s, next ast.Stmt) bool {
	v := g.zeroStruct(s)
	as, ok := next.(*ast.AssignStmt)
	if v == nil || !ok {
		return false
	}
	for _, l := range as.Lhs {
		if sel, ok := ast.Unparen(l).(*ast.SelectorExpr); ok {
			if id, ok := ast.Unparen(sel.X).(*ast.Ident); ok && g.localVar(id) == v {
				return true
			}
		}
	}
	return false
}

// letZeroFields declares v, a struct, with every field's zero value.
func (g *gen) letZeroFields(v *types.Var) {
	st := v.Type().Underlying().(*types.Struct)
	lit := &structLit{name: g.rustType(v.Type())}
	for i := 0; i < st.NumFields(); i++ {
		lit.fields = append(lit.fields, fieldInit{name: g.fields[st.Field(i)], value: g.zero(st.Field(i).Type())})
	}
	g.stmtLine("let "+g.mutPrefix(v)+g.varName(v)+" = ", lit, ";")
}

// letWithFields writes stmts, var v T and the field assignments after it,
// as one let with a struct literal; rest are the statements after, which
// decide whether v is still changed (mut).
func (g *gen) letWithFields(stmts, rest []ast.Stmt) {
	v := g.zeroStruct(stmts[0])
	st := v.Type().Underlying().(*types.Struct)
	lit := &structLit{name: g.rustType(v.Type())}
	set := map[*types.Var]bool{}
	for _, s := range stmts[1:] {
		as := s.(*ast.AssignStmt)
		sel := ast.Unparen(as.Lhs[0]).(*ast.SelectorExpr)
		f := g.info.Selections[sel].Obj().(*types.Var)
		set[f] = true
		lit.fields = append(lit.fields, fieldInit{name: g.fields[f], value: g.owned(as.Rhs[0])})
	}
	if len(lit.fields) < st.NumFields() {
		if n, ok := types.Unalias(v.Type()).(*types.Named); ok && len(g.dropFields(n.Obj())) > 0 {
			// A type with a Drop has no ..Default::default() (E0509):
			// every field, the others zero.
			for i := 0; i < st.NumFields(); i++ {
				if f := st.Field(i); !set[f] {
					lit.fields = append(lit.fields, fieldInit{name: g.fields[f], value: g.zero(f.Type())})
				}
			}
		} else {
			lit.base = "Default::default()"
		}
	}
	mut := ""
	for _, s := range rest {
		if g.changes(s, v) {
			mut = "mut "
			break
		}
	}
	g.stmtLine("let "+mut+g.varName(v)+" = ", lit, ";")
}

// changes reports whether s may change v: assigns or increments a place
// rooted at v, takes its address, or calls a method that changes it.
func (g *gen) changes(s ast.Node, v *types.Var) bool {
	rooted := func(e ast.Expr) bool {
		for {
			switch x := ast.Unparen(e).(type) {
			case *ast.Ident:
				return g.localVar(x) == v
			case *ast.SelectorExpr:
				e = x.X
			case *ast.IndexExpr:
				e = x.X
			default:
				return false
			}
		}
	}
	found := false
	ast.Inspect(s, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, l := range x.Lhs {
				found = found || rooted(l)
			}
		case *ast.IncDecStmt:
			found = found || rooted(x.X)
		case *ast.UnaryExpr:
			// &v moving v into a new box (subset.PtrPlan.EscapeBox) does
			// not change it.
			found = found || x.Op == token.AND && rooted(x.X) && g.ptrs.EscapeBox(x) == nil
		case *ast.CallExpr:
			// A method that changes its receiver: an exclusive borrow.
			if sel, ok := ast.Unparen(x.Fun).(*ast.SelectorExpr); ok && rooted(sel.X) {
				if fn := g.programFunc(x); fn != nil && fn.Recv != nil && fn.Recv.Mode == borrow.RefMut {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

// reborrowedMut resolves the marks let \x01name of a function's body: an
// Option<&mut T> local is mut where the body reborrows it (name.as_mut(),
// name.as_deref_mut()), and not where it only moves it.
func reborrowedMut(body string) string {
	for {
		i := strings.Index(body, "\x01")
		if i < 0 {
			return body
		}
		j := i + 1
		for j < len(body) && (body[j] == '_' || body[j] >= 'a' && body[j] <= 'z' || body[j] >= '0' && body[j] <= '9') {
			j++
		}
		name := body[i+1 : j]
		mut := ""
		if strings.Contains(body, name+".as_mut()") || strings.Contains(body, name+".as_deref_mut()") {
			mut = "mut "
		}
		body = body[:i] + mut + body[i+1:]
	}
}

// copyFirst reads into locals, before simple statement s, the places s
// hands to a print through an exclusive local borrow whose source it
// reads after (borrow.Result.CopyFirst): let n = p.n; println!("{} {}",
// n, x.n), the borrow ended before x is read.
func (g *gen) copyFirst(s ast.Stmt) {
	ast.Inspect(s, func(n ast.Node) bool {
		e, ok := n.(ast.Expr)
		if !ok || !g.res.CopyFirst[e] {
			return true
		}
		if _, done := g.temps[e]; done {
			return false
		}
		base := "copied"
		if sel, ok := ast.Unparen(e).(*ast.SelectorExpr); ok {
			base = snake(sel.Sel.Name)
		}
		name := g.lnames.name("", func(string) string { return base })
		g.stmtLine("let "+name+" = ", g.owned(e), ";")
		g.temps[e] = name
		return false
	})
}

// keepFactors reads into locals, before statement s, the factors of a
// multiply s gives a local that a later + fuses (load.Package.Kept): let
// x = ...; let v = x * y; and later v + z fuses x * y + z, with x and y as
// they were, as gc's SSA has them.
func (g *gen) keepFactors(s ast.Stmt) {
	ast.Inspect(s, func(n ast.Node) bool {
		m, ok := n.(*ast.BinaryExpr)
		if !ok || !g.pkg.Kept[m] {
			return true
		}
		for _, x := range []ast.Expr{m.X, m.Y} {
			if g.info.Types[x].Value != nil {
				continue // a constant is the same later
			}
			name := g.lnames.name("", func(string) string { return "factor" })
			g.stmtLine("let "+name+" = ", g.value(x), ";")
			g.temps[ast.Unparen(x)] = name
		}
		return false
	})
}

func isReturn(s ast.Stmt) bool {
	_, ok := s.(*ast.ReturnStmt)
	return ok
}

// commentsBefore writes the comments of the current function that come
// before pos and have not been written yet.
func (g *gen) commentsBefore(pos token.Pos) {
	if g.quiet {
		return
	}
	for _, cg := range g.comments {
		if g.usedComments[cg] || cg.Pos() >= pos || cg.Pos() < g.fnStart {
			continue
		}
		g.usedComments[cg] = true
		g.blankBefore(g.pkg.Fset.Position(cg.Pos()).Line)
		for _, c := range cg.List {
			for _, l := range strings.Split(c.Text, "\n") {
				g.line(strings.TrimLeft(l, "\t"))
			}
		}
		g.lastLine = g.pkg.Fset.Position(cg.End()).Line
	}
}

// trailingComment returns a comment on the same line after a statement
// ending at end on line, and marks it written.
func (g *gen) trailingComment(end token.Pos, line int) string {
	if g.quiet {
		return ""
	}
	for _, cg := range g.comments {
		if g.usedComments[cg] || cg.Pos() < end || g.pkg.Fset.Position(cg.Pos()).Line != line || len(cg.List) != 1 {
			continue
		}
		g.usedComments[cg] = true
		return cg.List[0].Text
	}
	return ""
}

// blankBefore keeps a blank line the Go source has before line.
func (g *gen) blankBefore(line int) {
	if g.lastLine > 0 && line > g.lastLine+1 {
		g.line("")
	}
}

func (g *gen) block(b *ast.BlockStmt) {
	g.ind++
	g.bodyTo(b.List, false, b.Rbrace)
	g.ind--
}

func (g *gen) stmt(s ast.Stmt) {
	if g.skip[s] {
		return // a field set to nil just after its take (RFC 0017, step 6)
	}
	if ec := g.errs.Calls[s]; ec != nil && ec.General {
		g.generalCall(ec) // an if statement's init (RFC 0012)
		return
	}
	switch s.(type) {
	case *ast.ExprStmt, *ast.ReturnStmt, *ast.DeclStmt, *ast.AssignStmt:
		g.copyFirst(s)
	}
	switch s.(type) {
	case *ast.DeclStmt, *ast.AssignStmt:
		g.keepFactors(s)
	}
	// A simple statement that reads or changes a changed package-level
	// variable is written in gc's order (RFC 0010).
	switch x := s.(type) {
	case *ast.ExprStmt, *ast.ReturnStmt, *ast.DeclStmt:
		if g.globalStmt(s, nil, func() { g.stmt(s) }) {
			return
		}
	case *ast.AssignStmt:
		if g.globalStmt(s, x.Lhs, func() { g.stmt(s) }) {
			return
		}
	case *ast.IncDecStmt:
		if g.globalStmt(s, []ast.Expr{x.X}, func() { g.stmt(s) }) {
			return
		}
	}
	g.heapOrder(s)
	g.hoist(s)
	switch s := s.(type) {
	case *ast.ExprStmt:
		g.exprStmt(s.X)
	case *ast.DeferStmt:
		g.deferStmt(s)
	case *ast.AssignStmt:
		g.assign(s)
	case *ast.IncDecStmt:
		if id, ok := s.X.(*ast.Ident); ok && g.fastVar(g.localVar(id)) {
			g.line(g.varName(g.localVar(id)) + " += 1;")
			return
		}
		g.hoistIndexes([]ast.Expr{s.X}, false)
		one := "1"
		if isFloat(g.typeOf(s.X)) {
			one = "1.0"
		}
		op := " += " + one + ";"
		if s.Tok == token.DEC {
			op = " -= " + one + ";"
		}
		g.line(g.lhsPlace(s.X).flat() + op)
	case *ast.DeclStmt:
		g.declStmt(s.Decl.(*ast.GenDecl))
	case *ast.BlockStmt:
		g.line("{")
		g.block(s)
		g.line("}")
	case *ast.IfStmt:
		g.ifStmt(s)
	case *ast.ForStmt:
		g.forStmt(s)
	case *ast.RangeStmt:
		g.rangeStmt(s)
	case *ast.SwitchStmt:
		g.switchStmt(s)
	case *ast.TypeSwitchStmt:
		g.typeSwitch(s) // RFC 0012
	case *ast.ReturnStmt:
		if e := g.returnValue(s); e != nil {
			g.stmtLine("return ", e, ";")
		} else {
			g.line("return;")
		}
	case *ast.BranchStmt:
		g.branch(s)
	}
}

// exprStmt writes a call used as a statement.
func (g *gen) exprStmt(e ast.Expr) {
	n := g.value(e)
	if c, ok := n.(*call); ok && (c.head == "write!" || c.head == "writeln!") {
		g.stmtLine("let _ = ", n, ";")
		return
	}
	g.stmtLine("", n, ";")
}

// returnValue renders the value of a return statement, or nil for none.
func (g *gen) returnValue(r *ast.ReturnStmt) node {
	sig := g.fn.Obj.Signature()
	results := r.Results
	if len(results) == 1 && g.ptrs != nil && g.ptrs.RetBorrow(g.fn.Obj) != nil {
		return g.ptrAs(results[0], g.retRep(g.fn.Obj)) // a borrow lent back (decision 4)
	}
	if len(results) == 1 && g.ownPtr(g.typeOf(results[0])) && g.ptrs.NonNilResult(g.fn.Obj) {
		g.boxSlot[ast.Unparen(results[0])] = true // returned as the Box it is
	}
	if len(results) == 0 && sig.Results().Len() > 0 {
		// A bare return of named results.
		var ns []node
		for i := 0; i < sig.Results().Len(); i++ {
			ns = append(ns, text(g.varName(sig.Results().At(i))))
		}
		return tuple(ns)
	}
	switch g.errs.Returns[r] {
	case subset.ReturnCall:
		return g.rawCall(results[0].(*ast.CallExpr))
	case subset.ReturnValue:
		return g.errResult(r) // RFC 0012
	case subset.ReturnOk:
		vals := g.ownedAll(results[:len(results)-1])
		if len(vals) == 0 {
			vals = []node{text("()")}
		}
		return &call{head: "Ok", open: "(", close: ")", args: []node{tuple(vals)}}
	case subset.ReturnErr:
		last := results[len(results)-1]
		var errNode node
		if id, ok := ast.Unparen(last).(*ast.Ident); ok && g.localVar(id) != nil && !g.boxedExpr(last) {
			errNode = text(g.varName(g.localVar(id)))
		} else {
			errNode = g.owned(last) // errors.New, or a value converted to error
		}
		return &call{head: "Err", open: "(", close: ")", args: []node{errNode}}
	}
	if len(results) == 0 {
		return nil
	}
	return tuple(g.ownedAll(results))
}

func (g *gen) ownedAll(es []ast.Expr) []node {
	var ns []node
	for _, e := range es {
		ns = append(ns, g.owned(e))
	}
	return ns
}

// tuple renders one value as itself and several as a tuple.
func tuple(ns []node) node {
	if len(ns) == 1 {
		return ns[0]
	}
	return &call{open: "(", close: ")", args: ns}
}

func (g *gen) declStmt(d *ast.GenDecl) {
	if d.Tok == token.CONST {
		for _, spec := range d.Specs {
			g.constDecl(d, spec.(*ast.ValueSpec), false)
		}
		return
	}
	for _, spec := range d.Specs {
		vs := spec.(*ast.ValueSpec)
		if len(vs.Values) == 0 {
			for _, name := range vs.Names {
				if v := g.localVar(name); v != nil {
					g.letZero(v)
				}
			}
			continue
		}
		if len(vs.Names) == 1 {
			g.let(g.localVar(vs.Names[0]), vs.Values[0], vs.Type != nil)
			continue
		}
		g.multiLet(identsOf(vs.Names), vs.Values)
	}
}

func identsOf(ids []*ast.Ident) []ast.Expr {
	es := make([]ast.Expr, len(ids))
	for i, id := range ids {
		es[i] = id
	}
	return es
}

// let declares v initialised to e.
func (g *gen) let(v *types.Var, e ast.Expr, typed bool) {
	if isPointerType(v.Type()) && !g.heapPtr(v.Type()) {
		g.letPointer(v, e)
		return
	}
	if g.ptrs != nil && g.ptrs.LocalRef(v) {
		// p := &x, a local borrow (RFC 0017): let p = &mut x, or &x; or p
		// := f(x), held as f returns it.
		mut := ""
		if g.localRefRep(v) == repOptRefMut {
			mut = "\x01" // mut if the body reborrows it (reborrowedMut)
		}
		g.stmtLine("let "+mut+g.varName(v)+" = ", g.ptrAs(e, g.localRefRep(v)), ";")
		return
	}
	prefix := "let " + g.mutPrefix(v) + g.varName(v)
	if g.isBorrow(v) {
		// A temporary borrow (RFC 0017, step 5): an Option<&T>.
		if g.info.Types[e].IsNil() {
			prefix += ": Option<&" + g.rustType(types.Unalias(v.Type()).(*types.Pointer).Elem()) + ">"
		}
		g.stmtLine(prefix+" = ", g.ptrAs(e, repOptRef), ";")
		return
	}
	box := g.ownPtr(v.Type()) && g.ptrs.NonNilVar(v)
	if box {
		g.boxSlot[ast.Unparen(e)] = true // a Box<T>, never nil (RFC 0017, step 2)
	}
	if typed || g.needsType(e) {
		t := g.rustType(v.Type())
		if box {
			t = g.boxOf(v.Type())
		}
		prefix += ": " + t
	}
	g.stmtLine(prefix+" = ", g.owned(e), ";")
}

// letPointer declares a pointer variable: a borrow of its target, or, for
// &T{...}, the value itself.
func (g *gen) letPointer(v *types.Var, e ast.Expr) {
	u := ast.Unparen(e).(*ast.UnaryExpr)
	ptr := g.res.Pointers[v]
	if ptr.Target == nil {
		g.stmtLine("let "+g.mutPrefix(v)+g.varName(v)+" = ", g.value(u.X), ";")
		return
	}
	if ptr.Exclusive {
		g.stmtLine("let "+g.varName(v)+" = ", g.mutRef(u.X), ";")
		return
	}
	g.stmtLine("let "+g.varName(v)+" = ", g.ref(u.X), ";")
}

// needsType reports whether a let of e needs its type written: Rust cannot
// infer the Go type of a constant, an empty slice or a slice literal.
func (g *gen) needsType(e ast.Expr) bool {
	tv := g.info.Types[e]
	if tv.Value != nil || tv.IsNil() {
		return true
	}
	switch x := ast.Unparen(e).(type) {
	case *ast.CompositeLit:
		return isSlice(tv.Type)
	case *ast.CallExpr:
		if id, ok := ast.Unparen(x.Fun).(*ast.Ident); ok && id.Name == "make" {
			return true
		}
	}
	return false
}

func (g *gen) assign(s *ast.AssignStmt) {
	if id, ok := s.Lhs[0].(*ast.Ident); ok && len(s.Lhs) == 1 && g.fastVar(g.localVar(id)) {
		// The start or the step of a usize loop variable (loopShape).
		name := g.varName(g.localVar(id))
		if s.Tok == token.DEFINE {
			g.stmtLine("let mut "+name+" = ", g.uTyped(s.Rhs[0]), ";")
		} else {
			g.stmtLine(name+" += ", g.uNode(s.Rhs[0]), ";")
		}
		return
	}
	varShift := (s.Tok == token.SHL_ASSIGN || s.Tok == token.SHR_ASSIGN) && g.info.Types[s.Rhs[0]].Value == nil
	g.hoistIndexes(s.Lhs, len(s.Lhs) > 1 || varShift)
	switch s.Tok {
	case token.DEFINE, token.ASSIGN:
	default:
		g.opAssign(s)
		return
	}
	if len(s.Lhs) == 1 && len(s.Rhs) == 1 {
		l, r := s.Lhs[0], s.Rhs[0]
		if g.isSelfAppend(l, r) {
			g.appendStmt(l, r.(*ast.CallExpr))
			return
		}
		if id, ok := l.(*ast.Ident); ok {
			if id.Name == "_" {
				g.stmtLine("let _ = ", g.value(r), ";")
				return
			}
			v := g.localVar(id)
			if s.Tok == token.DEFINE && g.info.Defs[id] != nil {
				g.let(v, r, false)
				return
			}
		}
		if id, ok := l.(*ast.Ident); ok && g.isBorrow(g.localVar(id)) {
			g.stmtLine(g.varName(g.localVar(id))+" = ", g.ptrAs(r, repOptRef), ";") // a borrow lent anew
			return
		}
		g.stmtLine(g.lhsPlace(l).flat()+" = ", g.owned(r), ";")
		return
	}
	if len(s.Rhs) == 1 {
		// v1, v2 := f()
		g.multiAssign(s.Lhs, s.Tok == token.DEFINE, g.value(s.Rhs[0]))
		return
	}
	if s.Tok == token.DEFINE {
		g.multiLet(s.Lhs, s.Rhs)
		return
	}
	var ls []string
	for _, l := range s.Lhs {
		ls = append(ls, g.lhs(l))
	}
	g.stmtLine("("+strings.Join(ls, ", ")+") = ", tuple(g.ownedAll(s.Rhs)), ";")
}

// lhs renders an assignment target; _ stays _.
func (g *gen) lhs(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok && id.Name == "_" {
		return "_"
	}
	return g.lhsPlace(e).flat()
}

// lhsPlace renders e, a place being written: through an owned pointer,
// exclusively (RFC 0017, step 3).
func (g *gen) lhsPlace(e ast.Expr) node {
	saved := g.writing
	g.writing = true
	n := g.place(e)
	g.writing = saved
	return n
}

// boundPtr is an owned pointer bound by if let Some: the binding's name
// and how it holds the pointer.
type boundPtr struct {
	name string
	rep  ptrRep
}

// changesThrough reports whether n writes through x, an owned pointer: a
// place under x assigned or incremented, or x lent to a function or method
// that changes it.
func (g *gen) changesThrough(n ast.Node, x ast.Expr) bool {
	key := types.ExprString(x)
	under := func(e ast.Expr) bool {
		for e = ast.Unparen(e); ; {
			var base ast.Expr
			switch x := e.(type) {
			case *ast.SelectorExpr:
				base = x.X
			case *ast.StarExpr:
				base = x.X // *p = v replaces the value in p's box
			default:
				return false
			}
			if types.ExprString(ast.Unparen(base)) == key {
				return true
			}
			e = ast.Unparen(base)
		}
	}
	found := false
	ast.Inspect(n, func(m ast.Node) bool {
		switch y := m.(type) {
		case *ast.AssignStmt:
			for _, l := range y.Lhs {
				found = found || under(l)
			}
		case *ast.IncDecStmt:
			found = found || under(y.X)
		case *ast.CallExpr:
			fn := g.programFunc(y)
			if fn == nil {
				break
			}
			if sel, ok := ast.Unparen(y.Fun).(*ast.SelectorExpr); ok && fn.Recv != nil && fn.Recv.Mode == borrow.RefMut {
				found = found || types.ExprString(ast.Unparen(sel.X)) == key || under(sel.X)
			}
			for i, a := range y.Args {
				if fn.Params[i].Mode == borrow.RefMut {
					found = found || types.ExprString(ast.Unparen(a)) == key || under(a)
				}
			}
		}
		return !found
	})
	return found
}

// multiLet declares several variables at once: Go evaluates every value
// before any is assigned, and so does a tuple.
func (g *gen) multiLet(lhs, rhs []ast.Expr) {
	g.letTypes = nil
	for i, l := range lhs {
		if id, ok := l.(*ast.Ident); ok && i < len(rhs) {
			if v := g.localVar(id); v != nil && g.ownPtr(v.Type()) && g.ptrs.NonNilVar(v) {
				g.boxSlot[ast.Unparen(rhs[i])] = true // a Box<T> (or T), never nil (RFC 0017, step 2)
			}
		}
	}
	for i, r := range rhs {
		if g.needsType(r) {
			for _, l := range lhs {
				g.letTypes = append(g.letTypes, g.rustType(g.typeOf(l)))
			}
			break
		}
		_ = i
	}
	g.multiAssign(lhs, true, tuple(g.ownedAll(rhs)))
	g.letTypes = nil
}

// multiAssign assigns value, a tuple, to the left-hand sides. With define,
// the new variables are declared; existing ones are assigned.
func (g *gen) multiAssign(lhs []ast.Expr, define bool, value node) {
	var pats []string
	allNew := true
	for _, l := range lhs {
		id, ok := l.(*ast.Ident)
		if !ok || !define || (id.Name != "_" && g.info.Defs[id] == nil) {
			allNew = false
		}
	}
	if allNew {
		for _, l := range lhs {
			id := l.(*ast.Ident)
			if id.Name == "_" {
				pats = append(pats, "_")
				continue
			}
			v := g.localVar(id)
			pats = append(pats, g.mutPrefix(v)+g.varName(v))
		}
		typ := ""
		if g.letTypes != nil {
			typ = ": (" + strings.Join(g.letTypes, ", ") + ")"
		}
		g.stmtLine("let ("+strings.Join(pats, ", ")+")"+typ+" = ", value, ";")
		return
	}
	for _, l := range lhs {
		if id, ok := l.(*ast.Ident); ok && define && g.info.Defs[id] != nil {
			// New beside existing ones: declare first, assign below.
			v := g.localVar(id)
			g.line("let " + g.mutPrefix(v) + g.varName(v) + ";")
		}
		pats = append(pats, g.lhs(l))
	}
	g.stmtLine("("+strings.Join(pats, ", ")+") = ", value, ";")
}

// isSelfAppend reports whether r is append(l, ...).
func (g *gen) isSelfAppend(l, r ast.Expr) bool {
	c, ok := ast.Unparen(r).(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := ast.Unparen(c.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	if b, ok := g.info.Uses[id].(*types.Builtin); !ok || b.Name() != "append" {
		return false
	}
	return true
}

// appendStmt writes s = append(s, ...) as pushes.
func (g *gen) appendStmt(l ast.Expr, c *ast.CallExpr) {
	target := paren(g.lhsPlace(l), precPostfix)
	if c.Ellipsis.IsValid() {
		x := c.Args[1]
		if isStringType(g.typeOf(x)) {
			g.stmtLine(target+".extend_from_slice(", &call{recv: g.strRef(x), head: ".as_bytes", open: "(", close: ")"}, ");")
			return
		}
		if sameExpr(l, x) {
			g.line(target + ".extend_from_within(..);")
			return
		}
		g.stmtLine(target+".extend_from_slice(", g.ref(x), ");")
		return
	}
	for _, a := range c.Args[1:] {
		g.stmtLine(target+".push(", g.owned(a), ");")
	}
}

// opAssign writes x op= y.
func (g *gen) opAssign(s *ast.AssignStmt) {
	l, r := s.Lhs[0], s.Rhs[0]
	target := g.lhsPlace(l).flat()
	lt := g.typeOf(l)
	op := s.Tok.String()
	switch {
	case isStringType(lt):
		g.stmtLine(target+" += ", g.strArg(r), ";")
		return
	case s.Tok == token.AND_NOT_ASSIGN:
		g.stmtLine(target+" &= ", atom{"!" + paren(g.value(r), precUnary), precUnary}, ";")
		return
	case (s.Tok == token.QUO_ASSIGN || s.Tok == token.REM_ASSIGN) && isInt(lt) && !isUnsigned(lt) && !g.safeDivisor(r):
		method := ".wrapping_div"
		if s.Tok == token.REM_ASSIGN {
			method = ".wrapping_rem"
		}
		g.stmtLine(target+" = ", &call{recv: g.place(l), head: method, open: "(", close: ")", args: []node{g.value(r)}}, ";")
		return
	case (s.Tok == token.SHL_ASSIGN || s.Tok == token.SHR_ASSIGN) && g.info.Types[r].Value == nil:
		// x = rt::shl(x, s): assign has computed every index of the place
		// once, first, as Go evaluates it before the count.
		op := token.SHL
		if s.Tok == token.SHR_ASSIGN {
			op = token.SHR
		}
		target := g.lhsPlace(l).flat()
		g.stmtLine(target+" = ", g.varShift(op, g.place(l), r), ";")
		return
	case s.Tok == token.SHL_ASSIGN || s.Tok == token.SHR_ASSIGN:
		if w, ok := g.wideShift(lt, r); ok {
			// A count at least the width (see wideShift): the place is
			// evaluated once, as in Go.
			if s.Tok == token.SHR_ASSIGN && !isUnsigned(lt) {
				g.line(target + " >>= " + strconv.Itoa(w-1) + ";")
			} else {
				g.line(target + " = 0;")
			}
			return
		}
	}
	g.stmtLine(target+" "+op+" ", g.value(r), ";")
}

func (g *gen) ifStmt(s *ast.IfStmt) {
	if s.Init != nil {
		g.line("{")
		g.ind++
		g.stmt(s.Init)
		g.ifChain(s, "")
		g.ind--
		g.line("}")
		return
	}
	g.ifChain(s, "")
}

// ifChain writes if ... else if ... else, starting with prefix ("" or
// "} else ").
func (g *gen) ifChain(s *ast.IfStmt, prefix string) {
	if g.ifSome(s, prefix) {
		return
	}
	if prefix == "" && borrow.ChangedBeforeRead(g.info, g.res.Funcs, s.Cond) {
		// A condition that reads a variable before a call that changes
		// it: gc makes the call first (RFC 0017).
		g.hoistCalls(&ast.ExprStmt{X: s.Cond}, true)
	}
	g.stmtLine(prefix+"if ", g.value(s.Cond), " {")
	g.block(s.Body)
	g.elseOf(s)
}

// ifSome writes if p != nil { ... }, for an owned pointer p that may be
// nil (RFC 0017, step 2), as Rust does: if let Some(p) = p.as_deref() {
// ... }, where the body reads through the binding. if p != nil && c { ...
// } with no else is if let Some(p) = ... { if c { ... } }; Rust 1.85, the
// crates' oldest, has no let chains. It reports false for any other if.
func (g *gen) ifSome(s *ast.IfStmt, prefix string) bool {
	test, rest := s.Cond, ast.Expr(nil)
	if b, ok := ast.Unparen(s.Cond).(*ast.BinaryExpr); ok && b.Op == token.LAND && s.Else == nil {
		test, rest = b.X, b.Y
	}
	x, ok := g.someTest(test)
	if !ok || g.assignsRoot(s.Body, x) || rest != nil && g.assignsRoot(rest, x) {
		return false
	}
	want, rep := repOptRef, repRef
	if g.changesThrough(s.Body, x) || rest != nil && g.changesThrough(rest, x) {
		want, rep = repOptRefMut, repRefMut // the body changes it: as_deref_mut
	}
	src := g.ptrAs(x, want)
	if id, ok := ast.Unparen(x).(*ast.Ident); ok && want == repOptRefMut && g.ptrRep(x) == repOptRefMut && !g.usedAfterPos(id, s.End()) {
		// An Option<&mut T> not used after the if: moved into the
		// binding, not reborrowed (clippy's needless_option_as_deref).
		src = g.rawPtr(x)
	}
	name := g.bindName(x)
	key := types.ExprString(x)
	g.bound[key] = boundPtr{name: name, rep: rep}
	g.stmtLine(prefix+"if let Some("+name+") = ", src, " {")
	if rest != nil {
		g.ind++
		g.stmtLine("if ", g.value(rest), " {")
		g.block(s.Body)
		g.line("}")
		g.ind--
	} else {
		g.block(s.Body)
	}
	delete(g.bound, key)
	g.elseOf(s)
	return true
}

// walkTest returns p for a loop whose condition is p != nil, p a
// temporary borrow (RFC 0017, step 5) that the body does not assign or
// test again: the loop binds it, while let Some(n) = p.
func (g *gen) walkTest(s *ast.ForStmt) (ast.Expr, bool) {
	x, ok := g.someTest(s.Cond)
	if !ok {
		return nil, false
	}
	id, ok := x.(*ast.Ident)
	if !ok || !g.isBorrow(g.localVar(id)) || g.assignsRoot(s.Body, x) {
		return nil, false
	}
	return x, true
}

// someTest returns p for a test p != nil of an owned pointer p that may be
// nil, held in a variable, a parameter or a field of one.
func (g *gen) someTest(e ast.Expr) (ast.Expr, bool) {
	b, ok := ast.Unparen(e).(*ast.BinaryExpr)
	if !ok || b.Op != token.NEQ {
		return nil, false
	}
	x, y := ast.Unparen(b.X), ast.Unparen(b.Y)
	if g.info.Types[x].IsNil() {
		x, y = y, x
	}
	if !g.info.Types[y].IsNil() || !g.ownPtr(g.typeOf(x)) {
		return nil, false
	}
	// x is an Option: a pointer never nil is never compared with nil, and
	// a bound one is never tested again (assignsRoot).
	root := x
	for {
		sel, ok := root.(*ast.SelectorExpr)
		if !ok {
			break
		}
		root = ast.Unparen(sel.X) // a field: an owned pointer is in no package variable
	}
	id, ok := root.(*ast.Ident)
	return x, ok && g.localVar(id) != nil
}

// assignsRoot reports whether s, an if statement's body or the rest of
// its condition, assigns the variable that place x starts from, so that x
// may change under the binding, or uses x as a whole where an Option is
// wanted, or tests it again. (A pointer's address is never taken: that
// would be a pointer to a pointer.)
func (g *gen) assignsRoot(s ast.Node, x ast.Expr) bool {
	root := x
	for {
		sel, ok := root.(*ast.SelectorExpr)
		if !ok {
			break
		}
		root = ast.Unparen(sel.X)
	}
	v := g.localVar(root.(*ast.Ident))
	key := types.ExprString(x)
	found := false
	var stack []ast.Node
	ast.Inspect(s, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		if e, ok := n.(ast.Expr); ok && types.ExprString(e) == key && len(stack) > 1 {
			// x itself, used other than read through, passed to a call or
			// tested for nil, would be the binding where the Option is
			// wanted: returned or stored.
			switch p := stack[len(stack)-2].(type) {
			case *ast.SelectorExpr:
				found = found || p.X != e
			case *ast.CallExpr:
			case *ast.StarExpr:
				// *x, read or replaced through the binding (RFC 0017)
			default:
				found = true // returned, stored, or tested for nil again
			}
		}
		if a, ok := n.(*ast.AssignStmt); ok {
			for _, l := range a.Lhs {
				if id, ok := ast.Unparen(l).(*ast.Ident); ok && g.localVar(id) == v {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

// bindName names the binding of if let Some(..) = x: the variable's own
// name, or the field's.
func (g *gen) bindName(x ast.Expr) string {
	goName := ""
	switch q := x.(type) {
	case *ast.Ident:
		return g.varName(g.localVar(q)) // shadowed by its own binding
	case *ast.SelectorExpr:
		goName = q.Sel.Name
	}
	return g.lnames.name("", func(string) string { return snake(goName) })
}

// elseOf writes the else branch of if statement s and closes it.
func (g *gen) elseOf(s *ast.IfStmt) {
	switch e := s.Else.(type) {
	case nil:
		g.line("}")
	case *ast.IfStmt:
		if e.Init != nil || g.errs.Calls[e] != nil || borrow.ChangedBeforeRead(g.info, g.res.Funcs, e.Cond) {
			g.line("} else {")
			g.ind++
			g.body([]ast.Stmt{e}, false)
			g.ind--
			g.line("}")
			return
		}
		g.ifChain(e, "} else ")
	case *ast.BlockStmt:
		g.line("} else {")
		g.block(e)
		g.line("}")
	}
}

func (g *gen) branch(s *ast.BranchStmt) {
	for i := len(g.labels) - 1; i >= 0; i-- {
		l := g.labels[i]
		if s.Tok == token.BREAK {
			if l.isSwitch {
				return // a trailing break leaves the match arm anyway
			}
			g.line("break;")
			return
		}
		if l.isSwitch {
			continue
		}
		if l.post != nil {
			g.stmt(l.post)
		}
		g.line("continue;")
		return
	}
}

// forStmt writes a 3-clause, condition or infinite for loop.
func (g *gen) forStmt(s *ast.ForStmt) {
	if g.fast == nil && !g.noPlan {
		if p := g.planNest(s); p != nil {
			if g.specialize(s, p) {
				return
			}
			g.noPlan = true
			g.forBody(s)
			g.noPlan = false
			return
		}
	}
	g.forBody(s)
}

// negation returns the Go expression !cond for a comparison of integers,
// booleans or strings, its operator turned round (x >= 20 for !(x < 20),
// which clippy's nonminimal_bool asks for), or nil: a float comparison
// does not turn round, since NaN makes !(a < b) differ from a >= b.
func (g *gen) negation(cond ast.Expr) ast.Expr {
	flip := map[token.Token]token.Token{token.EQL: token.NEQ, token.NEQ: token.EQL, token.LSS: token.GEQ, token.GEQ: token.LSS, token.GTR: token.LEQ, token.LEQ: token.GTR}
	if b, ok := ast.Unparen(cond).(*ast.BinaryExpr); ok {
		if op, ok := flip[b.Op]; ok {
			if t, ok := g.typeOf(b.X).Underlying().(*types.Basic); ok && t.Info()&types.IsFloat == 0 && t.Info()&types.IsComplex == 0 {
				n := &ast.BinaryExpr{X: b.X, OpPos: b.OpPos, Op: op, Y: b.Y}
				g.info.Types[n] = g.info.Types[b]
				return n
			}
		}
	}
	return nil
}

// forBody writes a for statement.
func (g *gen) forBody(s *ast.ForStmt) {
	if g.res.Counted[s] {
		g.countedLoop(s)
		return
	}
	wrap := false
	if init, ok := s.Init.(*ast.AssignStmt); ok && init.Tok == token.DEFINE {
		for _, l := range init.Lhs {
			if g.usedNames[snake(l.(*ast.Ident).Name)] > 1 {
				wrap = true
			}
		}
	}
	if wrap {
		g.line("{")
		g.ind++
	}
	if s.Init != nil {
		g.stmt(s.Init)
	}
	key := ""
	switch x, ok := g.walkTest(s); {
	case ok:
		// for p := head; p != nil; p = p.Next: Rust's while let Some(n) = p,
		// the body and the step reading through the binding (RFC 0017,
		// step 5).
		elem := types.Unalias(g.typeOf(x)).(*types.Pointer).Elem().(*types.Named).Obj().Name()
		name := g.lnames.name("", func(string) string { return snake(elem) })
		key = types.ExprString(x)
		g.bound[key] = boundPtr{name: name, rep: repRef}
		g.line("while let Some(" + name + ") = " + g.varName(g.localVar(x.(*ast.Ident))) + " {")
	case s.Cond == nil:
		g.line("loop {")
	case borrow.ChangedBeforeRead(g.info, g.res.Funcs, s.Cond):
		// A condition that reads a variable before a call that changes it:
		// gc makes the call first, each time round (RFC 0017). Rust's loop
		// makes it first, then tests: loop { let call = ..; if !c { break; } }.
		g.line("loop {")
		g.ind++
		g.hoistCalls(&ast.ExprStmt{X: s.Cond}, true)
		if neg := g.negation(s.Cond); neg != nil {
			g.stmtLine("if ", g.value(neg), " {")
		} else {
			// A test that turns round only with a ! (a float comparison,
			// a && or a call): named first, as clippy prefers.
			more := g.lnames.name("", func(string) string { return "more" })
			g.stmtLine("let "+more+" = ", g.value(s.Cond), ";")
			g.line("if !" + more + " {")
		}
		g.ind++
		g.line("break;")
		g.ind--
		g.line("}")
		g.ind--
	default:
		g.stmtLine("while ", g.value(s.Cond), " {")
	}
	g.labels = append(g.labels, label{post: s.Post})
	g.block(s.Body)
	if s.Post != nil {
		g.ind++
		g.stmt(s.Post)
		g.ind--
	}
	delete(g.bound, key)
	g.labels = g.labels[:len(g.labels)-1]
	g.line("}")
	if wrap {
		g.ind--
		g.line("}")
	}
}

// countedLoop writes for i := a; i < b; i++ as for i in a..b.
func (g *gen) countedLoop(s *ast.ForStmt) {
	init := s.Init.(*ast.AssignStmt)
	v := g.localVar(init.Lhs[0].(*ast.Ident))
	cond := s.Cond.(*ast.BinaryExpr)
	start, end := g.value(init.Rhs[0]), g.value(cond.Y)
	bothConst := g.info.Types[init.Rhs[0]].Value != nil && g.info.Types[cond.Y].Value != nil
	switch {
	case g.fastVar(v):
		start, _, _ = g.uOperand(init.Rhs[0])
		end, _, _ = g.uOperand(cond.Y)
		if bothConst {
			start = g.uTyped(init.Rhs[0])
		}
	case bothConst:
		start = text(start.flat() + "_" + g.rustType(v.Type()))
	}
	op := ".."
	if cond.Op == token.LEQ {
		op = "..="
	}
	rng := atom{paren(start, precRange+1) + op + paren(end, precRange+1), precRange}
	name := "_"
	ast.Inspect(s.Body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && g.info.Uses[id] == v {
			name = g.varName(v)
		}
		return true
	})
	g.stmtLine("for "+name+" in ", rng, " {")
	g.labels = append(g.labels, label{})
	g.block(s.Body)
	g.labels = g.labels[:len(g.labels)-1]
	g.line("}")
}

// rangeStmt writes a range loop.
func (g *gen) rangeStmt(s *ast.RangeStmt) {
	t := g.typeOf(s.X)
	key := g.rangeVar(s.Key)
	value := g.rangeVar(s.Value)
	var pat string
	var iter node
	var first []string
	switch {
	case isStringType(t):
		g.rt["runes"] = true
		x := g.strRef(s.X)
		var arg node = atom{"&" + paren(x, precUnary), precUnary}
		switch {
		case g.res.Ranges[s] == borrow.RangeIndex:
			arg = atom{"&" + (&call{recv: x, head: ".clone", open: "(", close: ")"}).flat(), precUnary}
		case g.isRefVar(s.X):
			arg = x
		}
		pat = "_"
		if key != "_" || value != "_" {
			pat = "(" + key + ", " + value + ")"
		}
		iter = &call{head: "rt::runes", open: "(", close: ")", args: []node{arg}}
	case isInt(t):
		start := "0"
		if g.info.Types[s.X].Value != nil {
			start = "0_" + g.rustType(t)
		}
		pat = key
		iter = atom{start + ".." + paren(g.value(s.X), precRange+1), precRange}
	default:
		pat, iter, first = g.sliceRange(s, key, value)
	}
	g.stmtLine("for "+pat+" in ", iter, " {")
	g.ind++
	for _, l := range first {
		g.line(l)
	}
	g.ind--
	g.labels = append(g.labels, label{})
	g.block(s.Body)
	g.labels = g.labels[:len(g.labels)-1]
	g.line("}")
}

// isRefVar reports whether e is a variable bound to a borrow.
func (g *gen) isRefVar(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok || g.localVar(id) == nil {
		return false
	}
	return g.bindingOf(g.localVar(id)) != bOwned
}

// rangeVar returns the pattern for a range loop's key or value.
func (g *gen) rangeVar(e ast.Expr) string {
	id, ok := e.(*ast.Ident)
	if !ok || id.Name == "_" {
		return "_"
	}
	v := g.localVar(id)
	return g.mutPrefix(v) + g.varName(v)
}

// sliceRange returns the pattern and iterator of a range loop over a
// slice, and the lines that start its body.
func (g *gen) sliceRange(s *ast.RangeStmt, key, value string) (string, node, []string) {
	t := g.typeOf(s.X)
	elem := t.Underlying().(*types.Slice).Elem()
	copyElem := borrow.KindOf(elem) == borrow.Copy
	isPlace := g.isPlace(s.X)
	var x node
	if isPlace {
		x = g.place(s.X)
	} else {
		x = g.value(s.X)
	}
	xs := paren(x, precPostfix)
	keyName := strings.TrimPrefix(key, "mut ")
	valueName := strings.TrimPrefix(value, "mut ")
	length := func(cast bool) node {
		n := xs + ".len()"
		if cast {
			n += " as i64"
		}
		return atom{"0.." + n, precRange}
	}
	if g.res.Ranges[s] == borrow.RangeIndex {
		// The body changes the slice: index it, up to its length when the
		// loop started, as Go does.
		idx := keyName
		if key == "_" {
			idx = g.lnames.name("i", snake)
		}
		var first []string
		if value != "_" {
			el := xs + "[" + idx + " as usize]"
			if !copyElem {
				el += ".clone()"
			}
			first = append(first, "let "+value+" = "+el+";")
		}
		pat := idx
		if strings.HasPrefix(key, "mut ") {
			pat = key
		}
		return pat, length(true), first
	}
	if value == "_" {
		if key == "_" {
			return "_", length(false), nil
		}
		return key, length(true), nil
	}
	var iter node
	switch {
	case !isPlace:
		iter = x
		if key != "_" {
			iter = &call{recv: x, head: ".into_iter", open: "(", close: ")"}
		}
	case copyElem:
		iter = &call{recv: &call{recv: x, head: ".iter", open: "(", close: ")"}, head: ".copied", open: "(", close: ")"}
	case g.res.RangeMut[s]:
		iter = &call{recv: x, head: ".iter_mut", open: "(", close: ")"}
	case key == "_" && !g.res.RangeCopy[s]:
		iter = g.ref(s.X)
	default:
		iter = &call{recv: x, head: ".iter", open: "(", close: ")"}
	}
	var first []string
	valPat := value
	if g.res.RangeMut[s] {
		valPat = valueName // a &mut borrow of the element, itself not rebound
	}
	if !copyElem && isPlace && g.res.RangeCopy[s] {
		valPat = valueName
		first = append(first, "let mut "+valueName+" = "+valueName+".clone();")
	}
	if key == "_" {
		return valPat, iter, first
	}
	first = append([]string{"let " + key + " = " + keyName + " as i64;"}, first...)
	return "(" + keyName + ", " + valPat + ")", &call{recv: iter, head: ".enumerate", open: "(", close: ")"}, first
}

// switchStmt writes a switch as a match or as an if chain.
func (g *gen) switchStmt(s *ast.SwitchStmt) {
	if s.Init != nil {
		g.line("{")
		g.ind++
		g.stmt(s.Init)
	}
	if s.Tag != nil && borrow.ChangedBeforeRead(g.info, g.res.Funcs, s.Tag) {
		// A tag that reads a variable before a call that changes it: gc
		// makes the call first (RFC 0017).
		g.hoistCalls(&ast.ExprStmt{X: s.Tag}, true)
	}
	if g.matchable(s) {
		g.match(s)
	} else {
		g.switchIf(s)
	}
	if s.Init != nil {
		g.ind--
		g.line("}")
	}
}

// matchable reports whether a switch can be a Rust match: a tag of integer
// or string type and constant cases.
func (g *gen) matchable(s *ast.SwitchStmt) bool {
	if s.Tag == nil {
		return false
	}
	t := g.typeOf(s.Tag)
	if !isInt(t) && !isStringType(t) {
		return false
	}
	for _, c := range s.Body.List {
		for _, e := range c.(*ast.CaseClause).List {
			if g.info.Types[e].Value == nil {
				return false
			}
		}
	}
	return true
}

func (g *gen) match(s *ast.SwitchStmt) {
	tag := g.value(s.Tag)
	if isStringType(g.typeOf(s.Tag)) {
		tag = g.strOrd(s.Tag)
	}
	g.stmtLine("match ", tag, " {")
	g.ind++
	var def *ast.CaseClause
	for _, c := range s.Body.List {
		cc := c.(*ast.CaseClause)
		if cc.List == nil {
			def = cc
			continue
		}
		var pats []string
		for _, e := range cc.List {
			tv := g.info.Types[e]
			pats = append(pats, constLit(tv.Value, g.typeOf(s.Tag)))
		}
		g.arm(strings.Join(pats, " | "), cc)
	}
	if def != nil {
		g.arm("_", def)
	} else {
		g.line("_ => {}")
	}
	g.ind--
	g.line("}")
}

// arm writes one match arm.
func (g *gen) arm(pat string, cc *ast.CaseClause) {
	body := cc.Body
	if len(withoutBreak(body)) == 0 {
		g.line(pat + " => {}")
		return
	}
	if g.bareArm(pat, cc) {
		return
	}
	g.line(pat + " => {")
	g.labels = append(g.labels, label{isSwitch: true})
	g.ind++
	g.body(body, false)
	g.ind--
	g.labels = g.labels[:len(g.labels)-1]
	g.line("}")
}

// bareArm writes an arm whose only statement is a match or a loop the way
// rustfmt does, without a block around it: pat => match x { ... },. It
// reports false, and writes nothing, for any other arm, and for one with a
// comment, which keeps its block.
func (g *gen) bareArm(pat string, cc *ast.CaseClause) bool {
	body := withoutBreak(cc.Body)
	if len(body) != 1 || g.hasLineCommentsBetween(cc.Colon, cc.End()) {
		return false
	}
	switch s := body[0].(type) {
	case *ast.SwitchStmt:
		if !g.matchable(s) {
			return false
		}
	case *ast.ForStmt:
		if s.Init != nil || s.Cond != nil {
			return false
		}
	default:
		return false
	}
	var b strings.Builder
	out := g.out
	g.out = &b
	g.labels = append(g.labels, label{isSwitch: true})
	g.stmt(body[0])
	g.labels = g.labels[:len(g.labels)-1]
	g.out = out
	indent := strings.Repeat(" ", g.ind*indentWidth)
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	// The first line is "match ...", "loop {", or "{" for a switch with
	// an init statement: hoist never writes before a switch or a loop, and
	// a loop without init or condition is never specialized (ranges.go).
	first := strings.TrimPrefix(lines[0], indent)
	if first == "{" {
		// A block of its own, which scopes an init statement: the arm's
		// block scopes it as well, and rustfmt drops the inner one.
		g.line(pat + " => {")
		// It was written at the arm's indentation, so its lines are
		// already where the arm's body goes.
		g.out.WriteString(strings.Join(lines[1:len(lines)-1], "\n") + "\n")
		g.line("}")
		return true
	}
	lines[0] = indent + pat + " => " + first
	lines[len(lines)-1] += ","
	g.out.WriteString(strings.Join(lines, "\n") + "\n")
	return true
}

func withoutBreak(body []ast.Stmt) []ast.Stmt {
	if n := len(body); n > 0 {
		if b, ok := body[n-1].(*ast.BranchStmt); ok && b.Tok == token.BREAK {
			return body[:n-1]
		}
	}
	return body
}

// switchIf writes a switch as if ... else if ... else.
func (g *gen) switchIf(s *ast.SwitchStmt) {
	var def *ast.CaseClause
	prefix := ""
	nested := 0 // else blocks opened for cases whose calls come first
	for _, c := range s.Body.List {
		cc := c.(*ast.CaseClause)
		if cc.List == nil {
			def = cc
			continue
		}
		first := false
		for _, e := range cc.List {
			first = first || borrow.ChangedBeforeRead(g.info, g.res.Funcs, e)
		}
		if first {
			// A case that reads a variable before a call that changes it:
			// gc makes the call first, when it comes to the case, so the
			// calls go in an else of the cases before it (RFC 0017).
			if prefix != "" {
				g.line("} else {")
				g.ind++
				nested++
			}
			for _, e := range cc.List {
				g.hoistCalls(&ast.ExprStmt{X: e}, true)
			}
			prefix = ""
		}
		var cond node
		for _, e := range cc.List {
			var c node
			if s.Tag == nil {
				c = g.value(e)
			} else {
				c = g.compare(&ast.BinaryExpr{X: s.Tag, Op: token.EQL, Y: e})
			}
			if cond == nil {
				cond = c
			} else {
				cond = &binary{op: "||", l: cond, r: c, p: precOr}
			}
		}
		g.stmtLine(prefix+"if ", cond, " {")
		g.switchBody(cc.Body)
		prefix = "} else "
	}
	switch {
	case def != nil && prefix == "":
		g.line("{")
		g.switchBody(def.Body)
		g.line("}")
	case def != nil:
		g.line("} else {")
		g.switchBody(def.Body)
		g.line("}")
	case prefix != "":
		g.line("}")
	}
	for ; nested > 0; nested-- {
		g.ind--
		g.line("}")
	}
}

func (g *gen) switchBody(body []ast.Stmt) {
	g.labels = append(g.labels, label{isSwitch: true})
	g.ind++
	g.body(withoutBreak(body), false)
	g.ind--
	g.labels = g.labels[:len(g.labels)-1]
}

// errCall writes a call of a function that returns an error, with its
// check, as Rust's ?, a match, or if let Err.
func (g *gen) errCall(s ast.Stmt, ec *subset.ErrCall) {
	if ec.General {
		g.generalCall(ec) // RFC 0012
		return
	}
	callN := g.rawCall(ec.Call)
	var vals []string
	allBlank := true
	for _, v := range ec.Values {
		l := g.lhsPattern(v, ec.Define)
		if l != "_" {
			allBlank = false
		}
		vals = append(vals, l)
	}
	assignPrefix := ""
	if len(vals) > 0 && !allBlank {
		pat := strings.Join(vals, ", ")
		if len(vals) > 1 {
			pat = "(" + pat + ")"
		}
		if ec.Define {
			assignPrefix = "let " + pat + " = "
		} else {
			assignPrefix = pat + " = "
		}
	}
	switch {
	case ec.Err == nil && assignPrefix == "":
		g.stmtLine("let _ = ", callN, ";")
	case ec.Err == nil:
		g.stmtLine(assignPrefix, &call{recv: callN, head: ".unwrap_or_default", open: "(", close: ")"}, ";")
	case ec.Init && ec.Propagate && s.(*ast.IfStmt).Else == nil:
		// if err := f(); err != nil { return ..., err }: the values it
		// declares live only in the if, so this is f()?, as clippy asks.
		g.stmtLine("", atom{paren(callN, precPostfix) + "?", precPostfix}, ";")
	case ec.Init:
		g.initCheck(s.(*ast.IfStmt), ec, callN)
	case ec.Propagate:
		g.stmtLine(assignPrefix, atom{paren(callN, precPostfix) + "?", precPostfix}, ";")
	case assignPrefix == "":
		g.stmtLine("if let Err("+g.errBinding(ec, ec.Check.Body)+") = ", callN, " {")
		g.block(ec.Check.Body)
		g.line("}")
	default:
		g.stmtLine(assignPrefix+"match ", callN, " {")
		g.ind++
		g.line("Ok(value) => value,")
		var zeros []node
		for _, v := range ec.Values {
			zeros = append(zeros, g.zero(g.typeOf(v)))
		}
		switch {
		case len(ec.Check.Body.List) == 0 && !g.hasComments(ec.Check.Body):
			// Go leaves the values at their zero values.
			g.line("Err(_) => " + tuple(zeros).flat() + ",")
		default:
			g.line("Err(" + g.errBinding(ec, ec.Check.Body) + ") => {")
			g.block(ec.Check.Body)
			if !diverges(g.info, ec.Check.Body.List) {
				g.ind++
				g.line(tuple(zeros).flat())
				g.ind--
			}
			g.line("}")
		}
		g.ind--
		g.line("};")
	}
}

// errBinding returns the name the error is bound to in the check's body,
// or _ when the body does not use it.
func (g *gen) errBinding(ec *subset.ErrCall, body *ast.BlockStmt) string {
	v := g.localVar(ec.Err)
	used := false
	ast.Inspect(body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && g.info.Uses[id] == v {
			used = true
		}
		return true
	})
	if !used {
		return "_"
	}
	return g.varName(v)
}

// lhsPattern renders the left-hand side for one value of a call.
func (g *gen) lhsPattern(e ast.Expr, define bool) string {
	if id, ok := e.(*ast.Ident); ok {
		if id.Name == "_" {
			return "_"
		}
		if define {
			v := g.localVar(id)
			return g.mutPrefix(v) + g.varName(v)
		}
	}
	return g.place(e).flat()
}

// initCheck writes if v, err := f(); err != nil { ... } else { ... }.
func (g *gen) initCheck(s *ast.IfStmt, ec *subset.ErrCall, callN node) {
	errName := g.errBinding(ec, s.Body)
	if s.Else == nil {
		g.stmtLine("if let Err("+errName+") = ", callN, " {")
		g.block(s.Body)
		g.line("}")
		return
	}
	var vals []string
	for _, v := range ec.Values {
		vals = append(vals, g.lhsPattern(v, true))
	}
	ok := "_"
	if len(vals) == 1 {
		ok = vals[0]
	} else if len(vals) > 1 {
		ok = "(" + strings.Join(vals, ", ") + ")"
	}
	g.stmtLine("match ", callN, " {")
	g.ind++
	g.line("Err(" + errName + ") => {")
	g.block(s.Body)
	g.line("}")
	g.line("Ok(" + ok + ") => {")
	g.ind++
	switch e := s.Else.(type) {
	case *ast.BlockStmt:
		g.body(e.List, false)
	default:
		g.body([]ast.Stmt{e}, false)
	}
	g.ind--
	g.line("}")
	g.ind--
	g.line("}")
}

// diverges reports whether a block always ends in return, os.Exit, panic,
// break or continue, so it never falls through.
func diverges(info *types.Info, list []ast.Stmt) bool {
	if len(list) == 0 {
		return false
	}
	switch s := list[len(list)-1].(type) {
	case *ast.ReturnStmt, *ast.BranchStmt:
		return true
	case *ast.ExprStmt:
		c := ast.Unparen(s.X).(*ast.CallExpr) // the subset allows only calls
		if f, ok := ast.Unparen(c.Fun).(*ast.Ident); ok {
			return f.Name == "panic" && info.Uses[f] == types.Universe.Lookup("panic")
		}
		if f, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr); ok {
			fn, _ := info.Uses[f.Sel].(*types.Func)
			return fn != nil && fn.Pkg() != nil && fn.Pkg().Path() == "os" && fn.Name() == "Exit"
		}
	}
	return false
}

// hoist writes the arguments of a statement's call into variables first,
// when the borrow checker found that the call changes a variable a later
// argument reads (f(&p, p.X)). Go evaluates every argument before the call,
// and so do the variables.
func (g *gen) hoist(s ast.Stmt) {
	var e ast.Expr
	switch s := s.(type) {
	case *ast.ExprStmt:
		e = s.X
	case *ast.AssignStmt:
		if len(s.Rhs) == 1 {
			e = s.Rhs[0]
		}
	case *ast.ReturnStmt:
		if len(s.Results) == 1 {
			e = s.Results[0]
		}
	}
	c, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || !g.res.Hoist[c] {
		return
	}
	fn := g.calledFunc(c)
	for i, a := range c.Args {
		p := fn.Params[i]
		if p.Mode == borrow.RefMut || g.info.Types[a].Value != nil {
			continue // lent to be changed, or a constant, which nothing changes
		}
		name := g.lnames.name("", func(string) string { return "arg" })
		if p.Mode == borrow.Ref && borrow.KindOf(g.typeOf(a)) != borrow.Copy && g.isPlace(a) {
			// A borrow of a string the call changes: take a copy first.
			g.stmtLine("let "+name+" = ", &call{recv: g.place(a), head: ".clone", open: "(", close: ")"}, ";")
			g.hoisted[a] = "&" + name
			continue
		}
		g.stmtLine("let "+name+" = ", g.arg(a, p), ";")
		g.hoisted[a] = name
	}
}

// calledFunc returns the package function or method c calls.
func (g *gen) calledFunc(c *ast.CallExpr) *borrow.Func {
	if f, ok := ast.Unparen(c.Fun).(*ast.Ident); ok {
		return g.res.Funcs[g.info.Uses[f].(*types.Func)]
	}
	return g.res.Funcs[g.info.Uses[ast.Unparen(c.Fun).(*ast.SelectorExpr).Sel].(*types.Func)]
}

// sameExpr reports whether two expressions are the same simple place: a
// variable, a field of one, or *p.
func sameExpr(a, b ast.Expr) bool {
	return types.ExprString(ast.Unparen(a)) == types.ExprString(ast.Unparen(b))
}

// hoistIndexes computes the index operands of assignment targets into
// variables first, where Rust's order of evaluation would differ from Go's:
// Go evaluates them before the right-hand side and before any assignment.
// With all, every non-constant index is computed first (a tuple
// assignment); otherwise only an index that reads the slice it indexes.
func (g *gen) hoistIndexes(lhs []ast.Expr, all bool) {
	for _, l := range lhs {
		ast.Inspect(l, func(n ast.Node) bool {
			ix, ok := n.(*ast.IndexExpr)
			if !ok || g.info.Types[ix.Index].Value != nil || !isSlice(g.typeOf(ix.X)) {
				return true
			}
			root := g.rootVar(ix.X)
			if !all && !g.mentions(ix.Index, root) {
				return true
			}
			name := g.lnames.name("", func(string) string { return "idx" })
			g.stmtLine("let "+name+" = ", g.value(ix.Index), ";")
			g.hoistedIdx[ix.Index] = name
			return true
		})
	}
}

// mentions reports whether e uses variable v.
func (g *gen) mentions(e ast.Expr, v *types.Var) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && v != nil && g.info.Uses[id] == v {
			found = true
		}
		return true
	})
	return found
}

// hasComments reports whether a block holds comments not written yet.
func (g *gen) hasComments(b *ast.BlockStmt) bool {
	for _, cg := range g.comments {
		if !g.usedComments[cg] && cg.Pos() > b.Lbrace && cg.End() < b.Rbrace {
			return true
		}
	}
	return false
}

// hasLineCommentsBetween reports whether an unwritten comment falls between
// start and end, which would be lost by folding statements across the gap.
func (g *gen) hasLineCommentsBetween(start, end token.Pos) bool {
	for _, cg := range g.comments {
		if !g.usedComments[cg] && cg.Pos() >= start && cg.End() <= end {
			return true
		}
	}
	return false
}

// foldTailLetReturn folds `v := e` followed by `return v` at the end of a
// function body into the tail expression `e` in the simple cases clippy flags
// as let_and_return.
func (g *gen) foldTailLetReturn(s ast.Stmt, r *ast.ReturnStmt) (node, *ast.ReturnStmt, bool) {
	if g.errs.Returns[r] != 0 || len(r.Results) != 1 {
		return nil, nil, false
	}
	res := ast.Unparen(r.Results[0])
	if u, ok := res.(*ast.UnaryExpr); ok && g.ptrs.EscapeBox(u) != nil && g.unboxed(g.typeOf(u)) && g.ptrs.NonNilResult(g.fn.Obj) {
		res = ast.Unparen(u.X) // return &x of an unboxed value: x itself
	}
	rid, ok := res.(*ast.Ident)
	if !ok || rid.Name == "_" {
		return nil, nil, false
	}
	rdef := g.info.Uses[rid]
	if rdef == nil {
		return nil, nil, false
	}
	as, ok := s.(*ast.AssignStmt)
	if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
		return nil, nil, false
	}
	id, ok := as.Lhs[0].(*ast.Ident)
	if !ok || id.Name == "_" || g.info.Defs[id] != rdef {
		return nil, nil, false
	}
	if t := g.typeOf(id); g.errs.Calls[as] != nil || isPointerType(t) && !g.heapPtr(t) {
		return nil, nil, false // a borrow; a heap pointer is a value (RFC 0013)
	}
	if g.ptrs.RetBorrow(g.fn.Obj) != nil {
		return g.ptrAs(as.Rhs[0], g.retRep(g.fn.Obj)), r, true // r := n.Next; return r: the borrow lent back
	}
	if g.ownPtr(g.typeOf(id)) && g.ptrs.NonNilResult(g.fn.Obj) {
		g.boxSlot[ast.Unparen(as.Rhs[0])] = true // returned as the Box it is
	}
	return g.owned(as.Rhs[0]), r, true
}
