package emit

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"
	"strings"
	"unicode"

	"github.com/pigfox/gopatina/internal/borrow"
)

// A binding is how a Go variable is held in Rust.
type binding int

const (
	bOwned   binding = iota // let x: T, or an owned parameter
	bRef                    // x: &T (or &str, &[T])
	bRefMut                 // x: &mut T (or &mut [T])
	bRefElem                // x: &T bound to an element by a range loop
)

// bindingOf returns how v is held.
func (g *gen) bindingOf(v *types.Var) binding {
	if p := g.res.Param(v); p != nil {
		switch p.Mode {
		case borrow.Ref:
			return bRef
		case borrow.RefMut:
			return bRefMut
		}
		return bOwned
	}
	if ptr := g.res.Pointers[v]; ptr != nil && ptr.Target != nil {
		if ptr.Exclusive {
			return bRefMut
		}
		return bRef
	}
	if g.refElem[v] {
		return bRefElem
	}
	return bOwned
}

// localVar returns the local variable or parameter id refers to, or nil.
func (g *gen) localVar(id *ast.Ident) *types.Var {
	obj := g.info.Uses[id]
	if obj == nil {
		obj = g.info.Defs[id]
	}
	v, ok := obj.(*types.Var)
	if !ok || v.IsField() || v.Parent() == g.pkg.Types.Scope() {
		return nil
	}
	return v
}

// varName returns the Rust name of a local variable or parameter.
func (g *gen) varName(v *types.Var) string {
	if v == g.recv {
		return "self"
	}
	return g.localName(v.Name(), v, snake)
}

// localName returns the Rust name for a local object, giving it one if it
// has none yet.
func (g *gen) localName(goName string, obj types.Object, conv func(string) string) string {
	if n, ok := g.locals[obj]; ok {
		return n
	}
	n := g.lnames.name(goName, conv)
	g.locals[obj] = n
	return n
}

// typeOf returns the type of e, including a name being declared.
func (g *gen) typeOf(e ast.Expr) types.Type {
	if id, ok := e.(*ast.Ident); ok {
		if v := g.localVar(id); v != nil {
			return v.Type()
		}
	}
	return g.info.Types[e].Type
}

// rootVar returns the variable at the root of an assignment target, which
// the subset checker requires to be a variable, a field, an element or *p.
func (g *gen) rootVar(e ast.Expr) *types.Var {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		default:
			return g.localVar(x.(*ast.Ident))
		}
	}
}

// isPlace reports whether e is a place: a variable, a field, an element or
// *p, rooted at a local variable.
func (g *gen) isPlace(e ast.Expr) bool {
	e = ast.Unparen(e)
	switch x := e.(type) {
	case *ast.Ident:
		return g.localVar(x) != nil
	case *ast.SelectorExpr:
		sel := g.info.Selections[x]
		return sel != nil && sel.Kind() == types.FieldVal
	case *ast.IndexExpr:
		return isSlice(g.typeOf(x.X))
	case *ast.StarExpr:
		return true
	}
	return false
}

// place renders a place expression.
func (g *gen) place(e ast.Expr) node {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		return text(g.varName(g.localVar(x)))
	case *ast.SelectorExpr:
		base := ast.Unparen(x.X)
		if st, ok := base.(*ast.StarExpr); ok {
			base = st.X
		}
		var b node
		if g.isPlace(base) {
			b = g.place(base)
		} else {
			b = g.value(base)
		}
		field := g.fields[g.info.Selections[x].Obj().(*types.Var)]
		return atom{paren(b, precPostfix) + "." + field, precPostfix}
	case *ast.IndexExpr:
		var b node
		if g.isPlace(x.X) {
			b = g.place(x.X)
		} else {
			b = g.value(x.X)
		}
		return atom{paren(b, precPostfix) + "[" + g.index(x.Index) + "]", precPostfix}
	}
	// *p
	v := g.localVar(ast.Unparen(ast.Unparen(e).(*ast.StarExpr).X).(*ast.Ident))
	if g.bindingOf(v) == bOwned {
		return text(g.varName(v)) // a pointer that owns its value
	}
	return atom{"*" + g.varName(v), precUnary}
}

// index renders an index operand as a usize.
func (g *gen) index(e ast.Expr) string {
	if tv := g.info.Types[e]; tv.Value != nil {
		return tv.Value.ExactString()
	}
	if name, ok := g.hoistedIdx[e]; ok {
		return name + " as usize"
	}
	return paren(g.value(e), precCast) + " as usize"
}

