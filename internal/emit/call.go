package emit

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strconv"
	"strings"

	"github.com/pigfox/gopatina/internal/borrow"
	"github.com/pigfox/gopatina/internal/load"
	"github.com/pigfox/gopatina/internal/subset"
)

// userPanicMarker prefixes every panic that comes from a Go panic(...) call.
// The runtime hook strips it and prints the payload verbatim; Rust's own
// runtime panics do not carry it and can be translated to Go's wording.
const userPanicMarker = "gopatina-user-panic:"

// call renders a call expression as a value.
func (g *gen) call(e *ast.CallExpr) node {
	fun := ast.Unparen(e.Fun)
	if tv := g.info.Types[fun]; tv.IsType() {
		return g.conversion(e, tv.Type)
	}
	switch f := fun.(type) {
	case *ast.Ident:
		if b, ok := g.info.Uses[f].(*types.Builtin); ok {
			return g.builtin(e, b.Name())
		}
		fn := g.res.Funcs[g.info.Uses[f].(*types.Func)]
		return &call{head: g.funcs[fn.Obj], open: "(", close: ")", args: g.args(e, fn)}
	}
	f := fun.(*ast.SelectorExpr)
	{
		if id, ok := f.X.(*ast.Ident); ok {
			if pn, ok := g.info.Uses[id].(*types.PkgName); ok {
				if obj, ok := g.info.Uses[f.Sel].(*types.Func); ok && g.pkg.Local[pn.Imported()] {
					// A function of another package of the program (RFC 0009).
					if load.IsIntrinsic(obj) {
						return g.intrinsic(e, obj.Name())
					}
					fn := g.res.Funcs[obj]
					return &call{head: g.qual(obj.Pkg(), g.funcs[obj]), open: "(", close: ")", args: g.args(e, fn)}
				}
				return g.std(e, pn.Imported().Path()+"."+f.Sel.Name)
			}
		}
		obj := g.info.Selections[f].Obj().(*types.Func)
		fn := g.res.Funcs[obj]
		if helper, _ := subset.ProvidedMethod(obj); helper != "" {
			var args []node
			for _, a := range e.Args {
				args = append(args, g.value(a))
			}
			return g.methodCall(f.X, obj, args)
		}
		if fn == nil { // err.Error()
			return &call{recv: g.place(f.X), head: ".to_string", open: "(", close: ")"}
		}
		return g.methodCall(f.X, obj, g.args(e, fn))
	}
}

// methodCall renders a call of method m on receiver x with the rendered
// arguments: x.m(args) for a struct type, m(x, args) for the functions of
// other named types (RFC 0007).
func (g *gen) methodCall(x ast.Expr, m *types.Func, args []node) node {
	if helper, _ := subset.ProvidedMethod(m); helper != "" {
		g.rt[helper] = true
		return &call{head: "rt::" + helper, open: "(", close: ")", args: append([]node{g.value(x)}, args...)}
	}
	fn := g.res.Funcs[m]
	if g.freeMethods(g.recvTypeName(m)) {
		return &call{head: g.qual(m.Pkg(), g.funcs[m]), open: "(", close: ")", args: append([]node{g.recvArg(x, fn.Recv)}, args...)}
	}
	return &call{recv: g.receiver(x, fn.Recv), head: "." + g.funcs[m], open: "(", close: ")", args: args}
}

// recvArg renders receiver x as the first argument of a method function,
// in the receiver's mode. Go takes &x for a pointer receiver and *x for a
// value receiver called through a pointer, implicitly; Rust needs both
// spelled out.
func (g *gen) recvArg(x ast.Expr, p *borrow.Param) node {
	x = ast.Unparen(x)
	if u, ok := x.(*ast.UnaryExpr); ok && !isPointerType(p.Var.Type()) {
		x = u.X // (&v).M() with a value receiver
	}
	if isPointerType(g.typeOf(x)) && !isPointerType(p.Var.Type()) {
		switch p.Mode {
		case borrow.Ref, borrow.RefMut:
			return g.value(x) // the pointer is the borrow
		case borrow.Owned:
			return &call{recv: atom{"(*" + g.value(x).flat() + ")", precAtom}, head: ".clone", open: "(", close: ")"}
		}
		return atom{"*" + paren(g.value(x), precUnary), precUnary}
	}
	return g.arg(x, p)
}

