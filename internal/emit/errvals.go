package emit

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strings"

	"github.com/pigfox/gopatina/internal/subset"
)

// Errors as interface values (RFC 0012). error is an interface like any
// other; errors.New and fmt.Errorf make values of Go's *errors.errorString,
// which is the variant ErrorsErrorString holding an rt::Error, compared by
// identity as Go compares the pointers. A function returning an error is
// still lowered to Result<T, Iface>; where its error is kept as a value,
// the call is split into its values and the error (iface_split, iface_err),
// and a return of an error that may be nil becomes Ok or Err by its value
// (iface_result).

// goErrorVariant is the variant of errors.New's and fmt.Errorf's values.
const goErrorVariant = "ErrorsErrorString"

// goError renders errors.New(msg) or fmt.Errorf(...): an interface value.
func (g *gen) goError(msg node) node {
	g.ifc.goError = true
	g.rt["Error"] = true
	return &call{head: g.root("go_error"), open: "(", close: ")", args: []node{msg}}
}

// helper marks a crate-level error helper as used and returns its name.
func (g *gen) helper(name string) string {
	g.ifc.helpers[name] = true
	return g.root(name)
}

// errValue renders a value-context call of a function lowered to Result
// whose only result is an error: the error as an interface value.
func (g *gen) errValue(e *ast.CallExpr, n node) node {
	if fn := g.loweredCallee(e); fn != nil && fn.Signature().Results().Len() == 1 && !g.rawCalls[e] {
		return &call{head: g.helper("iface_err"), open: "(", close: ")", args: []node{n}}
	}
	return n
}

// loweredCallee returns the function e calls if it is lowered to Result.
func (g *gen) loweredCallee(e *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch f := ast.Unparen(e.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	default:
		return nil
	}
	fn, ok := g.info.Uses[id].(*types.Func)
	if !ok || !g.errs.Funcs[fn] {
		return nil
	}
	return fn
}

// rawCall renders a call of a function lowered to Result as the Result.
func (g *gen) rawCall(e *ast.CallExpr) node {
	g.rawCalls[e] = true
	defer delete(g.rawCalls, e)
	return g.value(e)
}

// generalCall writes v..., err := f(...) whose error is kept as an
// interface value: the values are the zero values when it is not nil.
func (g *gen) generalCall(ec *subset.ErrCall) {
	callN := g.rawCall(ec.Call)
	// := that also assigns to a variable declared before declares only the
	// new ones, then assigns them all, as Go does.
	define := ec.Define
	lhs := append(append([]ast.Expr{}, ec.Values...), ec.Err)
	if define {
		for _, l := range lhs {
			if id, ok := l.(*ast.Ident); ok && id.Name != "_" && g.info.Defs[id] == nil {
				define = false
			}
		}
		if !define {
			for _, l := range lhs {
				if id, ok := l.(*ast.Ident); ok && id.Name != "_" && g.info.Defs[id] != nil {
					v := g.localVar(id)
					g.line("let " + g.mutPrefix(v) + g.varName(v) + ": " + g.rustType(v.Type()) + ";")
				}
			}
		}
	}
	pattern := func(e ast.Expr) string {
		if id, ok := e.(*ast.Ident); ok && id.Name != "_" && !define {
			return g.varName(g.localVar(id))
		}
		return g.lhsPattern(e, define)
	}
	var vals, zeros []string
	for i, v := range ec.Values {
		// The callee's result type: _ has none of its own.
		t := g.info.TypeOf(ec.Call).(*types.Tuple).At(i).Type()
		vals = append(vals, pattern(v))
		zeros = append(zeros, g.zero(t).flat())
	}
	errPat := "_"
	if ec.Err != nil {
		errPat = pattern(ec.Err)
	}
	var pat string
	var rhs node
	if len(vals) == 0 {
		pat = errPat
		rhs = &call{head: g.helper("iface_err"), open: "(", close: ")", args: []node{callN}}
	} else {
		v := strings.Join(vals, ", ")
		z := strings.Join(zeros, ", ")
		if len(vals) > 1 {
			v, z = "("+v+")", "("+z+")"
		}
		pat = "(" + v + ", " + errPat + ")"
		rhs = &call{head: g.helper("iface_split"), open: "(", close: ")", args: []node{callN, text(z)}}
	}
	prefix := pat + " = "
	if define {
		prefix = "let " + prefix
	}
	g.stmtLine(prefix, rhs, ";")
}

