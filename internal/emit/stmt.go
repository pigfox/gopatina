package emit

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/pigfox/gopatina/internal/borrow"
	"github.com/pigfox/gopatina/internal/subset"
)

// funcDecl writes a function or method.
func (g *gen) funcDecl(fd *ast.FuncDecl) {
	obj := g.info.Defs[fd.Name].(*types.Func)
	g.fn = g.res.Funcs[obj]
	g.locals = map[types.Object]string{}
	g.lnames = newNamer()
	for f, n := range g.funcs {
		if f.Signature().Recv() == nil {
			g.lnames.taken[n] = f.Name()
		}
	}
	g.recv = nil
	g.usedNames = map[string]int{}
	g.countNames(fd)
	g.docComment(fd.Doc)
	sig := obj.Signature()
	var params []string
	if g.fn.Recv != nil {
		g.recv = g.fn.Recv.Var
		params = append(params, g.selfParam(g.fn.Recv))
	}
	for _, p := range g.fn.Params {
		name := "_"
		if p.Var.Name() != "_" && p.Var.Name() != "" {
			name = g.varName(p.Var)
		}
		if p.Mut && (p.Mode == borrow.ByValue || p.Mode == borrow.Owned) {
			name = "mut " + name
		}
		params = append(params, name+": "+g.paramType(p.Var.Type(), p.Mode))
	}
	vis := ""
	if obj.Exported() && obj.Name() != "main" {
		vis = "pub "
	}
	ret := g.resultType(obj)
	if ret != "" {
		ret = " -> " + ret
	}
	head := vis + "fn " + g.funcs[obj] + "("
	sigLine := head + strings.Join(params, ", ") + ")" + ret + " {"
	if g.ind*indentWidth+len(sigLine) <= maxWidth {
		g.line(sigLine)
	} else {
		g.line(head)
		g.ind++
		for _, p := range params {
			g.line(p + ",")
		}
		g.ind--
		g.line(")" + ret + " {")
	}
	g.ind++
	if obj.Name() == "main" && g.fn.Recv == nil {
		g.rt["exit_on_panic"] = true
		g.line("rt::exit_on_panic();")
	}
	res := sig.Results()
	for i := 0; i < res.Len(); i++ {
		if r := res.At(i); r.Name() != "" && r.Name() != "_" {
			g.letZero(r)
		}
	}
	g.fnStart = fd.Body.Lbrace
	g.bodyTo(fd.Body.List, true, fd.Body.Rbrace)
	g.ind--
	g.line("}")
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
		ts = append(ts, g.rustType(res.At(i).Type()))
	}
	t := strings.Join(ts, ", ")
	if len(ts) != 1 {
		t = "(" + t + ")"
	}
	if lowered {
		g.rt["Error"] = true
		return "Result<" + t + ", rt::Error>"
	}
	if len(ts) == 0 {
		return ""
	}
	return t
}

// letZero declares v with its zero value.
func (g *gen) letZero(v *types.Var) {
	g.line("let " + g.mutPrefix(v) + g.varName(v) + ": " + g.rustType(v.Type()) + " = " + g.zero(v.Type()).flat() + ";")
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
		switch {
		case g.errs.Calls[s] != nil:
			ec := g.errs.Calls[s]
			g.errCall(s, ec)
			if ec.Check != nil && !ec.Init {
				i++ // the check is written with the call
				last = list[i]
			}
		case isReturn(s) && fnBody && i == len(list)-1:
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

func isReturn(s ast.Stmt) bool {
	_, ok := s.(*ast.ReturnStmt)
	return ok
}

// commentsBefore writes the comments of the current function that come
// before pos and have not been written yet.
func (g *gen) commentsBefore(pos token.Pos) {
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
	g.hoist(s)
	switch s := s.(type) {
	case *ast.ExprStmt:
		g.exprStmt(s.X)
	case *ast.AssignStmt:
		g.assign(s)
	case *ast.IncDecStmt:
		g.hoistIndexes([]ast.Expr{s.X}, false)
		one := "1"
		if isFloat(g.typeOf(s.X)) {
			one = "1.0"
		}
		op := " += " + one + ";"
		if s.Tok == token.DEC {
			op = " -= " + one + ";"
		}
		g.line(g.place(s.X).flat() + op)
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
		return g.value(results[0])
	case subset.ReturnOk:
		vals := g.ownedAll(results[:len(results)-1])
		if len(vals) == 0 {
			vals = []node{text("()")}
		}
		return &call{head: "Ok", open: "(", close: ")", args: []node{tuple(vals)}}
	case subset.ReturnErr:
		last := results[len(results)-1]
		var errNode node
		if id, ok := ast.Unparen(last).(*ast.Ident); ok {
			errNode = text(g.varName(g.localVar(id)))
		} else {
			errNode = g.value(last)
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
	if isPointerType(v.Type()) {
		g.letPointer(v, e)
		return
	}
	prefix := "let " + g.mutPrefix(v) + g.varName(v)
	if typed || g.needsType(e) {
		prefix += ": " + g.rustType(v.Type())
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
	g.hoistIndexes(s.Lhs, len(s.Lhs) > 1)
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
		g.stmtLine(g.place(l).flat()+" = ", g.owned(r), ";")
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
	return g.place(e).flat()
}

// multiLet declares several variables at once: Go evaluates every value
// before any is assigned, and so does a tuple.
func (g *gen) multiLet(lhs, rhs []ast.Expr) {
	g.letTypes = nil
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
	target := paren(g.place(l), precPostfix)
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
	target := g.place(l).flat()
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
	g.stmtLine(prefix+"if ", g.value(s.Cond), " {")
	g.block(s.Body)
	switch e := s.Else.(type) {
	case nil:
		g.line("}")
	case *ast.IfStmt:
		if e.Init != nil || g.errs.Calls[e] != nil {
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
	if s.Cond == nil {
		g.line("loop {")
	} else {
		g.stmtLine("while ", g.value(s.Cond), " {")
	}
	g.labels = append(g.labels, label{post: s.Post})
	g.block(s.Body)
	if s.Post != nil {
		g.ind++
		g.stmt(s.Post)
		g.ind--
	}
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
	if g.info.Types[init.Rhs[0]].Value != nil && g.info.Types[cond.Y].Value != nil {
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
	case key == "_" && !g.res.RangeCopy[s]:
		iter = g.ref(s.X)
	default:
		iter = &call{recv: x, head: ".iter", open: "(", close: ")"}
	}
	var first []string
	valPat := value
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
		g.arm(strings.Join(pats, " | "), cc.Body)
	}
	if def != nil {
		g.arm("_", def.Body)
	} else {
		g.line("_ => {}")
	}
	g.ind--
	g.line("}")
}

// arm writes one match arm.
func (g *gen) arm(pat string, body []ast.Stmt) {
	if len(withoutBreak(body)) == 0 {
		g.line(pat + " => {}")
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
	for _, c := range s.Body.List {
		cc := c.(*ast.CaseClause)
		if cc.List == nil {
			def = cc
			continue
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
	callN := g.value(ec.Call)
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
		if p.Mode == borrow.RefMut {
			continue
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
	rid, ok := ast.Unparen(r.Results[0]).(*ast.Ident)
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
	if g.errs.Calls[as] != nil || isPointerType(g.typeOf(id)) {
		return nil, nil, false
	}
	return g.owned(as.Rhs[0]), r, true
}