// receiver renders the receiver of a method call.
func (g *gen) receiver(x ast.Expr, p *borrow.Param) node {
	x = ast.Unparen(x)
	if u, ok := x.(*ast.UnaryExpr); ok {
		x = u.X // (&v).M()
	}
	if s, ok := x.(*ast.StarExpr); ok {
		x = s.X // (*p).M()
	}
	if p.Mode == borrow.Owned {
		return g.owned(x)
	}
	if g.isPlace(x) {
		return g.place(x)
	}
	return g.value(x)
}

// args renders the arguments of a call to a function in the package.
func (g *gen) args(e *ast.CallExpr, fn *borrow.Func) []node {
	if s := g.res.Suffixes[e]; s != nil {
		g.rt["slice_suffix"] = true
		low := s.Low
		var index node
		if tv := g.info.Types[low]; tv.Value != nil {
			index = g.constant(tv.Value, types.Typ[types.Int])
		} else {
			index = g.value(low)
		}
		return []node{&call{head: "rt::slice_suffix", open: "(", close: ")", args: []node{g.ref(s.X), index}}}
	}
	var out []node
	for i, a := range e.Args {
		out = append(out, g.arg(a, fn.Params[i]))
	}
	return out
}

func (g *gen) arg(a ast.Expr, p *borrow.Param) node {
	if name, ok := g.hoisted[a]; ok {
		return text(name)
	}
	if c, ok := ast.Unparen(a).(*ast.CallExpr); ok && g.callTemps[c] != "" {
		// A call computed into a local first (RFC 0010's order).
		switch p.Mode {
		case borrow.Ref:
			if borrow.KindOf(g.typeOf(a)) != borrow.Copy {
				return text("&" + g.callTemps[c])
			}
		case borrow.RefMut:
			return text("&mut " + g.callTemps[c])
		}
		return text(g.callTemps[c])
	}
	switch p.Mode {
	case borrow.Ref:
		if isStringType(g.typeOf(a)) {
			if tv := g.info.Types[a]; tv.Value != nil {
				return g.constant(tv.Value, tv.Type)
			}
		}
		return g.ref(a)
	case borrow.RefMut:
		return g.mutRef(a)
	case borrow.Owned:
		return g.owned(a)
	}
	return g.value(a)
}

// builtin renders a call of a built-in function used as a value.
func (g *gen) builtin(e *ast.CallExpr, name string) node {
	switch name {
	case "len":
		return atom{(&call{recv: g.strRef(e.Args[0]), head: ".len", open: "(", close: ")"}).flat() + " as i64", precCast}
	case "min", "max":
		// Go's min/max take two or more arguments; Rust's take two, so
		// fold from the left. Naming the type keeps an all-constant call
		// from defaulting to Rust's i32 rather than Go's int (i64).
		t := g.rustType(g.typeOf(e))
		out := g.value(e.Args[0])
		for _, a := range e.Args[1:] {
			out = &call{
				head:  "std::cmp::" + name + "::<" + t + ">",
				open:  "(",
				close: ")",
				args:  []node{out, g.value(a)},
			}
		}
		return out
	case "make":
		return g.makeSlice(e)
	}
	// panic(s). The marker lets the runtime hook tell a Go panic apart from
	// Rust's own runtime messages, which it translates to Go's text.
	return g.format("panic!", nil, userPanicMarker+"{}", []ast.Expr{e.Args[0]})
}

// identityConversion returns x when e is a conversion T(x) whose Rust type
// is x's own, such as string(n) for a named string type n (RFC 0007).
func (g *gen) identityConversion(e ast.Expr) (ast.Expr, bool) {
	c, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || !g.info.Types[c.Fun].IsType() {
		return nil, false
	}
	x := c.Args[0]
	if isSlice(g.typeOf(e)) && isStringType(g.typeOf(x)) {
		return nil, false // []byte(s)
	}
	return x, g.rustType(g.typeOf(e)) == g.rustType(g.typeOf(x))
}

