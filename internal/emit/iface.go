package emit

import (
	"fmt"
	"go/ast"
	"go/types"
	"sort"
	"strings"

	"github.com/pigfox/gopatina/internal/borrow"
	"github.com/pigfox/gopatina/internal/subset"
)

// Interface values (RFC 0012). The program's dynamic types are the
// variants of one enum, Dyn; an interface value is Iface, an Option of a
// reference-counted Dyn, None for a nil interface. Values are converted
// with boxed(Dyn::V(x)), so the reference count is chosen in one place.
// Method calls through an interface are dispatcher functions matching the
// dynamic types that have the method.

// ifaceState is the interface part of a translation.
type ifaceState struct {
	plan     *subset.IfacePlan
	variants []string            // the Rust variant of each plan.Dyn
	inBox    map[ast.Expr]bool   // expressions being rendered inside boxed(...)
	used     bool                // some interface item is used
	calls    map[string]dispatch // dispatchers by key
	callKeys []string
	misses   map[string]types.Type // missing-method functions by name
	missKeys []string
	gets     map[string]types.Type // x.(T) and v, ok := x.(T) helpers by name
	getKeys  []string
	slices   bool                  // iface_slice is used
	goError  bool                  // errors.New or fmt.Errorf is used
	wrap     bool                  // fmt.Errorf with %w is used
	ases     map[string]types.Type // errors.As helpers by name
	asKeys   []string
	helpers  map[string]bool // error helpers used (errvals.go)
}

// dispatch is one dispatcher: a method called through an interface.
type dispatch struct {
	name string
	m    *types.Func // the interface's method
}

func (g *gen) initIfaces() {
	g.ifc = &ifaceState{
		plan:    subset.Ifaces(g.pkg),
		inBox:   map[ast.Expr]bool{},
		calls:   map[string]dispatch{},
		misses:  map[string]types.Type{},
		gets:    map[string]types.Type{},
		helpers: map[string]bool{},
		ases:    map[string]types.Type{},
	}
	seen := map[string]int{}
	for _, t := range g.ifc.plan.Dyn {
		n := variantName(t)
		seen[n]++
		if seen[n] > 1 {
			n += fmt.Sprint(seen[n])
		}
		g.ifc.variants = append(g.ifc.variants, n)
	}
}

// variantName is the Dyn variant of a dynamic type, from its Go name.
func variantName(t types.Type) string {
	switch t := types.Unalias(t).(type) {
	case *types.Named:
		n := t.Obj().Name()
		n = strings.ToUpper(n[:1]) + n[1:]
		if p := t.Obj().Pkg(); p != nil && p.Name() != "main" {
			n = strings.ToUpper(p.Name()[:1]) + p.Name()[1:] + n
		}
		return n
	case *types.Basic:
		n := types.Typ[t.Kind()].Name()
		return strings.ToUpper(n[:1]) + n[1:]
	}
	// The checker lets no other kind of type be converted to an interface.
	return "Slice" + variantName(t.(*types.Slice).Elem())
}

// goTypeName is a type's name as Go's %T and panic messages print it.
func goTypeName(t types.Type) string {
	switch u := types.Unalias(t).(type) {
	case *types.Basic:
		return types.Typ[u.Kind()].Name() // int32, not rune
	case *types.Slice:
		return "[]" + goTypeName(u.Elem())
	case *types.Interface:
		if u.NumMethods() == 0 {
			return "interface {}"
		}
	}
	return types.TypeString(t, func(p *types.Package) string { return p.Name() })
}

// variant returns the Dyn variant of t, or "" if no value of type t is
// ever converted to an interface.
func (g *gen) variant(t types.Type) string {
	for i, d := range g.ifc.plan.Dyn {
		if types.Identical(d, t) {
			return g.ifc.variants[i]
		}
	}
	return ""
}

// root names an interface item at the crate root, from the package being
// written.
func (g *gen) root(name string) string {
	g.ifc.used = true
	if g.cur == nil || g.cur == g.pkg.Types {
		return name
	}
	return "crate::" + name
}

