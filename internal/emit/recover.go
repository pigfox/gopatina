package emit

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/pigfox/gopatina/internal/subset"
)

// recover (RFC 0020): a function that defers a literal calling recover
// runs its body in rt::recovering, which stops a recovered panic; recover's
// value is an Option<String>, the panic's text.

// recoveringBody writes the body of fd, a function that recovers: its
// named results are declared already, outside the closure, so that after a
// recovered panic it returns them as they are; an unnamed result is its
// zero value.
func (g *gen) recoveringBody(fd *ast.FuncDecl, sig *types.Signature) {
	g.rt["recovering"] = true
	g.deferred = g.lnames.name("", func(string) string { return "deferred" })
	g.line("rt::recovering(|" + g.deferred + "| {")
	g.ind++
	g.bodyTo(fd.Body.List, true, fd.Body.Rbrace)
	g.ind--
	res := sig.Results()
	if res.Len() == 0 {
		g.line("});")
		return
	}
	var named []string
	for i := 0; i < res.Len(); i++ {
		r := res.At(i)
		if r.Name() == "" || r.Name() == "_" {
			named = nil
			break
		}
		named = append(named, g.varName(r))
	}
	g.line("})")
	switch {
	case len(named) == 0:
		g.line(".unwrap_or_default()")
	case len(named) == 1:
		g.line(".unwrap_or(" + named[0] + ")")
	default:
		g.line(".unwrap_or((" + strings.Join(named, ", ") + "))")
	}
}

// recoverRef renders a reference to recover's value e: &rt::recover() or
// &r.
func (g *gen) recoverRef(e ast.Expr) node {
	if id, ok := ast.Unparen(e).(*ast.Ident); ok {
		return text("&" + g.varName(g.localVar(id)))
	}
	return text("&rt::recover()")
}

// recoverNil renders r == nil or r != nil of recover's value r, and
// reports whether e is one.
func (g *gen) recoverNil(e *ast.BinaryExpr) (node, bool) {
	if e.Op != token.EQL && e.Op != token.NEQ {
		return nil, false
	}
	x, y := e.X, e.Y
	if g.info.Types[x].IsNil() {
		x, y = y, x
	}
	if !g.info.Types[y].IsNil() || !subset.IsRecoverValue(g.pkg, x) {
		return nil, false
	}
	method := ".is_some"
	if e.Op == token.EQL {
		method = ".is_none"
	}
	g.rt["recovering"] = true
	var recv node = text("rt::recover()")
	if id, ok := ast.Unparen(x).(*ast.Ident); ok {
		recv = text(g.varName(g.localVar(id)))
	}
	return &call{recv: recv, head: method, open: "(", close: ")"}, true
}