// conversion renders T(x).
func (g *gen) conversion(e *ast.CallExpr, to types.Type) node {
	x := e.Args[0]
	from := g.typeOf(x)
	if isSlice(to) && isStringType(from) { // []byte(s)
		if tv := g.info.Types[x]; tv.Value != nil {
			s := constant.StringVal(tv.Value)
			if isASCII(s) {
				return &call{recv: text("b" + strLit(s)), head: ".to_vec", open: "(", close: ")"}
			}
		}
		return &call{recv: &call{recv: g.strRef(x), head: ".as_bytes", open: "(", close: ")"}, head: ".to_vec", open: "(", close: ")"}
	}
	if g.rustType(to) == g.rustType(from) {
		return g.value(x)
	}
	if isFloat(from) && isInt(to) {
		// Go's compiler, not Rust's saturating as, decides an out-of-range
		// conversion (RFC 0008, measured against gc on amd64): types
		// narrower than 32 bits go through int32, uint32 through int64,
		// uint64 by its own rule; a float32 as its exact float64 widening.
		r := g.rustType(to)
		helper := map[string]string{"i64": "f64_to_i64", "u32": "f64_to_i64", "u64": "f64_to_u64"}[r]
		if helper == "" {
			helper = "f64_to_i32"
		}
		g.rt[helper] = true
		arg := g.value(x)
		if g.rustType(from) == "f32" {
			arg = atom{paren(arg, precCast) + " as f64", precCast}
		}
		n := node(&call{head: "rt::" + helper, open: "(", close: ")", args: []node{arg}})
		if r != map[string]string{"f64_to_i64": "i64", "f64_to_i32": "i32", "f64_to_u64": "u64"}[helper] {
			n = atom{n.flat() + " as " + r, precCast}
		}
		return n
	}
	return atom{paren(g.value(x), precCast) + " as " + g.rustType(to), precCast}
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] >= 0x7f {
			return false
		}
	}
	return true
}

// std renders a call of a standard-library function.
func (g *gen) std(e *ast.CallExpr, name string) node {
	a := e.Args
	method := func(recv node, m string, args ...node) node {
		return &call{recv: recv, head: "." + m, open: "(", close: ")", args: args}
	}
	switch name {
	case "strings.Repeat":
		return method(g.strRef(a[0]), "repeat", text(g.index(a[1])))
	case "strings.TrimSpace":
		g.rt["trim_space"] = true
		return &call{head: "rt::trim_space", open: "(", close: ")", args: []node{g.strArg(a[0])}}
	case "strings.Contains":
		return method(g.strRef(a[0]), "contains", g.strArg(a[1]))
	case "strings.HasPrefix":
		return method(g.strRef(a[0]), "starts_with", g.strArg(a[1]))
	case "strings.HasSuffix":
		return method(g.strRef(a[0]), "ends_with", g.strArg(a[1]))
	case "strings.Index":
		return method(method(g.strRef(a[0]), "find", g.strArg(a[1])), "map_or", text("-1"), text("|i| i as i64"))
	case "strings.ToLower", "strings.ToUpper", "strings.EqualFold", "strings.TrimPrefix",
		"strings.TrimSuffix", "strings.Split", "strings.Cut":
		helper := map[string]string{
			"strings.ToLower": "to_lower", "strings.ToUpper": "to_upper", "strings.EqualFold": "equal_fold",
			"strings.TrimPrefix": "trim_prefix", "strings.TrimSuffix": "trim_suffix",
			"strings.Split": "split", "strings.Cut": "cut",
		}[name]
		g.rt[helper] = true
		var args []node
		for _, x := range a {
			args = append(args, g.strArg(x))
		}
		return &call{head: "rt::" + helper, open: "(", close: ")", args: args}
	case "strings.IndexByte":
		g.rt["index_byte"] = true
		return &call{head: "rt::index_byte", open: "(", close: ")", args: []node{g.strArg(a[0]), g.value(a[1])}}
	case "strings.ReplaceAll":
		return method(g.strRef(a[0]), "replace", g.strArg(a[1]), g.strArg(a[2]))
	case "strings.Join":
		return method(g.sliceRef(a[0]), "join", g.strArg(a[1]))
	case "errors.New":
		g.rt["Error"] = true
		return &call{head: "rt::Error::new", open: "(", close: ")", args: []node{g.strArg(a[0])}}
	case "fmt.Errorf":
		g.rt["Error"] = true
		return &call{head: "rt::Error::new", open: "(", close: ")", args: []node{g.fmtCall(e, name)}}
	case "os.Exit":
		code := text(g.index(a[0]))
		if tv := g.info.Types[a[0]]; tv.Value != nil {
			code = text(tv.Value.ExactString())
		} else {
			code = atom{paren(g.value(a[0]), precCast) + " as i32", precCast}
		}
		return &call{head: "std::process::exit", open: "(", close: ")", args: []node{code}}
	case "slices.Clone", "bytes.Clone":
		return method(g.sliceRef(a[0]), "to_vec")
	}
	return g.fmtCall(e, name)
}