// boxedExpr reports whether e is converted to an interface here, and is not
// already being rendered inside its conversion.
func (g *gen) boxedExpr(e ast.Expr) bool {
	if g.ifc == nil || g.ifc.inBox[e] {
		return false
	}
	_, ok := g.ifc.plan.Boxed[e]
	return ok
}

// box renders e converted to an interface: boxed(Dyn::V(e)).
func (g *gen) box(e ast.Expr) node {
	g.ifc.inBox[e] = true
	defer delete(g.ifc.inBox, e)
	v := g.variant(g.ifc.plan.Boxed[e])
	return &call{head: g.root("boxed"), open: "(", close: ")", args: []node{
		&call{head: g.root("Dyn::" + v), open: "(", close: ")", args: []node{g.owned(e)}},
	}}
}

// copyOut renders the value a reference v to a T stands for: *v for a
// Copy type, else a clone.
func copyOut(t types.Type, v string) string {
	if borrow.KindOf(t) == borrow.Copy {
		return "*" + v
	}
	return v + ".clone()"
}

// ifaceCall renders x.M(args) where x is an interface value.
func (g *gen) ifaceCall(f *ast.SelectorExpr, m *types.Func, e *ast.CallExpr) node {
	key := m.Name() + " " + types.TypeString(m.Signature(), nil)
	d, ok := g.ifc.calls[key]
	if !ok {
		name := "dyn_" + snake(m.Name())
		for n := 2; g.dispatchNamed(name); n++ {
			name = fmt.Sprintf("dyn_%s_%d", snake(m.Name()), n)
		}
		d = dispatch{name: name, m: m}
		g.ifc.calls[key] = d
		g.ifc.callKeys = append(g.ifc.callKeys, key)
	}
	args := []node{g.ref(f.X)}
	for _, a := range e.Args {
		args = append(args, g.owned(a))
	}
	return &call{head: g.root(d.name), open: "(", close: ")", args: args}
}

func (g *gen) dispatchNamed(name string) bool {
	for _, d := range g.ifc.calls {
		if d.name == name {
			return true
		}
	}
	return false
}

// missing returns the name of the function telling which method of
// interface t a dynamic value lacks.
func (g *gen) missing(t types.Type) string {
	name := "iface_missing_" + snake(variantName(t))
	if _, ok := g.ifc.misses[name]; !ok {
		g.ifc.misses[name] = t
		g.ifc.missKeys = append(g.ifc.missKeys, name)
	}
	return g.root(name)
}

// getter returns the name of the helper for x.(T) (get) or v, ok :=
// x.(T) (try) to a concrete type T.
func (g *gen) getter(kind string, t types.Type) string {
	name := "iface_" + kind + "_" + snake(variantName(t))
	if _, ok := g.ifc.gets[name]; !ok {
		g.ifc.gets[name] = t
		g.ifc.getKeys = append(g.ifc.getKeys, name)
	}
	return g.root(name)
}

// typeAssert renders x.(T), or v, ok := x.(T) as a tuple.
func (g *gen) typeAssert(e *ast.TypeAssertExpr) node {
	t := g.typeOf(e.Type)
	commaOk := g.ifc.plan.CommaOk[e]
	if subset.IsIface(t) {
		miss := text(g.missing(t))
		if commaOk {
			return &call{head: g.root("iface_try"), open: "(", close: ")", args: []node{g.ref(e.X), miss}}
		}
		return &call{head: g.root("iface_assert"), open: "(", close: ")", args: []node{g.owned(e.X), miss, text(strLit(goTypeName(t)))}}
	}
	if commaOk {
		return &call{head: g.getter("try", t), open: "(", close: ")", args: []node{g.ref(e.X)}}
	}
	return &call{head: g.getter("get", t), open: "(", close: ")", args: []node{g.ref(e.X), text(strLit(goTypeName(g.typeOf(e.X))))}}
}