// returnValue renders return zero..., err where err may be nil.
func (g *gen) errResult(r *ast.ReturnStmt) node {
	results := r.Results
	vals := g.ownedAll(results[:len(results)-1])
	if len(vals) == 0 {
		vals = []node{text("()")}
	}
	return &call{head: g.helper("iface_result"), open: "(", close: ")", args: []node{tuple(vals), g.owned(results[len(results)-1])}}
}

// errHelpers writes the error helpers the crate uses.
func (g *gen) errHelpers() {
	if g.ifc.goError {
		g.line("")
		g.line("/// errors.New: a new error with the message, equal only to itself.")
		g.line("pub fn go_error(message: impl Into<String>) -> Iface {")
		g.line("    boxed(Dyn::" + goErrorVariant + "(rt::Error::new(message)))")
		g.line("}")
	}
	if g.ifc.helpers["iface_panic"] {
		g.line("")
		g.line("/// panic(err): Go prints Error(), and for a nil error the text of its")
		g.line("/// runtime.PanicNilError.")
		g.line("pub fn iface_panic(x: &Iface) -> String {")
		g.line("    if x.is_none() {")
		g.line("        return String::from(\"runtime error: panic called with nil argument\");")
		g.line("    }")
		g.line("    iface_show(x)")
		g.line("}")
	}
	if g.ifc.helpers["iface_s"] {
		g.line("")
		g.line("/// %s of an error, padded to width: %!s(<nil>), unpadded, for nil.")
		g.line("pub fn iface_s(x: &Iface, width: usize, left: bool) -> String {")
		g.line("    if x.is_none() {")
		g.line("        return String::from(\"%!s(<nil>)\");")
		g.line("    }")
		g.line("    let s = iface_show(x);")
		g.line("    if left {")
		g.line("        format!(\"{s:<width$}\")")
		g.line("    } else {")
		g.line("        format!(\"{s:>width$}\")")
		g.line("    }")
		g.line("}")
	}
	if g.ifc.helpers["iface_err"] {
		g.line("")
		g.line("/// The error of a call that returns only an error.")
		g.line("pub fn iface_err(r: Result<(), Iface>) -> Iface {")
		g.line("    r.err().unwrap_or_default()")
		g.line("}")
	}
	if g.ifc.helpers["iface_split"] {
		g.line("")
		g.line("/// A call's values and its error: the zero values when it is not nil.")
		g.line("pub fn iface_split<T>(r: Result<T, Iface>, zero: T) -> (T, Iface) {")
		g.line("    match r {")
		g.line("        Ok(v) => (v, None),")
		g.line("        Err(e) => (zero, e),")
		g.line("    }")
		g.line("}")
	}
	if g.ifc.helpers["iface_result"] {
		g.line("")
		g.line("/// return v..., err: Ok when err is nil, else Err.")
		g.line("pub fn iface_result<T>(v: T, err: Iface) -> Result<T, Iface> {")
		g.line("    match err {")
		g.line("        None => Ok(v),")
		g.line("        e => Err(e),")
		g.line("    }")
		g.line("}")
	}
}

// A builtin is one of Go's own error types a program's values may have:
// *errors.errorString (errors.New, fmt.Errorf) and *fmt.wrapError
// (fmt.Errorf with %w). Each is a Dyn variant whose payload a reference v
// stands for, compared by identity, as Go compares the pointers.
type builtin struct {
	variant, payload, goName string
	methods                  []builtinMethod // Error first
}

// builtinMethod is a method of a builtin: its name, its result (string
// for Error, error for Unwrap), and the Rust that calls it on v.
type builtinMethod struct {
	name, result, call string
}