// strArg renders a string argument passed by reference.
func (g *gen) strArg(e ast.Expr) node {
	if tv := g.info.Types[e]; tv.Value != nil {
		return g.constant(tv.Value, tv.Type)
	}
	return g.ref(e)
}

// sliceRef renders a slice as something that derefs to [T].
func (g *gen) sliceRef(e ast.Expr) node {
	if g.isPlace(e) {
		return g.place(e)
	}
	return g.value(e)
}

// fmtCall renders a call of a fmt printing function.
func (g *gen) fmtCall(e *ast.CallExpr, name string) node {
	args := e.Args
	macro := map[string]string{
		"fmt.Print": "print!", "fmt.Println": "println!", "fmt.Printf": "print!",
		"fmt.Sprint": "format!", "fmt.Sprintln": "format!", "fmt.Sprintf": "format!",
		"fmt.Errorf": "format!",
		"fmt.Fprint": "write!", "fmt.Fprintln": "writeln!", "fmt.Fprintf": "write!",
	}[name]
	var writer ast.Expr
	if strings.HasPrefix(name, "fmt.F") {
		writer, args = args[0], args[1:]
		if sel, ok := ast.Unparen(writer).(*ast.SelectorExpr); ok && g.info.Selections[sel] == nil {
			// fmt.Fprint(os.Stdout, ...) is fmt.Print(...).
			macro = map[string]string{"write!": "print!", "writeln!": "println!"}[macro]
			if sel.Sel.Name == "Stderr" {
				macro = "e" + macro
			}
			writer = nil
		}
	}
	var format string
	var operands []ast.Expr
	switch {
	case strings.HasSuffix(name, "f"):
		format = constant.StringVal(g.info.Types[args[0]].Value)
		operands = args[1:]
		verbs, _ := subset.ParseFormat(format)
		if g.printsBytes(verbs, operands) {
			return g.writeBytes(writer, verbs, operands, macro)
		}
		return g.formatVerbs(macro, writer, verbs, operands)
	case strings.HasSuffix(name, "ln"):
		operands = args
		var f strings.Builder
		for i := range args {
			if i > 0 {
				f.WriteString(" ")
			}
			f.WriteString("%v")
		}
		format = f.String()
		if macro == "format!" {
			format += "\n"
		}
	default: // Print, Sprint, Fprint: spaces only between operands that are not strings
		operands = args
		var f strings.Builder
		for i, a := range args {
			if i > 0 && !isStringType(g.typeOf(a)) && !isStringType(g.typeOf(args[i-1])) {
				f.WriteString(" ")
			}
			f.WriteString("%v")
		}
		format = f.String()
	}
	verbs, _ := subset.ParseFormat(format)
	return g.formatVerbs(macro, writer, verbs, operands)
}

// format renders a format macro call with simple {} arguments.
func (g *gen) format(macro string, writer ast.Expr, f string, operands []ast.Expr) node {
	verbs, _ := subset.ParseFormat(strings.ReplaceAll(f, "{}", "%v"))
	return g.formatVerbs(macro, writer, verbs, operands)
}