// typeSwitch writes switch [v :=] x.(type) as a match on the dynamic value.
func (g *gen) typeSwitch(s *ast.TypeSwitchStmt) {
	var x ast.Expr
	switch a := s.Assign.(type) {
	case *ast.AssignStmt:
		x = a.Rhs[0].(*ast.TypeAssertExpr).X
	case *ast.ExprStmt:
		x = a.X.(*ast.TypeAssertExpr).X
	}
	xt := g.typeOf(x)
	g.line("{")
	g.ind++
	g.stmtLine("let dyn_x = ", g.ref(x), ";")
	g.line("match dyn_x.as_deref() {")
	g.ind++
	var def *ast.CaseClause
	covered := map[string]bool{}
	nilCovered := false
	for _, c := range s.Body.List {
		cc := c.(*ast.CaseClause)
		if cc.List == nil {
			def = cc
			continue
		}
		var pats []string
		single := len(cc.List) == 1
		var guard string
		for _, te := range cc.List {
			tv := g.info.Types[te]
			switch {
			case tv.IsNil():
				if !nilCovered {
					pats = append(pats, "None")
					nilCovered = true
				}
			case subset.IsIface(tv.Type):
				guard = g.missing(tv.Type)
			default:
				if v := g.variant(tv.Type); v != "" && !covered[v] {
					pats = append(pats, "Some("+g.root("Dyn::"+v)+"(v))")
					covered[v] = true
				}
			}
		}
		var bind string
		if obj, ok := g.info.Implicits[cc].(*types.Var); ok {
			name := g.varName(obj)
			switch {
			case single && guard == "" && len(pats) == 1 && pats[0] != "None":
				bind = "let " + g.mutPrefix(obj) + name + " = " + copyOut(obj.Type(), "v") + ";"
			default:
				bind = "let " + g.mutPrefix(obj) + name + " = dyn_x.clone();"
			}
		}
		switch {
		case guard != "" && len(pats) == 0:
			g.typeArm("Some(d) if "+guard+"(d).is_none()", bind, cc)
		case guard != "":
			// An interface case beside other types: its own arm first,
			// then theirs, with one body each.
			g.typeArm("Some(d) if "+guard+"(d).is_none()", bind, cc)
			g.typeArm(strings.Join(pats, " | "), bind, cc)
		case len(pats) > 0:
			if len(pats) > 1 {
				pats = rebindAny(pats)
			}
			g.typeArm(strings.Join(pats, " | "), bind, cc)
		}
	}
	var bind string
	if def != nil {
		if obj, ok := g.info.Implicits[def].(*types.Var); ok {
			bind = "let " + g.mutPrefix(obj) + g.varName(obj) + " = dyn_x.clone();"
		}
		g.typeArm("_", bind, def)
	} else {
		g.line("_ => {}")
	}
	_ = xt
	g.ind--
	g.line("}")
	g.ind--
	g.line("}")
}

// rebindAny turns the patterns of a case with several types into ones that
// bind nothing, since its variable has the interface's type.
func rebindAny(pats []string) []string {
	var out []string
	for _, p := range pats {
		out = append(out, strings.Replace(p, "(v))", "(_))", 1))
	}
	return out
}

// typeArm writes one arm of a type switch.
func (g *gen) typeArm(pat, bind string, cc *ast.CaseClause) {
	if strings.Contains(pat, "(v)") && bind == "" {
		pat = strings.Replace(pat, "(v)", "(_)", -1)
	}
	g.line(pat + " => {")
	g.labels = append(g.labels, label{isSwitch: true})
	g.ind++
	if bind != "" {
		g.line(bind)
	}
	g.body(cc.Body, false)
	g.ind--
	g.labels = g.labels[:len(g.labels)-1]
	g.line("}")
}

// ifaceVerb renders %v or %T of an interface value.
func (g *gen) ifaceVerb(verb rune, e ast.Expr) node {
	fn := "iface_show"
	if verb == 'T' {
		fn = "iface_type"
	}
	return &call{head: g.root(fn), open: "(", close: ")", args: []node{g.ref(e)}}
}

// ifaceCompare renders x == y or x != y where one side is an interface.
func (g *gen) ifaceCompare(e *ast.BinaryExpr) node {
	neq := e.Op.String() == "!="
	for _, pair := range [][2]ast.Expr{{e.X, e.Y}, {e.Y, e.X}} {
		if g.info.Types[pair[1]].IsNil() {
			m := ".is_none"
			if neq {
				m = ".is_some"
			}
			return &call{recv: atom{paren(g.ifaceOperand(pair[0]), precPostfix), precPostfix}, head: m, open: "(", close: ")"}
		}
	}
	op := binOps[e.Op]
	return &binary{op: op.op, l: g.ifaceOperand(e.X), r: g.ifaceOperand(e.Y), p: op.prec, nonAssoc: true}
}