var (
	errorString = builtin{goErrorVariant, "rt::Error", "*errors.errorString", []builtinMethod{
		{"Error", "string", "v.to_string()"},
	}}
	wrapError = builtin{"FmtWrapError", "WrapError", "*fmt.wrapError", []builtinMethod{
		{"Error", "string", "v.msg.clone()"},
		{"Unwrap", "error", "v.err.clone()"},
	}}
)

// builtins returns the builtins the program uses, in a fixed order.
func (g *gen) builtins() []builtin {
	var bs []builtin
	if g.ifc.goError {
		bs = append(bs, errorString)
	}
	if g.ifc.wrap {
		bs = append(bs, wrapError)
	}
	return bs
}

// method returns the Rust that calls m on a builtin value, or "" if the
// builtin lacks m.
func (b builtin) method(m *types.Func) string {
	sig := m.Signature()
	for _, bm := range b.methods {
		if bm.name == m.Name() && sig.Params().Len() == 0 && sig.Results().Len() == 1 &&
			sig.Results().At(0).Type().String() == bm.result {
			return bm.call
		}
	}
	return ""
}

// missing returns the first method of interface it the builtin lacks, or
// "".
func (b builtin) missing(it *types.Interface) string {
	for i := 0; i < it.NumMethods(); i++ { // sorted by name
		if b.method(it.Method(i)) == "" {
			return it.Method(i).Name()
		}
	}
	return ""
}

// isErrorType reports whether t is the error interface.
func isErrorType(t types.Type) bool {
	return types.Identical(t, types.Universe.Lookup("error").Type())
}

// stripAddrExpr is x for &x, which the checker requires of errors.As.
func stripAddrExpr(e ast.Expr) ast.Expr {
	return ast.Unparen(e).(*ast.UnaryExpr).X
}

// wrapped returns the operand of fmt.Errorf's %w, or nil.
func (g *gen) wrapped(e *ast.CallExpr) ast.Expr {
	verbs, _ := subset.ParseFormat(constant.StringVal(g.info.Types[e.Args[0]].Value))
	n := 1
	for _, v := range verbs {
		if v.Verb == 0 {
			continue
		}
		if v.Verb == 'w' {
			return e.Args[n]
		}
		n++
	}
	return nil
}

// asHelper returns the helper of errors.As to a target of type t.
func (g *gen) asHelper(t types.Type) string {
	name := "iface_as_" + snake(variantName(t))
	if _, ok := g.ifc.ases[name]; !ok {
		g.ifc.ases[name] = t
		g.ifc.asKeys = append(g.ifc.asKeys, name)
	}
	g.ifc.helpers["iface_unwrap"] = true
	return g.root(name)
}

// asRefHelper names the helper of errors.As(err, &q) for q a borrow of
// an owned pointer (RFC 0017): it lends q the pointer err's chain holds,
// exclusively when mut.
func (g *gen) asRefHelper(t *types.Pointer, mut bool) string {
	name := "iface_as_ref_" + snake(variantName(t))
	if mut {
		name = "iface_as_mut_" + snake(variantName(t))
	}
	if g.ifc.asRefs == nil {
		g.ifc.asRefs = map[string]*types.Pointer{}
	}
	if _, ok := g.ifc.asRefs[name]; !ok {
		g.ifc.asRefs[name] = t
		g.ifc.asRefKeys = append(g.ifc.asRefKeys, name)
	}
	return g.root(name)
}