// formatVerbs renders a format macro call for parsed verbs.
func (g *gen) formatVerbs(macro string, writer ast.Expr, verbs []subset.Verb, operands []ast.Expr) node {
	var f strings.Builder
	var args []node
	i := 0
	for _, v := range verbs {
		if v.Verb == 0 {
			f.WriteString(escapeFormat(v.Text))
			continue
		}
		op := operands[i]
		i++
		spec, arg, lit := g.verb(v, op)
		if arg == nil {
			f.WriteString(escapeFormat(lit))
			continue
		}
		f.WriteString(spec)
		args = append(args, arg)
	}
	// A trailing newline in print!/eprint! is spelled as println!/eprintln!
	// to keep clippy clean. The format string is built first so we can test
	// and strip the final newline character.
	if (macro == "print!" || macro == "eprint!") && strings.HasSuffix(f.String(), "\n") {
		switch macro {
		case "print!":
			macro = "println!"
		case "eprint!":
			macro = "eprintln!"
		}
		s := f.String()
		f.Reset()
		f.WriteString(strings.TrimSuffix(s, "\n"))
	}
	var all []node
	if writer != nil {
		all = append(all, g.mutRef(writer))
	}
	if macro == "format!" || f.Len() > 0 || len(args) > 0 {
		all = append(all, text(strLit(f.String())))
	} else if macro == "println!" || macro == "eprintln!" {
		// keep the bare macro form println!()/eprintln!() rather than
		// println!("")/eprintln!(""), which clippy flags as empty strings.
	}
	return &call{head: macro, open: "(", close: ")", args: append(all, args...)}
}