// value renders e as a value: a copy of a Copy value, or a value the
// caller owns.
func (g *gen) value(e ast.Expr) node {
	tv := g.info.Types[e]
	if tv.Value != nil {
		return g.constant(tv.Value, tv.Type)
	}
	e = ast.Unparen(e)
	t := tv.Type
	switch x := e.(type) {
	case *ast.Ident:
		if tv.IsNil() {
			return text("Vec::<" + g.rustType(g.nils[e].Underlying().(*types.Slice).Elem()) + ">::new()")
		}
		v := g.localVar(x)
		if g.bindingOf(v) != bOwned && borrow.KindOf(t) != borrow.Copy && !isPointerType(v.Type()) {
			return g.owned(e)
		}
		return text(g.varName(v))
	case *ast.SelectorExpr, *ast.IndexExpr, *ast.StarExpr:
		if ix, ok := x.(*ast.IndexExpr); ok && isStringType(g.typeOf(ix.X)) {
			return atom{paren(g.strRef(ix.X), precPostfix) + ".as_bytes()[" + g.index(ix.Index) + "]", precPostfix}
		}
		return g.place(e) // a Copy value; others are handed over with owned
	case *ast.CallExpr:
		return g.call(x)
	case *ast.BinaryExpr:
		return g.binary(x)
	case *ast.UnaryExpr:
		return g.unary(x)
	}
	return g.composite(e.(*ast.CompositeLit))
}

func isPointerType(t types.Type) bool {
	_, ok := t.(*types.Pointer)
	return ok
}

// stdValue renders os.Stdout and os.Stderr.
func (g *gen) stdValue(sel *ast.SelectorExpr) node {
	if sel.Sel.Name == "Stderr" {
		return text("&mut std::io::stderr()")
	}
	return text("&mut std::io::stdout()")
}

// owned renders e as a value handed to a new owner: moved when the borrow
// checker found it is not used again, copied when it is.
func (g *gen) owned(e ast.Expr) node {
	tv := g.info.Types[e]
	if tv.Value != nil {
		if tv.Value.Kind() == constant.String {
			return &call{head: "String::from", open: "(", close: ")", args: []node{g.constant(tv.Value, tv.Type)}}
		}
		return g.constant(tv.Value, tv.Type)
	}
	e = ast.Unparen(e)
	if !g.isPlace(e) {
		return g.value(e)
	}
	p := g.place(e)
	if borrow.KindOf(tv.Type) == borrow.Copy {
		return p
	}
	if g.res.Actions[e] == borrow.Move {
		return p
	}
	method := ".clone"
	if id, ok := e.(*ast.Ident); ok {
		v := g.localVar(id)
		if b := g.bindingOf(v); (b == bRef || b == bRefMut) && isStringType(v.Type()) {
			method = ".to_string" // a &str
		}
	}
	return &call{recv: p, head: method, open: "(", close: ")"}
}

// ref renders e as a shared borrow.
func (g *gen) ref(e ast.Expr) node {
	e = ast.Unparen(e)
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		return g.ref(u.X)
	}
	if id, ok := e.(*ast.Ident); ok && g.localVar(id) != nil {
		if b := g.bindingOf(g.localVar(id)); b != bOwned {
			return text(g.varName(g.localVar(id)))
		}
	}
	if g.isPlace(e) {
		return atom{"&" + paren(g.place(e), precUnary), precUnary}
	}
	return &addrOf{"&", g.value(e)}
}

// mutRef renders e as an exclusive borrow.
func (g *gen) mutRef(e ast.Expr) node {
	e = ast.Unparen(e)
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		return g.mutRef(u.X)
	}
	if id, ok := e.(*ast.Ident); ok && g.localVar(id) != nil {
		if b := g.bindingOf(g.localVar(id)); b == bRefMut || b == bRef {
			return text(g.varName(g.localVar(id)))
		}
	}
	if sel, ok := e.(*ast.SelectorExpr); ok && g.info.Selections[sel] == nil {
		return g.stdValue(sel)
	}
	if g.isPlace(e) {
		return atom{"&mut " + paren(g.place(e), precUnary), precUnary}
	}
	return &addrOf{"&mut ", g.value(e)}
}

// strRef renders a string expression as something that derefs to str.
func (g *gen) strRef(e ast.Expr) node {
	tv := g.info.Types[e]
	if tv.Value != nil {
		return g.constant(tv.Value, tv.Type)
	}
	if g.isPlace(e) {
		return g.place(e)
	}
	return g.value(e)
}

// constant renders a constant value of type t as a Rust literal.
func (g *gen) constant(v constant.Value, t types.Type) node {
	return text(constLit(v, t))
}

// constLit renders a constant as a Rust literal of type t.
func constLit(v constant.Value, t types.Type) string {
	b, _ := t.Underlying().(*types.Basic)
	switch {
	case b != nil && b.Info()&types.IsString != 0:
		return strLit(constant.StringVal(v))
	case b != nil && b.Info()&types.IsBoolean != 0:
		return strconv.FormatBool(constant.BoolVal(v))
	case b != nil && b.Info()&types.IsFloat != 0:
		f, _ := constant.Float64Val(constant.ToFloat(v))
		return floatLit(f)
	case b != nil && b.Kind() == types.Uint8:
		n, _ := constant.Uint64Val(constant.ToInt(v))
		if n >= 0x20 && n < 0x7f && n != '\'' && n != '\\' {
			return "b'" + string(rune(n)) + "'"
		}
		return strconv.FormatUint(n, 10)
	}
	return constant.ToInt(v).ExactString()
}

