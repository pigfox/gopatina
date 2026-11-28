package emit

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// Maps (RFC 0019, step 1): a local map is an rt::Map, a HashMap with a
// fixed hasher, so every run is the same. Go reads a missing key as the
// zero value, and prints a map with its keys sorted.

func isMapType(t types.Type) bool {
	_, ok := t.Underlying().(*types.Map)
	return ok
}

// mapIndex returns e as m[k] of a map, if it is one.
func (g *gen) mapIndex(e ast.Expr) (*ast.IndexExpr, bool) {
	ix, ok := ast.Unparen(e).(*ast.IndexExpr)
	if !ok || !isMapType(g.typeOf(ix.X)) {
		return nil, false
	}
	return ix, true
}

// mapType renders map type t.
func (g *gen) mapType(t *types.Map) string {
	g.rt["Map"] = true
	return "rt::Map<" + g.rustType(t.Key()) + ", " + g.rustType(t.Elem()) + ">"
}

// mapKey renders key k of a map as the reference get and remove take: a
// &str for a string, else a reference to the key, typed.
func (g *gen) mapKey(k ast.Expr) node {
	tv := g.info.Types[k]
	if isStringType(tv.Type) {
		n := g.strRef(k)
		if tv.Value != nil {
			return n // a literal is a &str already
		}
		return atom{"&" + paren(n, precUnary), precUnary}
	}
	if tv.Value != nil {
		return text("&" + constLit(tv.Value, tv.Type) + "_" + g.rustType(tv.Type))
	}
	return atom{"&" + paren(g.value(k), precUnary), precUnary}
}

// mapGet renders a read of m[k]: the value, or the zero value for a
// missing key.
func (g *gen) mapGet(ix *ast.IndexExpr) node {
	g.rt["map_get"] = true
	return &call{head: "rt::map_get", open: "(", close: ")", args: []node{g.ref(ix.X), g.mapKey(ix.Index)}}
}

// mapLit renders a map literal, its entries inserted in source order.
func (g *gen) mapLit(e *ast.CompositeLit) node {
	g.rt["Map"] = true
	if len(e.Elts) == 0 {
		return text("rt::Map::default()")
	}
	var pairs []node
	for _, el := range e.Elts {
		kv := el.(*ast.KeyValueExpr)
		pairs = append(pairs, &call{open: "(", close: ")", args: []node{g.owned(kv.Key), g.owned(kv.Value)}})
	}
	return &call{head: "rt::Map::from_iter", open: "(", close: ")", args: []node{&call{open: "[", close: "]", args: pairs, array: true}}}
}

// mapStmt writes an assignment or ++/-- whose target is an element of a
// map, or v, ok := m[k], and reports whether s is one.
func (g *gen) mapStmt(s ast.Stmt) bool {
	if inc, ok := s.(*ast.IncDecStmt); ok {
		ix, ok := g.mapIndex(inc.X)
		if !ok {
			return false
		}
		op := "+="
		if inc.Tok == token.DEC {
			op = "-="
		}
		g.mapUpdate(ix, op, text("1"))
		return true
	}
	as := s.(*ast.AssignStmt)
	if len(as.Lhs) == 2 && len(as.Rhs) == 1 {
		ix, ok := g.mapIndex(as.Rhs[0])
		if !ok {
			return false
		}
		g.commaOk(as, ix)
		return true
	}
	if len(as.Lhs) != 1 {
		return false
	}
	ix, ok := g.mapIndex(as.Lhs[0])
	if !ok {
		return false
	}
	if as.Tok == token.ASSIGN {
		g.stmtLine("", &call{recv: text(g.varName(g.rootVar(ix.X))), head: ".insert", open: "(", close: ")", args: []node{g.owned(ix.Index), g.owned(as.Rhs[0])}}, ";")
		return true
	}
	g.mapUpdate(ix, as.Tok.String(), g.value(as.Rhs[0]))
	return true
}

// mapUpdate writes m[k] op= v, which Go reads as the zero value for a
// missing key: *m.entry(k).or_default() op= v.
func (g *gen) mapUpdate(ix *ast.IndexExpr, op string, v node) {
	entry := &call{recv: &call{recv: text(g.varName(g.rootVar(ix.X))), head: ".entry", open: "(", close: ")", args: []node{g.owned(ix.Index)}}, head: ".or_default", open: "(", close: ")"}
	g.stmtLine("*", entry, " "+op+" "+v.flat()+";")
}

// commaOk writes v, ok := m[k] (or =): the value, or the zero value and
// false for a missing key.
func (g *gen) commaOk(s *ast.AssignStmt, ix *ast.IndexExpr) {
	g.rt["map_get_ok"] = true
	var names []string
	for _, l := range s.Lhs {
		id := l.(*ast.Ident)
		if id.Name == "_" {
			names = append(names, "_")
			continue
		}
		v := g.localVar(id)
		name := g.varName(v)
		if s.Tok == token.DEFINE && g.res.Mut[v] {
			name = "mut " + name
		}
		names = append(names, name)
	}
	head := "(" + strings.Join(names, ", ") + ") = "
	if s.Tok == token.DEFINE {
		head = "let " + head
	}
	g.stmtLine(head, &call{head: "rt::map_get_ok", open: "(", close: ")", args: []node{g.ref(ix.X), g.mapKey(ix.Index)}}, ";")
}

// mapShow renders %v of the map that p references, of type t.
func (g *gen) mapShow(t *types.Map, p node) node {
	g.rt["show_map"] = true
	return &call{head: "rt::show_map", open: "(", close: ")", args: []node{
		p,
		text("|k| " + g.show(t.Key(), "k")),
		text("|v| " + g.show(t.Elem(), "v")),
	}}
}

// mapRange returns the pattern and iterator of for k, v := range m, and
// the lines that start its body (RFC 0019, step 3): m.iter(), or m.keys()
// or m.values() for one of them, in the hash table's order, which Go
// leaves unspecified. A Copy key or value is bound by pattern, &k; a
// string, or one the body changes, is copied in a first line.
func (g *gen) mapRange(s *ast.RangeStmt) (string, node, []string) {
	mt := g.typeOf(s.X).Underlying().(*types.Map)
	var first []string
	bind := func(e ast.Expr, t types.Type) string {
		id, ok := e.(*ast.Ident)
		if !ok || id.Name == "_" {
			return "_"
		}
		v := g.localVar(id)
		name := g.varName(v)
		switch {
		case !isStringType(t) && g.mutPrefix(v) == "":
			return "&" + name
		case isStringType(t):
			first = append(first, "let "+g.mutPrefix(v)+name+" = "+name+".clone();")
		default:
			first = append(first, "let "+g.mutPrefix(v)+name+" = *"+name+";")
		}
		return name
	}
	key, value := bind(s.Key, mt.Key()), "_"
	if s.Value != nil {
		value = bind(s.Value, mt.Elem())
	}
	m := g.place(s.X)
	switch {
	case value == "_" && key == "_":
		return "_", &call{recv: m, head: ".iter", open: "(", close: ")"}, first
	case value == "_":
		return key, &call{recv: m, head: ".keys", open: "(", close: ")"}, first
	case key == "_":
		return value, &call{recv: m, head: ".values", open: "(", close: ")"}, first
	}
	return "(" + key + ", " + value + ")", &call{recv: m, head: ".iter", open: "(", close: ")"}, first
}