// verb renders one verb applied to operand e: the Rust format spec and the
// argument, or, for a constant that needs no formatting, the literal text
// Go prints.
func (g *gen) verb(v subset.Verb, e ast.Expr) (spec string, arg node, lit string) {
	t := g.typeOf(e)
	tv := g.info.Types[e]
	plain := v.Width < 0 && v.Prec < 0 && !v.Plus && !v.Minus && !v.Zero
	if m := subset.PrintMethod(t); m != nil && (v.Verb == 'v' || v.Verb == 's') {
		// fmt prints the result of Error or String (RFC 0007); the subset
		// checker has made sure the method cannot panic.
		var b strings.Builder
		b.WriteString("{")
		if v.Width >= 0 {
			b.WriteString(":")
			if v.Minus {
				b.WriteString("<")
			} else {
				b.WriteString(">")
			}
			b.WriteString(itoa(v.Width))
		}
		if v.Prec >= 0 {
			if v.Width < 0 {
				b.WriteString(":")
			}
			b.WriteString("." + itoa(v.Prec))
		}
		b.WriteString("}")
		return b.String(), g.methodCall(e, m, nil), ""
	}
	if m := subset.PrintMethod(t); m != nil && (v.Verb == 'x' || v.Verb == 'X') {
		// The method's result in hex.
		g.rt["hex_bytes"] = true
		bytes := &call{recv: atom{paren(g.methodCall(e, m, nil), precPostfix), precPostfix}, head: ".as_bytes", open: "(", close: ")"}
		return hexPadSpec(v), &call{head: "rt::hex_bytes", open: "(", close: ")", args: []node{bytes, text(boolLit(v.Verb == 'X')), text(boolLit(v.Sharp))}}, ""
	}
	if tv.Value != nil && plain && (v.Verb == 'v' || v.Verb == 'd' || v.Verb == 's' || v.Verb == 't') {
		return "", nil, goConstText(tv.Value, t)
	}
	var b strings.Builder
	b.WriteString("{")
	flags := func(numeric bool) {
		if v.Width < 0 && v.Prec < 0 && !v.Plus && !v.Zero {
			return
		}
		b.WriteString(":")
		switch {
		case v.Minus:
			b.WriteString("<")
		case v.Width >= 0 && !numeric:
			// Rust pads strings on the right by default; Go on the left.
			b.WriteString(">")
		}
		if v.Plus {
			b.WriteString("+")
		}
		if v.Zero && !v.Minus {
			b.WriteString("0")
		}
		if v.Width >= 0 {
			b.WriteString(itoa(v.Width))
		}
		if v.Prec >= 0 {
			b.WriteString("." + itoa(v.Prec))
		}
	}
	switch {
	case (v.Verb == 'x' || v.Verb == 'X') && (isStringType(t) || isSlice(t)):
		// The bytes of a string or []byte in hex.
		g.rt["hex_bytes"] = true
		var bytes node
		if isStringType(t) {
			bytes = &call{recv: atom{paren(g.strRef(e), precPostfix), precPostfix}, head: ".as_bytes", open: "(", close: ")"}
		} else {
			bytes = g.ref(e)
		}
		arg = &call{head: "rt::hex_bytes", open: "(", close: ")", args: []node{bytes, text(boolLit(v.Verb == 'X')), text(boolLit(v.Sharp))}}
		return hexPadSpec(v), arg, ""
	case (v.Verb == 'x' || v.Verb == 'X') && (v.Zero || v.Sharp):
		// Go pads the digits with zeros to the width (the sign counts, a 0x
		// prefix does not); then only a space-padded width is left.
		g.rt["hex_int"] = true
		zw := 0
		spec := "{}"
		if v.Zero && !v.Minus && v.Width >= 0 {
			zw = v.Width
		} else {
			flags(false)
			b.WriteString("}")
			spec = b.String()
		}
		x := atom{paren(g.value(e), precCast) + " as i128", precCast}
		return spec, &call{head: "rt::hex_int", open: "(", close: ")", args: []node{x, text(boolLit(v.Verb == 'X')), text(boolLit(v.Sharp)), text(itoa(zw))}}, ""
	case v.Verb == 'x' || v.Verb == 'X':
		if isUnsigned(t) {
			flags(true)
			if !strings.Contains(b.String(), ":") {
				b.WriteString(":")
			}
			b.WriteString(string(v.Verb))
			arg = g.value(e)
			break
		}
		g.rt["hex"] = true
		flags(false)
		arg = &call{head: "rt::hex", open: "(", close: ")", args: []node{g.asI64(e), text(boolLit(v.Verb == 'X'))}}
	case v.Verb == 'c':
		g.rt["rune"] = true
		flags(false)
		arg = &call{head: "rt::rune", open: "(", close: ")", args: []node{g.asI64(e)}}
	case isFloat(t) && g32(t):
		g.rt["Float32"] = true
		if v.Verb == 'f' || v.Verb == 'F' {
			if v.Prec < 0 {
				v.Prec = 6
			}
		}
		flags(false)
		arg = &call{head: "rt::Float32", open: "(", close: ")", args: []node{g.value(e)}}
	case isFloat(t):
		g.rt["Float"] = true
		if v.Verb == 'f' || v.Verb == 'F' {
			if v.Prec < 0 {
				v.Prec = 6
			}
		}
		flags(false)
		arg = &call{head: "rt::Float", open: "(", close: ")", args: []node{g.value(e)}}
	case isSlice(t) && subset.PrintMethod(t.Underlying().(*types.Slice).Elem()) != nil:
		// Each element through its String or Error method.
		g.rt["SliceMap"] = true
		flags(false)
		m := subset.PrintMethod(t.Underlying().(*types.Slice).Elem())
		arg = &call{head: "rt::SliceMap", open: "(", close: ")", args: []node{g.ref(e), text(g.elemPrinter(m))}}
	case isSlice(t):
		g.rt["Slice"] = true
		switch g.rustType(t.Underlying().(*types.Slice).Elem()) {
		case "i8", "i16", "u16", "u32", "u64":
			g.rt["show_ints"] = true
		case "f32":
			g.rt["show_f32"] = true
		}
		flags(false)
		x := g.ref(e)
		if lit, ok := ast.Unparen(e).(*ast.CompositeLit); ok && len(lit.Elts) == 0 {
			x = text("&Vec::<" + g.rustType(t.Underlying().(*types.Slice).Elem()) + ">::new()")
		}
		arg = &call{head: "rt::Slice", open: "(", close: ")", args: []node{x}}
	case isInt(t):
		flags(true)
		arg = g.value(e)
	default: // strings, booleans, errors
		flags(false)
		arg = g.formatDisplay(e)
	}
	b.WriteString("}")
	return b.String(), arg, ""
}