// ifaceOperand renders an operand of an interface comparison.
func (g *gen) ifaceOperand(e ast.Expr) node {
	if g.boxedExpr(e) {
		return g.box(e)
	}
	if g.isPlace(ast.Unparen(e)) {
		return g.place(ast.Unparen(e))
	}
	return g.value(e)
}

// ifaceItems writes the crate's interface items: Dyn, Iface, boxed, the
// comparison, %v and %T, the dispatchers, the missing-method functions and
// the assertion helpers. It writes nothing for a program without
// interface values.
func (g *gen) ifaceItems() {
	if !g.ifc.used && len(g.ifc.plan.Dyn) == 0 {
		return
	}
	dyn := g.ifc.plan.Dyn
	vs := g.ifc.variants
	n := len(dyn) + len(g.builtins()) // variants, with Go's own error types
	g.line("/// The dynamic types of the program's interface values (RFC 0012).")
	if n == 0 {
		g.line("pub enum Dyn {}")
	} else {
		g.line("pub enum Dyn {")
		for i, t := range dyn {
			g.line("    " + vs[i] + "(" + g.rustType(t) + "),")
		}
		for _, b := range g.builtins() {
			g.line("    " + b.variant + "(" + b.payload + "),")
		}
		g.line("}")
	}
	g.line("")
	g.line("/// An interface value: its dynamic value, or None for a nil interface. The")
	g.line("/// value cannot be changed through the interface, so copies share it.")
	g.line("pub type Iface = Option<std::rc::Rc<Dyn>>;")
	g.line("")
	g.line("/// Converts a value to an interface value.")
	g.line("pub fn boxed(d: Dyn) -> Iface {")
	g.line("    Some(std::rc::Rc::new(d))")
	g.line("}")
	g.line("")
	g.line("/// Go's == on two dynamic values: the same type and equal values; a type")
	g.line("/// that is not comparable panics.")
	g.line("impl PartialEq for Dyn {")
	g.line("    fn eq(&self, other: &Self) -> bool {")
	if n == 0 {
		g.line("        match *self {}")
	} else {
		g.line("        match (self, other) {")
		for i, t := range dyn {
			if types.Comparable(t) {
				g.line("            (Dyn::" + vs[i] + "(a), Dyn::" + vs[i] + "(b)) => a == b,")
			} else {
				g.line("            (Dyn::" + vs[i] + "(_), Dyn::" + vs[i] + "(_)) => {")
				msg := "\"gopatina-user-panic:runtime error: comparing uncomparable type " + goTypeName(t) + "\""
				if len("                panic!("+msg+")") <= maxWidth {
					g.line("                panic!(" + msg + ")")
				} else {
					g.line("                panic!(")
					g.line("                    " + msg)
					g.line("                )")
				}
				g.line("            }")
			}
		}
		for _, b := range g.builtins() {
			// Go compares the pointers errors.New and fmt.Errorf return.
			g.line("            (Dyn::" + b.variant + "(a), Dyn::" + b.variant + "(b)) => std::ptr::eq(a, b),")
		}
		if n > 1 {
			g.line("            _ => false,")
		}
		g.line("        }")
	}
	g.line("    }")
	g.line("}")
	g.line("")
	g.line("/// The Go name of a dynamic type, as %T prints it.")
	g.line("pub fn iface_name(d: Option<&Dyn>) -> &'static str {")
	g.line("    match d {")
	g.line("        None => \"<nil>\",")
	for i, t := range dyn {
		g.line("        Some(Dyn::" + vs[i] + "(_)) => " + strLit(goTypeName(t)) + ",")
	}
	for _, b := range g.builtins() {
		g.line("        Some(Dyn::" + b.variant + "(_)) => " + strLit(b.goName) + ",")
	}
	g.noDyn("        ")
	g.line("    }")
	g.line("}")
	g.line("")
	g.line("/// %T of an interface value.")
	g.line("pub fn iface_type(x: &Iface) -> &'static str {")
	g.line("    iface_name(x.as_deref())")
	g.line("}")
	g.line("")
	g.line("/// %v of an interface value.")
	g.line("pub fn iface_show(x: &Iface) -> String {")
	g.line("    match x.as_deref() {")
	g.line("        None => String::from(\"<nil>\"),")
	for i, t := range dyn {
		g.line("        Some(Dyn::" + vs[i] + "(v)) => " + g.show(t, "v") + ",")
	}
	for _, b := range g.builtins() {
		g.line("        Some(Dyn::" + b.variant + "(v)) => " + b.methods[0].call + ",")
	}
	g.noDyn("        ")
	g.line("    }")
	g.line("}")
	g.line("")
	g.line("/// The panic of a failed x.(T) to a concrete type T.")
	g.line("pub fn iface_assert_fail(d: Option<&Dyn>, iface: &str, want: &str) -> ! {")
	g.line("    match d {")
	g.line("        None => panic!(\"gopatina-user-panic:interface conversion: {iface} is nil, not {want}\"),")
	g.line("        Some(_) => panic!(")
	g.line("            \"gopatina-user-panic:interface conversion: {iface} is {}, not {want}\",")
	g.line("            iface_name(d)")
	g.line("        ),")
	g.line("    }")
	g.line("}")
	g.line("")
	g.line("/// x.(I) to an interface type I: x, or Go's panic.")
	g.line("pub fn iface_assert(x: Iface, missing: fn(&Dyn) -> Option<&'static str>, want: &str) -> Iface {")
	g.line("    match x.as_deref() {")
	g.line("        None => panic!(\"gopatina-user-panic:interface conversion: interface is nil, not {want}\"),")
	g.line("        Some(d) => {")
	g.line("            if let Some(m) = missing(d) {")
	g.line("                panic!(")
	g.line("                    \"gopatina-user-panic:interface conversion: {} is not {want}: missing method {m}\",")
	g.line("                    iface_name(Some(d))")
	g.line("                )")
	g.line("            }")
	g.line("        }")
	g.line("    }")
	g.line("    x")
	g.line("}")
	g.line("")
	g.line("/// v, ok := x.(I) to an interface type I.")
	g.line("pub fn iface_try(x: &Iface, missing: fn(&Dyn) -> Option<&'static str>) -> (Iface, bool) {")
	g.line("    match x.as_deref() {")
	g.line("        Some(d) if missing(d).is_none() => (x.clone(), true),")
	g.line("        _ => (None, false),")
	g.line("    }")
	g.line("}")
	if g.ifc.slices {
		g.line("")
		g.line("/// %v of a slice: its elements' %v, in brackets.")
		g.line("pub fn iface_slice<T>(v: &[T], show: impl Fn(&T) -> String) -> String {")
		g.line("    let parts: Vec<String> = v.iter().map(show).collect();")
		g.line("    format!(\"[{}]\", parts.join(\" \"))")
		g.line("}")
	}
	for _, name := range g.ifc.getKeys {
		g.getterFn(name, g.ifc.gets[name])
	}
	g.errorsAPI()
	for _, name := range g.ifc.missKeys {
		g.missingFn(name, g.ifc.misses[name])
	}
	for _, key := range g.ifc.callKeys {
		g.dispatcher(g.ifc.calls[key])
	}
	g.errHelpers()
}