// floatLit renders f so that Rust reads back exactly the same float64.
// A Go constant is always finite.
func floatLit(f float64) string {
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if strings.ContainsAny(s, "e") {
		mant, exp, _ := strings.Cut(s, "e")
		if !strings.Contains(mant, ".") {
			mant += ".0"
		}
		return mant + "e" + strings.TrimPrefix(exp, "+")
	}
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// strLit renders s as a Rust string literal.
func strLit(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == 0:
			b.WriteString(`\0`)
		case !unicode.IsPrint(r) || r < 0x20:
			fmt.Fprintf(&b, `\u{%x}`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// zero renders the zero value of t.
func (g *gen) zero(t types.Type) node {
	switch u := t.Underlying().(type) {
	case *types.Basic:
		switch {
		case u.Info()&types.IsString != 0:
			return text("String::new()")
		case u.Info()&types.IsBoolean != 0:
			return text("false")
		case u.Info()&types.IsFloat != 0:
			return text("0.0")
		}
		return text("0")
	case *types.Slice:
		return text("Vec::new()")
	}
	return text(g.rustType(t) + "::default()")
}

// typedZero renders the zero value of t with its type spelled out, for
// places where Rust cannot infer it.
func (g *gen) typedZero(t types.Type) node {
	if b, ok := t.Underlying().(*types.Basic); ok && b.Info()&types.IsNumeric != 0 {
		return text(g.zero(t).flat() + "_" + g.rustType(t))
	}
	return g.zero(t)
}

func (g *gen) unary(e *ast.UnaryExpr) node {
	switch e.Op {
	case token.SUB:
		if isUnsigned(g.typeOf(e.X)) {
			return &call{recv: g.value(e.X), head: ".wrapping_neg", open: "(", close: ")"}
		}
		return atom{"-" + paren(g.value(e.X), precUnary), precUnary}
	case token.NOT, token.XOR:
		return atom{"!" + paren(g.value(e.X), precUnary), precUnary}
	case token.AND:
		return g.ref(e.X)
	}
	return g.value(e.X) // unary +
}

var binOps = map[token.Token]struct {
	op   string
	prec int
}{
	token.ADD: {"+", precAdd}, token.SUB: {"-", precAdd},
	token.MUL: {"*", precMul}, token.QUO: {"/", precMul}, token.REM: {"%", precMul},
	token.AND: {"&", precBitAnd}, token.OR: {"|", precBitOr}, token.XOR: {"^", precBitXor},
	token.SHL: {"<<", precShift}, token.SHR: {">>", precShift},
	token.EQL: {"==", precCmp}, token.NEQ: {"!=", precCmp},
	token.LSS: {"<", precCmp}, token.LEQ: {"<=", precCmp},
	token.GTR: {">", precCmp}, token.GEQ: {">=", precCmp},
	token.LAND: {"&&", precAnd}, token.LOR: {"||", precOr},
}

func (g *gen) binary(e *ast.BinaryExpr) node {
	xt := g.typeOf(e.X)
	switch {
	case e.Op == token.ADD && isStringType(xt):
		return g.concat(e)
	case e.Op == token.AND_NOT:
		return &binary{op: "&", l: g.value(e.X), r: atom{"!" + paren(g.value(e.Y), precUnary), precUnary}, p: precBitAnd}
	case (e.Op == token.QUO || e.Op == token.REM) && isInt(xt) && !isUnsigned(xt) && !g.safeDivisor(e.Y):
		method := ".wrapping_div"
		if e.Op == token.REM {
			method = ".wrapping_rem"
		}
		return &call{recv: g.receiverValue(e.X), head: method, open: "(", close: ")", args: []node{g.value(e.Y)}}
	case binOps[e.Op].prec == precCmp:
		return g.compare(e)
	}
	op := binOps[e.Op]
	l := g.value(e.X)
	if (e.Op == token.SHL || e.Op == token.SHR) && l.prec() == precCast {
		l = atom{"(" + l.flat() + ")", precAtom}
	}
	return &binary{op: op.op, l: l, r: g.value(e.Y), p: op.prec}
}

// safeDivisor reports whether dividing by e can neither overflow nor need
// Go's wrapping: a constant other than 0 and -1.
func (g *gen) safeDivisor(e ast.Expr) bool {
	v := g.info.Types[e].Value
	return v != nil && constant.Sign(v) != 0 && constant.Compare(v, token.NEQ, constant.MakeInt64(-1))
}

// receiverValue renders e as the receiver of a method call; a constant gets
// its type suffix, since Rust cannot call a method on an untyped literal.
func (g *gen) receiverValue(e ast.Expr) node {
	tv := g.info.Types[e]
	if tv.Value != nil {
		return text(constLit(tv.Value, tv.Type) + "_" + g.rustType(tv.Type))
	}
	return g.value(e)
}

// compare renders a comparison.
func (g *gen) compare(e *ast.BinaryExpr) node {
	op := binOps[e.Op]
	xt := g.typeOf(e.X)
	var l, r node
	switch {
	case isStringType(xt) && e.Op != token.EQL && e.Op != token.NEQ:
		l, r = g.strOrd(e.X), g.strOrd(e.Y)
	case borrow.KindOf(xt) != borrow.Copy:
		l, r = g.cmpOperand(e.X), g.cmpOperand(e.Y)
	default:
		l, r = g.value(e.X), g.value(e.Y)
	}
	if l.prec() == precCast && (e.Op == token.LSS || e.Op == token.LEQ) {
		// x as i64 < y would read as the start of generic arguments.
		l = atom{"(" + l.flat() + ")", precAtom}
	}
	return &binary{op: op.op, l: l, r: r, p: op.prec, nonAssoc: true}
}

// cmpOperand renders an operand of == or != on a string or struct.
func (g *gen) cmpOperand(e ast.Expr) node {
	tv := g.info.Types[e]
	if tv.Value != nil {
		return g.constant(tv.Value, tv.Type)
	}
	if g.isPlace(e) {
		if id, ok := ast.Unparen(e).(*ast.Ident); ok {
			v := g.localVar(id)
			b := g.bindingOf(v)
			if b == bRefElem || (b != bOwned && !isStringType(v.Type())) {
				return atom{"*" + g.varName(v), precUnary}
			}
		}
		return g.place(e)
	}
	return g.value(e)
}

// strOrd renders a string operand of an ordering comparison as a &str.
func (g *gen) strOrd(e ast.Expr) node {
	tv := g.info.Types[e]
	if tv.Value != nil {
		return g.constant(tv.Value, tv.Type)
	}
	if id, ok := ast.Unparen(e).(*ast.Ident); ok && g.bindingOf(g.localVar(id)) == bRef {
		return text(g.varName(g.localVar(id)))
	}
	return &call{recv: g.strRef(e), head: ".as_str", open: "(", close: ")"}
}

// concat renders a chain of string + as one format! call.
func (g *gen) concat(e *ast.BinaryExpr) node {
	var parts []ast.Expr
	var walk func(x ast.Expr)
	walk = func(x ast.Expr) {
		if b, ok := ast.Unparen(x).(*ast.BinaryExpr); ok && b.Op == token.ADD && g.info.Types[b].Value == nil {
			walk(b.X)
			walk(b.Y)
			return
		}
		parts = append(parts, x)
	}
	walk(e)
	var f strings.Builder
	var args []node
	for _, p := range parts {
		if tv := g.info.Types[p]; tv.Value != nil {
			f.WriteString(escapeFormat(constant.StringVal(tv.Value)))
			continue
		}
		f.WriteString("{}")
		args = append(args, g.strRef(p))
	}
	return &call{head: "format!", open: "(", close: ")", args: append([]node{text(strLit(f.String()))}, args...)}
}

// escapeFormat escapes literal text for a format string, before it is put
// in a string literal.
func escapeFormat(s string) string {
	return strings.NewReplacer("{", "{{", "}", "}}").Replace(s)
}

// composite renders a composite literal.
func (g *gen) composite(e *ast.CompositeLit) node {
	t := g.typeOf(e)
	if _, ok := t.Underlying().(*types.Slice); ok {
		if len(e.Elts) == 0 {
			return text("Vec::new()")
		}
		var elems []node
		for _, el := range e.Elts {
			elems = append(elems, g.owned(el))
		}
		return &call{head: "vec!", open: "[", close: "]", args: elems, array: true}
	}
	u := t.Underlying().(*types.Struct)
	lit := &structLit{name: g.types[t.(*types.Named).Obj()]}
	set := map[int]bool{}
	for i, el := range e.Elts {
		idx := i
		val := el
		if kv, ok := el.(*ast.KeyValueExpr); ok {
			name := kv.Key.(*ast.Ident).Name
			for j := 0; j < u.NumFields(); j++ {
				if u.Field(j).Name() == name {
					idx = j
				}
			}
			val = kv.Value
		}
		set[idx] = true
		lit.fields = append(lit.fields, fieldInit{name: g.fields[u.Field(idx)], value: g.owned(val)})
	}
	if len(set) < u.NumFields() {
		lit.base = "Default::default()"
	}
	return lit
}
