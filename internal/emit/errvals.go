package emit

import (
	"go/ast"
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
	for _, v := range ec.Values {
		vals = append(vals, pattern(v))
		zeros = append(zeros, g.zero(g.typeOf(v)).flat())
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

// isGoErrorMethod reports whether m is Error() string, which Go's
// *errors.errorString has.
func isGoErrorMethod(m *types.Func) bool {
	sig := m.Signature()
	return m.Name() == "Error" && sig.Params().Len() == 0 && sig.Results().Len() == 1 &&
		types.Identical(sig.Results().At(0).Type(), types.Typ[types.String])
}

// goErrorMissing returns the first method of interface it that Go's
// *errors.errorString lacks, or "".
func goErrorMissing(it *types.Interface) string {
	for i := 0; i < it.NumMethods(); i++ { // sorted by name
		if !isGoErrorMethod(it.Method(i)) {
			return it.Method(i).Name()
		}
	}
	return ""
}

// isErrorType reports whether t is the error interface.
func isErrorType(t types.Type) bool {
	return types.Identical(t, types.Universe.Lookup("error").Type())
}
