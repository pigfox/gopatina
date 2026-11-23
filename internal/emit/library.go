package emit

import (
	"go/ast"
	"go/types"
	"sort"
	"strings"

	"github.com/pigfox/gopatina/internal/borrow"
)

// Library crates (RFC 0014): a package other than main is written as
// src/lib.rs, whose API is the package's exported items. A function whose
// error result is lowered to Result<T, Iface> keeps a private name (f_go)
// and gets a pub wrapper returning Result<T, Error>, Error being the
// crate's opaque Go error; an exported sentinel error is a pub function
// returning it. The crate's own machinery (Dyn, Iface, the helpers) is
// pub(crate), so the API holds only the package's types and Error.

// apiItems records, while the crate is written, what api.txt lists beyond
// the signatures: why a parameter is owned, by item name.
type apiItems struct {
	names    map[*types.Func]string // the pub wrapper of a lowered function
	sentinel map[*types.Var]string  // the pub function of a sentinel error
	notes    map[string][]string    // api.txt notes, by the item's Rust name
	errorAPI bool                   // the API mentions Error
}

// apiName returns the name of the pub wrapper of fn, or "" if fn is
// written as it is.
func (g *gen) apiName(fn *types.Func) string {
	if g.api == nil {
		return ""
	}
	return g.api.names[fn]
}

// inAPI reports whether fn is an exported function or method of the
// library's own package, on an exported type.
func (g *gen) inAPI(fn *types.Func) bool {
	if g.api == nil || fn.Pkg() != g.pkg.Types || !fn.Exported() {
		return false
	}
	if r := fn.Signature().Recv(); r != nil {
		t := r.Type()
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		return types.Unalias(t).(*types.Named).Obj().Exported()
	}
	return true
}

// apiWrapper writes the pub wrapper of the lowered function fn, whose
// internal version has the given rendered parameters and result type.
func (g *gen) apiWrapper(fd *ast.FuncDecl, fn *types.Func, params []string, ret string) {
	name := g.api.names[fn]
	g.api.errorAPI = true
	g.line("")
	g.docComment(fd.Doc)
	ret = strings.Replace(ret, ", "+g.root("Iface")+">", ", Error>", 1)
	var args []string
	recv := ""
	for _, p := range params {
		n := strings.TrimPrefix(strings.SplitN(p, ":", 2)[0], "mut ")
		if strings.HasSuffix(n, "self") {
			recv = "self."
			continue
		}
		args = append(args, n)
	}
	head := "pub fn " + name + "("
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
	var argNodes []node
	for _, a := range args {
		argNodes = append(argNodes, text(a))
	}
	inner := &call{head: recv + g.funcs[fn], open: "(", close: ")", args: argNodes}
	g.stmtLine("", &call{recv: inner, head: ".map_err", open: "(", close: ")", args: []node{text("Error")}}, "")
	g.ind--
	g.line("}")
}

// apiNotes records, for api.txt, each parameter of API function fn that
// the function takes ownership of, with the borrow checker's reason.
func (g *gen) apiNotes(fn *types.Func, rustName string) {
	f := g.res.Funcs[fn]
	why := map[string]string{}
	for _, x := range g.res.Fixes {
		why[x.Pos.String()] = x.Text
	}
	for _, p := range f.Params {
		if p.Mode != borrow.Owned || borrow.KindOf(p.Var.Type()) == borrow.Copy {
			continue
		}
		note := g.varName(p.Var) + ": owned (" + g.rustType(p.Var.Type()) + ")"
		if w := why[g.pkg.Fset.Position(p.Var.Pos()).String()]; w != "" {
			if _, reason, ok := strings.Cut(w, ": "); ok {
				w = reason
			}
			note += ", because " + w
		}
		g.api.notes[rustName] = append(g.api.notes[rustName], note)
	}
}