// missingFn writes the function telling which method of interface it a
// dynamic value lacks: the first by name, as Go's panic names it.
func (g *gen) missingFn(name string, t types.Type) {
	it := t.Underlying().(*types.Interface)
	byMissing := map[string][]string{}
	var order []string
	for i, t := range g.ifc.plan.Dyn {
		m := ""
		if f, _ := types.MissingMethod(t, it, false); f != nil {
			m = f.Name()
		}
		if _, ok := byMissing[m]; !ok {
			order = append(order, m)
		}
		byMissing[m] = append(byMissing[m], "Dyn::"+g.ifc.variants[i]+"(_)")
	}
	for _, b := range g.builtins() {
		m := b.missing(it)
		if _, ok := byMissing[m]; !ok {
			order = append(order, m)
		}
		byMissing[m] = append(byMissing[m], "Dyn::"+b.variant+"(_)")
	}
	g.line("")
	g.line("/// The first method of " + goTypeName(t) + " a dynamic value lacks, if any.")
	g.line("pub fn " + strings.TrimPrefix(name, "crate::") + "(d: &Dyn) -> Option<&'static str> {")
	if len(order) == 0 {
		g.line("    match *d {}")
		g.line("}")
		return
	}
	g.line("    match d {")
	sort.SliceStable(order, func(i, j int) bool { return order[i] < order[j] })
	for _, m := range order {
		res := "None"
		if m != "" {
			res = "Some(" + strLit(m) + ")"
		}
		g.patArm("        ", byMissing[m], res+",")
	}
	g.line("    }")
	g.line("}")
}