// hexPadSpec is the format spec of a hex string from rt::hex_bytes: Go pads
// it with zeros for the 0 flag, unless - is given, and with spaces
// otherwise.
func hexPadSpec(v subset.Verb) string {
	switch {
	case v.Width < 0:
		return "{}"
	case v.Minus:
		return "{:<" + itoa(v.Width) + "}"
	case v.Zero:
		return "{:0>" + itoa(v.Width) + "}"
	}
	return "{:>" + itoa(v.Width) + "}"
}

// elemPrinter renders a closure that prints one element (a &T) through
// print method m: |e| m(e) for a borrowed receiver, m(*e) for a copied
// one, e.m() for a struct's method. The receiver is never owned: the subset
// prints only through a method that cannot panic, and that rule admits
// nothing that moves the receiver (were it owned, the crate would not
// compile, never print something else).
func (g *gen) elemPrinter(m *types.Func) string {
	if helper, _ := subset.ProvidedMethod(m); helper != "" {
		g.rt[helper] = true
		return "|e| rt::" + helper + "(*e)"
	}
	switch {
	case !g.freeMethods(g.recvTypeName(m)):
		return "|e| e." + g.funcs[m] + "()"
	case g.res.Funcs[m].Recv.Mode == borrow.Ref:
		return "|e| " + g.qual(m.Pkg(), g.funcs[m]) + "(e)"
	}
	return "|e| " + g.qual(m.Pkg(), g.funcs[m]) + "(*e)"
}

// g32 reports whether t is float32.
func g32(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Kind() == types.Float32
}