// libraryErrorItems writes Error, the API's Go error, when the API uses it.
func (g *gen) libraryErrorItems() {
	if g.api == nil || !g.api.errorAPI {
		return
	}
	g.helper("iface_is")
	g.helper("iface_unwrap")
	for _, l := range []string{
		"/// A Go error value returned by this crate. It prints as Go's Error()",
		"/// prints it, and compares as Go's == does.",
		"#[derive(Clone, PartialEq)]",
		"pub struct Error(Iface);",
		"",
		"impl Error {",
		"    /// errors.Is(self, target).",
		"    pub fn is(&self, target: &Error) -> bool {",
		"        iface_is(&self.0, &target.0)",
		"    }",
		"",
		"    /// errors.Unwrap(self): the error it wraps, if any.",
		"    pub fn unwrap(&self) -> Option<Error> {",
		"        iface_unwrap(&self.0).flatten().map(|e| Error(Some(e)))",
		"    }",
		"}",
		"",
		"impl std::fmt::Display for Error {",
		"    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {",
		"        f.write_str(&iface_show(&self.0))",
		"    }",
		"}",
		"",
		"impl std::fmt::Debug for Error {",
		"    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {",
		"        f.write_str(&iface_show(&self.0))",
		"    }",
		"}",
		"",
		"impl std::error::Error for Error {}",
	} {
		g.line(l)
	}
}

// sentinelFn writes the pub function returning sentinel error v.
func (g *gen) sentinelFn(v *types.Var) {
	name := g.api.sentinel[v]
	g.api.errorAPI = true
	g.line("")
	g.line("/// " + v.Name() + ", a sentinel error: the same value on every call, as in Go.")
	g.line("pub fn " + name + "() -> Error {")
	g.line("    Error(" + g.gnames[v] + ".with(Clone::clone))")
	g.line("}")
}

// apiText returns api.txt for a library crate: every pub item of lib.rs,
// one per entry, sorted, each followed by the notes on why a parameter is
// owned.
func apiText(crate, from, lib string, notes map[string][]string) string {
	var items []string
	lines := strings.Split(lib, "\n")
	impl := ""
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		switch {
		case strings.HasPrefix(l, "impl ") && strings.HasSuffix(l, " {"):
			impl = strings.TrimSuffix(strings.TrimPrefix(l, "impl "), " {")
			continue
		case l == "}":
			impl = ""
			continue
		}
		t := strings.TrimSpace(l)
		if !strings.HasPrefix(t, "pub fn ") && !strings.HasPrefix(t, "pub struct ") &&
			!strings.HasPrefix(t, "pub const ") && !strings.HasPrefix(t, "pub type ") {
			continue
		}
		// A signature or a struct over several lines: up to its body.
		sig := t
		for !strings.HasSuffix(sig, "{") && !strings.HasSuffix(sig, ";") && i+1 < len(lines) {
			i++
			sig += " " + strings.TrimSpace(lines[i])
		}
		sig = strings.TrimSuffix(strings.TrimSuffix(sig, " {"), ";")
		sig = strings.ReplaceAll(strings.ReplaceAll(sig, "(mut ", "("), ", mut ", ", ")
		sig = strings.ReplaceAll(strings.ReplaceAll(sig, "( ", "("), ", )", ")")
		if strings.HasPrefix(sig, "pub struct ") && strings.HasSuffix(t, "{") {
			var fields []string
			for i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "}" {
				i++
				if f := strings.TrimSpace(lines[i]); strings.HasPrefix(f, "pub ") {
					fields = append(fields, strings.TrimSuffix(f, ","))
				}
			}
			sig += " { " + strings.Join(fields, ", ") + " }"
		}
		if strings.HasPrefix(sig, "pub struct Error(") {
			sig = "pub struct Error" // opaque: a Go error value
		}
		key := sig
		if impl != "" {
			key = "impl " + impl + ": " + sig
		}
		name := sig
		if strings.HasPrefix(sig, "pub fn ") {
			name = strings.SplitN(strings.TrimPrefix(sig, "pub fn "), "(", 2)[0]
			if impl != "" {
				name = impl + "::" + name
			}
		}
		for _, n := range notes[name] {
			key += "\n    " + n
		}
		items = append(items, key)
	}
	sort.Strings(items)
	return "# The API of crate " + crate + ", generated by GoPatina from " + from + " (RFC 0014).\n" +
		"# Each pub item, sorted; under a function, each parameter it takes\n" +
		"# ownership of, and why (gopatina -explain).\n" +
		strings.Join(items, "\n") + "\n"
}

// debuggable reports whether Rust can derive Debug for a value of type t:
// numbers, strings, and slices and structs of them (not interface values or
// heap pointers, whose types are the crate's own).
func debuggable(t types.Type, seen map[types.Type]bool) bool {
	if seen[t] {
		return true
	}
	seen[t] = true
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return true
	case *types.Slice:
		return debuggable(u.Elem(), seen)
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if !debuggable(u.Field(i).Type(), seen) {
				return false
			}
		}
		return true
	}
	return false
}