// getterFn writes the helper of x.(T) (iface_get_t) or v, ok := x.(T)
// (iface_try_t) to the concrete type t.
func (g *gen) getterFn(name string, t types.Type) {
	v := g.variant(t)
	ty := g.rustType(t)
	g.line("")
	if strings.HasPrefix(name, "iface_try_") {
		g.line("/// v, ok := x.(" + goTypeName(t) + ").")
		g.line("pub fn " + name + "(x: &Iface) -> (" + ty + ", bool) {")
		if v == "" {
			g.line("    (" + g.zero(t).flat() + ", false)")
		} else {
			g.line("    match x.as_deref() {")
			g.line("        Some(Dyn::" + v + "(v)) => (" + copyOut(t, "v") + ", true),")
			g.line("        _ => (" + g.zero(t).flat() + ", false),")
			g.line("    }")
		}
		g.line("}")
		return
	}
	g.line("/// x.(" + goTypeName(t) + "): the dynamic value, or Go's panic.")
	g.line("pub fn " + name + "(x: &Iface, iface: &str) -> " + ty + " {")
	want := strLit(goTypeName(t))
	if v == "" {
		g.line("    iface_assert_fail(x.as_deref(), iface, " + want + ")")
	} else {
		g.line("    match x.as_deref() {")
		g.line("        Some(Dyn::" + v + "(v)) => " + copyOut(t, "v") + ",")
		g.line("        d => iface_assert_fail(d, iface, " + want + "),")
		g.line("    }")
	}
	g.line("}")
}

// dispatcher writes the function calling method d.m on an interface
// value's dynamic value.
func (g *gen) dispatcher(d dispatch) {
	sig := d.m.Signature()
	params := []string{"x: &Iface"}
	var args []string
	for i := 0; i < sig.Params().Len(); i++ {
		n := fmt.Sprintf("a%d", i)
		params = append(params, n+": "+g.rustType(sig.Params().At(i).Type()))
		args = append(args, n)
	}
	ret := g.resultType(d.m)
	if ret != "" {
		ret = " -> " + ret
	}
	g.line("")
	g.line("/// Calls " + d.m.Name() + " on an interface value's dynamic value.")
	g.line("pub fn " + d.name + "(" + strings.Join(params, ", ") + ")" + ret + " {")
	g.line("    match x.as_deref() {")
	all := true
	for i, t := range g.ifc.plan.Dyn {
		obj, _, _ := types.LookupFieldOrMethod(t, false, d.m.Pkg(), d.m.Name())
		m, ok := obj.(*types.Func)
		if !ok || !types.Identical(dropRecv(m.Signature()), dropRecv(sig)) {
			all = false
			continue
		}
		g.line("        Some(Dyn::" + g.ifc.variants[i] + "(v)) => " + g.methodOn(m, t, "v", args) + ",")
	}
	g.line("        None => panic!(")
	g.line("            \"gopatina-user-panic:runtime error: invalid memory address or nil pointer dereference\"")
	g.line("        ),")
	for _, b := range g.builtins() {
		if call := b.method(d.m); call != "" {
			g.line("        Some(Dyn::" + b.variant + "(v)) => " + call + ",")
		} else {
			all = false
		}
	}
	if !all {
		g.line("        _ => unreachable!(\"only values whose type has the method are converted\"),")
	}
	g.noDyn("        ")
	g.line("    }")
	g.line("}")
}