// asRefFns writes the asRefHelper helpers: each follows err's chain
// through %w's wrappers by reference, so the borrow lives as long as err's.
func (g *gen) asRefFns() {
	for _, name := range g.ifc.asRefKeys {
		t := g.ifc.asRefs[name]
		mut := strings.HasPrefix(name, "iface_as_mut_")
		ref, view, inner := "&", g.view(), ".as_deref()"
		if g.unboxed(t) {
			inner = ".as_ref()"
		}
		if mut {
			ref = "&mut "
			view = map[string]string{"as_deref": "as_deref_mut", "as_ref": "as_mut"}[view]
			inner = map[string]string{".as_deref()": ".as_deref_mut()", ".as_ref()": ".as_mut()"}[inner]
		}
		g.line("")
		g.line("/// errors.As to a " + goTypeName(t) + ": lends target the first one in err's chain.")
		g.line("pub fn " + name + "<'a>(err: &'a " + strings.TrimPrefix(ref, "&") + "Iface, target: &mut Option<&'a " + strings.TrimPrefix(ref, "&") + g.rustType(t.Elem()) + ">) -> bool {")
		g.line("    match err." + view + "() {")
		g.line("        Some(Dyn::" + g.variant(t) + "(v)) => {")
		g.line("            *target = v" + inner + ";")
		g.line("            true")
		g.line("        }")
		if g.ifc.wrap {
			g.line("        Some(Dyn::FmtWrapError(w)) => " + name + "(" + ref + "w.err, target),")
		}
		g.line("        _ => false,")
		g.line("    }")
		g.line("}")
	}
}

// methodsNamed returns the dynamic types with a value method named name
// whose signature ok accepts, and the methods.
func (g *gen) methodsNamed(name string, ok func(*types.Signature) bool) ([]int, []*types.Func) {
	var is []int
	var ms []*types.Func
	for i, t := range g.ifc.plan.Dyn {
		obj, _, _ := types.LookupFieldOrMethod(t, false, nil, name)
		if m, isFunc := obj.(*types.Func); isFunc && ok(m.Signature()) {
			is = append(is, i)
			ms = append(ms, m)
		}
	}
	return is, ms
}

func returnsOnly(t types.Type) func(*types.Signature) bool {
	return func(s *types.Signature) bool {
		return s.Params().Len() == 0 && s.Results().Len() == 1 && types.Identical(s.Results().At(0).Type(), t)
	}
}