func boolLit(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// asI64 renders an integer as an i64.
func (g *gen) asI64(e ast.Expr) node {
	if g.rustType(g.typeOf(e)) == "i64" {
		return g.value(e)
	}
	if tv := g.info.Types[e]; tv.Value != nil {
		return text(tv.Value.ExactString())
	}
	return atom{paren(g.value(e), precCast) + " as i64", precCast}
}

// formatDisplay renders a string, boolean or error operand of a format macro.
// rt::Error implements Display with exactly the text Go's Error() returns, so
// passing the error itself avoids a redundant to_string() in the arguments.
func (g *gen) formatDisplay(e ast.Expr) node {
	if call, ok := ast.Unparen(e).(*ast.CallExpr); ok {
		if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok && sel.Sel.Name == "Error" {
			if obj, ok := g.info.Selections[sel].Obj().(*types.Func); ok && g.res.Funcs[obj] == nil {
				return g.place(sel.X)
			}
		}
	}
	return g.display(e)
}

// display renders a string, boolean or error operand of a format macro.
func (g *gen) display(e ast.Expr) node {
	if tv := g.info.Types[e]; tv.Value != nil {
		return g.constant(tv.Value, tv.Type)
	}
	if g.isPlace(e) {
		return g.place(e)
	}
	return g.value(e)
}

// goConstText is the text Go prints for constant v of type t with %v.
func goConstText(v constant.Value, t types.Type) string {
	switch {
	case isStringType(t):
		return constant.StringVal(v)
	case isFloat(t) && g32(t):
		f, _ := constant.Float32Val(constant.ToFloat(v))
		return strconv.FormatFloat(float64(f), 'g', -1, 32)
	case isFloat(t):
		f, _ := constant.Float64Val(constant.ToFloat(v))
		return goFloat(f)
	case isBool(t):
		return boolLit(constant.BoolVal(v))
	}
	return constant.ToInt(v).ExactString()
}

// printsBytes reports whether a []byte is printed with %s.
func (g *gen) printsBytes(verbs []subset.Verb, operands []ast.Expr) bool {
	i := 0
	for _, v := range verbs {
		if v.Verb == 0 {
			continue
		}
		if v.Verb == 's' && isByteSliceType(g.typeOf(operands[i])) {
			return true
		}
		i++
	}
	return false
}

// isByteSliceType reports whether t is a []byte, or a named type of one.
func isByteSliceType(t types.Type) bool {
	s, ok := t.Underlying().(*types.Slice)
	return ok && types.Identical(s.Elem(), types.Typ[types.Uint8])
}

// writeBytes renders a Printf that prints a []byte with %s: Go prints its
// bytes as they are, so they are written as bytes, not through a str.
func (g *gen) writeBytes(writer ast.Expr, verbs []subset.Verb, operands []ast.Expr, macro string) node {
	g.rt["write_bytes"] = true
	var parts []node
	i := 0
	for _, v := range verbs {
		if v.Verb == 0 {
			parts = append(parts, &call{recv: text(strLit(v.Text)), head: ".as_bytes", open: "(", close: ")"})
			continue
		}
		op := operands[i]
		i++
		if v.Verb == 's' && isSlice(g.typeOf(op)) {
			parts = append(parts, g.ref(op))
			continue
		}
		f := g.formatVerbs("format!", nil, []subset.Verb{v}, []ast.Expr{op})
		parts = append(parts, &call{recv: f, head: ".as_bytes", open: "(", close: ")"})
	}
	w := text("&mut std::io::stdout()")
	switch {
	case writer != nil:
		w = g.mutRef(writer)
	case strings.HasPrefix(macro, "e"):
		w = text("&mut std::io::stderr()")
	}
	return &call{head: "rt::write_bytes", open: "(", close: ")", args: []node{w, &call{head: "&", open: "[", close: "]", args: parts, array: true}}}
}

// goFloat is the text Go's fmt prints for a float64 with %v.
func goFloat(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

// makeSlice renders make([]T, len) and make([]T, len, cap). Constant
// arguments were checked by the Go compiler. Otherwise Go's runtime checks
// them (makeslice) and panics with its own message when one is negative or
// too large, or len > cap, so the length goes through rt::make_len,
// rt::make_len_cap or rt::make_cap, which do the same checks in the same
// order, with the element size Go uses.
func (g *gen) makeSlice(e *ast.CallExpr) node {
	elem := g.info.Types[e.Args[0]].Type.Underlying().(*types.Slice).Elem()
	size := text(strconv.FormatInt(g.pkg.Sizes.Sizeof(elem), 10))
	isConst := func(x ast.Expr) bool { return g.info.Types[x].Value != nil }
	vec := func(n string) node {
		return &call{head: "vec!", open: "[", close: "]", args: []node{atom{g.typedZero(elem).flat() + "; " + n, precAtom}}}
	}
	helper := func(name string, args ...ast.Expr) node {
		g.rt[name] = true
		var as []node
		for _, a := range args {
			as = append(as, g.int64Arg(a))
		}
		return &call{head: "rt::" + name, open: "(", close: ")", args: append(as, size)}
	}
	if len(e.Args) == 3 {
		if v := g.info.Types[e.Args[1]].Value; v != nil && constant.Sign(v) == 0 {
			if isConst(e.Args[2]) {
				return &call{head: "Vec::with_capacity", open: "(", close: ")", args: []node{text(g.index(e.Args[2]))}}
			}
			return &call{head: "Vec::with_capacity", open: "(", close: ")", args: []node{helper("make_cap", e.Args[2])}}
		}
		if !isConst(e.Args[1]) || !isConst(e.Args[2]) {
			return vec(helper("make_len_cap", e.Args[1], e.Args[2]).flat())
		}
	} else if !isConst(e.Args[1]) {
		return vec(helper("make_len", e.Args[1]).flat())
	}
	return vec(g.index(e.Args[1]))
}

// int64Arg renders an integer argument as an i64.
func (g *gen) int64Arg(x ast.Expr) node {
	if tv := g.info.Types[x]; tv.Value != nil {
		return g.constant(tv.Value, types.Typ[types.Int])
	}
	v := g.value(x)
	if g.rustType(g.typeOf(x)) == "i64" {
		return v
	}
	return atom{paren(v, precCast) + " as i64", precCast}
}