func dropRecv(s *types.Signature) *types.Signature {
	return types.NewSignatureType(nil, nil, nil, s.Params(), s.Results(), s.Variadic())
}

// methodOn renders a call of method m of type t on v, a reference to a t,
// with the rendered owned arguments. A dynamic type's methods are its
// program's own (a provided type is never converted to an interface), and
// their parameters are never changed through (the checker rejects slices
// in an interface method's parameters).
func (g *gen) methodOn(m *types.Func, t types.Type, v string, args []string) string {
	fn := g.res.Funcs[m]
	var as []string
	for i, a := range args {
		if fn.Params[i].Mode == borrow.Ref {
			a = "&" + a
		}
		as = append(as, a)
	}
	recv := v // a borrow
	switch {
	case fn.Recv.Mode == borrow.Ref || fn.Recv.Mode == borrow.RefMut:
	case borrow.KindOf(t) == borrow.Copy:
		recv = "*" + v
	default:
		recv = v + ".clone()"
	}
	if g.freeMethods(g.recvTypeName(m)) {
		return g.qual(m.Pkg(), g.funcs[m]) + "(" + strings.Join(append([]string{recv}, as...), ", ") + ")"
	}
	if strings.HasPrefix(recv, "*") {
		recv = "(" + recv + ")"
	}
	return recv + "." + g.funcs[m] + "(" + strings.Join(as, ", ") + ")"
}

// show renders, as a String, %v of the value v, a reference to a t,
// stands for.
func (g *gen) show(t types.Type, v string) string {
	e, isString := g.dynDisplay(t, v, true)
	if isString {
		return e
	}
	return e + ".to_string()"
}

// dynDisplay renders an expression whose Display is %v of p, a reference to a
// t (ref) or a place of type t, and whether it is a String already.
func (g *gen) dynDisplay(t types.Type, p string, ref bool) (string, bool) {
	if m := subset.PrintMethod(t); m != nil {
		// Only a whole dynamic value: the checker rejects a struct with a
		// field fmt prints through a method.
		return g.methodOn(m, t, p, nil), true
	}
	val := p
	if ref {
		val = "*" + p
	}
	switch u := t.Underlying().(type) {
	case *types.Slice:
		g.ifc.slices = true
		return "iface_slice(" + p + ", |e| " + g.show(u.Elem(), "e") + ")", true
	case *types.Struct:
		if u.NumFields() == 0 {
			return "String::from(\"{}\")", true
		}
		var parts, holes []string
		for i := 0; i < u.NumFields(); i++ {
			f := u.Field(i)
			holes = append(holes, "{}")
			e, _ := g.dynDisplay(f.Type(), p+"."+g.fields[f], false)
			parts = append(parts, e)
		}
		return "format!(\"{{" + strings.Join(holes, " ") + "}}\", " + strings.Join(parts, ", ") + ")", true
	}
	switch b := t.Underlying().(*types.Basic); {
	case b.Kind() == types.Float32:
		g.rt["Float32"] = true
		return "rt::Float32(" + val + ")", false
	case b.Info()&types.IsFloat != 0:
		g.rt["Float"] = true
		return "rt::Float(" + val + ")", false
	}
	return p, false
}

// patArm writes the match arm pats => rest at indent, with the patterns on
// one line, or one per line as rustfmt writes them when they do not fit.
func (g *gen) patArm(indent string, pats []string, rest string) {
	one := indent + strings.Join(pats, " | ") + " => " + rest
	if len(one)+g.ind*indentWidth <= maxWidth {
		g.line(one)
		return
	}
	for i, p := range pats {
		switch {
		case i == 0:
			g.line(indent + p)
		case i == len(pats)-1:
			g.line(indent + "| " + p + " => " + rest)
		default:
			g.line(indent + "| " + p)
		}
	}
}

// noDyn writes the arm a match on Option<&Dyn> needs when the program
// converts no value to an interface, and Dyn has no variants.
func (g *gen) noDyn(indent string) {
	if len(g.ifc.plan.Dyn) == 0 && len(g.builtins()) == 0 {
		g.line(indent + "Some(d) => match *d {},")
	}
}