// errorsAPI writes the helpers of errors.Is, As and Unwrap and of %w
// (RFC 0012), as Go's errors package defines them.
func (g *gen) errorsAPI() {
	errT := types.Universe.Lookup("error").Type()
	h := g.ifc.helpers
	if g.ifc.wrap {
		g.line("")
		g.line("/// Go's *fmt.wrapError: fmt.Errorf's value with one %w. Each one is a")
		g.line("/// new pointer in Go; id stands for that identity.")
		g.line("#[derive(Clone)]")
		g.line("pub struct WrapError {")
		g.line("    msg: String,")
		g.line("    err: Iface,")
		g.line("    id: u64,")
		g.line("}")
		g.line("")
		g.line("impl WrapError {")
		g.line("    fn id(&self) -> u64 {")
		g.line("        self.id")
		g.line("    }")
		g.line("}")
		g.line("")
		g.line("/// fmt.Errorf with %w: the message, wrapping err.")
		g.line("pub fn go_wrap(msg: String, err: Iface) -> Iface {")
		g.line("    let id = rt::error_id();")
		g.line("    boxed(Dyn::FmtWrapError(WrapError { msg, err, id }))")
		g.line("}")
	}
	if h["iface_w"] {
		g.line("")
		g.line("/// %w of an error: its text, or %!w(<nil>).")
		g.line("pub fn iface_w(x: &Iface) -> String {")
		g.line("    if x.is_none() {")
		g.line("        return String::from(\"%!w(<nil>)\");")
		g.line("    }")
		g.line("    iface_show(x)")
		g.line("}")
	}
	if h["iface_is"] {
		h["iface_unwrap"] = true
		var uncomparable []string
		for i, t := range g.ifc.plan.Dyn {
			if !types.Comparable(t) {
				uncomparable = append(uncomparable, "Dyn::"+g.ifc.variants[i]+"(_)")
			}
		}
		is, ms := g.methodsNamed("Is", func(s *types.Signature) bool {
			return s.Params().Len() == 1 && types.Identical(s.Params().At(0).Type(), errT) &&
				s.Results().Len() == 1 && types.Identical(s.Results().At(0).Type(), types.Typ[types.Bool])
		})
		g.line("")
		g.line("/// errors.Is: whether err or an error it wraps is target, or says it is.")
		g.line("pub fn iface_is(err: &Iface, target: &Iface) -> bool {")
		g.line("    if err.is_none() || target.is_none() {")
		g.line("        return err.is_none() && target.is_none();")
		g.line("    }")
		cmp := "err == *target"
		if len(uncomparable) > 0 {
			if one := "    let comparable = !matches!(target." + g.view() + "(), Some(" + strings.Join(uncomparable, " | ") + "));"; len(one) <= maxWidth {
				g.line(one)
			} else {
				g.line("    let comparable = !matches!(")
				g.line("        target." + g.view() + "(),")
				if some := "        Some(" + strings.Join(uncomparable, " | ") + ")"; len(some) <= maxWidth {
					g.line(some)
				} else {
					g.line("        Some(")
					for i, u := range uncomparable {
						if i == 0 {
							g.line("            " + u)
						} else {
							g.line("                | " + u)
						}
					}
					g.line("        )")
				}
				g.line("    );")
			}
			cmp = "comparable && err == *target"
		}
		g.line("    let mut err = err.clone();")
		g.line("    loop {")
		g.line("        if " + cmp + " {")
		g.line("            return true;")
		g.line("        }")
		if len(is) > 0 {
			g.line("        let says = match err." + g.view() + "() {")
			for j, i := range is {
				g.line("            Some(Dyn::" + g.ifc.variants[i] + "(v)) => " + g.methodOn(ms[j], g.ifc.plan.Dyn[i], "v", []string{"target.clone()"}) + ",")
			}
			g.line("            _ => false,")
			g.line("        };")
			g.line("        if says {")
			g.line("            return true;")
			g.line("        }")
		}
		g.line("        match iface_unwrap(&err) {")
		g.line("            Some(next) if next.is_some() => err = next,")
		g.line("            _ => return false,")
		g.line("        }")
		g.line("    }")
		g.line("}")
	}
	g.asRefFns()
	for _, name := range g.ifc.asKeys {
		t := g.ifc.ases[name]
		g.line("")
		g.line("/// errors.As to a " + goTypeName(t) + ": sets target to the first error in err's chain that is one.")
		g.line("pub fn " + name + "(err: &Iface, target: &mut " + g.rustType(t) + ") -> bool {")
		g.line("    let mut err = err.clone();")
		g.line("    loop {")
		switch v := g.variant(t); {
		case subset.IsIface(t):
			g.line("        if let Some(d) = err." + g.view() + "() {")
			g.line("            if " + g.missing(t) + "(d).is_none() {")
			g.line("                *target = err.clone();")
			g.line("                return true;")
			g.line("            }")
			g.line("        }")
		case v != "":
			g.line("        if let Some(Dyn::" + v + "(v)) = err." + g.view() + "() {")
			g.line("            *target = " + copyOut(t, "v") + ";")
			g.line("            return true;")
			g.line("        }")
		}
		g.line("        match iface_unwrap(&err) {")
		g.line("            Some(next) if next.is_some() => err = next,")
		g.line("            _ => return false,")
		g.line("        }")
		g.line("    }")
		g.line("}")
	}
	if h["iface_unwrap"] {
		is, ms := g.methodsNamed("Unwrap", returnsOnly(errT))
		g.line("")
		g.line("/// Unwrap() error of the dynamic value: the error it wraps, or None if it has")
		g.line("/// no such method.")
		g.line("pub fn iface_unwrap(x: &Iface) -> Option<Iface> {")
		g.line("    match x." + g.view() + "() {")
		if g.ifc.wrap {
			g.line("        Some(Dyn::FmtWrapError(v)) => Some(v.err.clone()),")
		}
		for j, i := range is {
			h["iface_err"] = true
			g.line("        Some(Dyn::" + g.ifc.variants[i] + "(v)) => Some(iface_err(" + g.methodOn(ms[j], g.ifc.plan.Dyn[i], "v", nil) + ")),")
		}
		g.line("        _ => None,")
		g.line("    }")
		g.line("}")
	}
}
