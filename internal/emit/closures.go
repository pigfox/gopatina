package emit

import (
	"go/ast"
	"go/types"
	"strings"
	"unicode/utf8"

	"github.com/pigfox/gopatina/internal/subset"
)

// closureDecl writes f := func(...) {...} (RFC 0018, step 1): a Rust
// closure, borrowing the variables it captures, mut when it changes one.
// Its body is written as the function the borrow checker checked it as,
// its returns its own.
func (g *gen) closureDecl(s *ast.AssignStmt, lit *ast.FuncLit) {
	v := g.info.Defs[s.Lhs[0].(*ast.Ident)].(*types.Var)
	cl := subset.Closures(g.pkg)[v]
	fn := g.res.Closures[v]
	sig := fn.Obj.Signature()
	var params []string
	for _, p := range fn.Params {
		name := "_"
		if p.Var.Name() != "_" && p.Var.Name() != "" {
			name = g.varName(p.Var)
		}
		if p.Mut {
			name = "mut " + name
		}
		params = append(params, name+": "+g.rustType(p.Var.Type()))
	}
	head := "let "
	if len(cl.Writes) > 0 {
		head += "mut "
	}
	head += g.varName(v) + " = |" + strings.Join(params, ", ") + "|"
	if sig.Results().Len() == 1 {
		head += " -> " + g.rustType(sig.Results().At(0).Type())
	}
	savedFn, savedRecv, savedStart, savedBody, savedDeferred := g.fn, g.recv, g.fnStart, g.fnBody, g.deferred
	savedOut := g.out
	var body strings.Builder
	g.out = &body
	g.fn, g.recv = fn, nil
	g.ind++
	g.funcBody(fn.Decl, fn.Obj, sig, fn.Params)
	g.ind--
	g.fn, g.recv, g.fnStart, g.fnBody, g.deferred = savedFn, savedRecv, savedStart, savedBody, savedDeferred
	g.out = savedOut
	// rustfmt writes an empty body as {}, and a body that is one
	// expression on the closure's own line when it fits.
	text := body.String()
	if text == "" {
		g.line(head + " {};")
		return
	}
	if expr := strings.TrimSpace(text); !strings.Contains(expr, "\n") && !strings.HasSuffix(expr, ";") && !strings.Contains(expr, "//") {
		if one := head + " { " + expr + " };"; g.ind*indentWidth+utf8.RuneCountInString(one) <= maxWidth {
			g.line(one)
			return
		}
	}
	g.line(head + " {")
	g.out.WriteString(text)
	g.line("};")
}
