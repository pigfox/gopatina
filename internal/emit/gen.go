// Package emit is GoPatina's code generator (stage 4). It translates a Go
// package that is inside the patina subset, with the borrow checker's
// decisions, into a readable Rust 2024 cargo crate. The output is
// deterministic: the same input, GoPatina version and options always give
// the same bytes, and the crate lists them in gopatina-manifest.sha256.
package emit

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"github.com/pigfox/gopatina/internal/borrow"
	"github.com/pigfox/gopatina/internal/load"
	"github.com/pigfox/gopatina/internal/subset"
)

// gen holds the state of one translation.
type gen struct {
	pkg    *load.Package
	info   *types.Info
	res    *borrow.Result
	errs   *subset.ErrPlan
	rt     map[string]bool // runtime support items used
	types  map[*types.TypeName]string
	funcs  map[*types.Func]string
	consts map[*types.Const]string
	fields map[*types.Var]string
	// structOwner is the type declared with each struct literal; a named
	// type with another type's struct (type B A) is an alias of it.
	structOwner map[*types.Struct]*types.TypeName
	// inits are the program's init functions, in the order Go runs them:
	// by package, then file, then source order.
	inits []*types.Func
	// A program of several packages (RFC 0009): the package being written,
	// and each other package's Rust module name.
	cur  *types.Package
	mods map[*types.Package]string
	// Package-level variables (RFC 0010): their kinds and Rust names, each
	// package's initVars, the locals changed ones are read into for one
	// statement, and the calls computed into locals first.
	globals      map[*types.Var]subset.GlobalKind
	gnames       map[*types.Var]string
	initVarsName map[*types.Package]string
	galias       map[*types.Var]string
	callTemps    map[*ast.CallExpr]string
	inGlobalStmt bool
	modules      []Module    // the other packages' Rust modules, once written
	ifc          *ifaceState // interface values (RFC 0012, iface.go)
	api          *apiItems   // a library crate's API (RFC 0014, library.go), or nil
	// Pointers that escape (RFC 0013, ptrs.go): the plan, the node standing
	// for a cell's contents while an access to it is rendered, and the
	// values of a write computed first.
	ptrs        *subset.PtrPlan
	cellRoot    map[ast.Expr]string
	inCellRead  bool
	inCellWrite bool
	cellName    string
	temps       map[ast.Expr]string
	// raw are the owned pointers being rendered as they are held, and
	// boxSlot those stored where a Box<T> is held (RFC 0017, step 2):
	// value wraps any other Box in Some.
	raw, boxSlot map[ast.Expr]bool
	// bound are the owned pointers (by their source text) that an
	// if let Some(name) binds in the statements being written.
	bound map[string]boundPtr
	// writing is set while a place being assigned is written: the owned
	// pointers it goes through are reached exclusively (RFC 0017, step 3).
	writing    bool
	inCellCall map[*ast.CallExpr]bool
	// rawCalls are calls of functions lowered to Result being rendered as
	// the Result (errvals.go).
	rawCalls map[*ast.CallExpr]bool
	// Per function.
	fn         *borrow.Func
	locals     map[types.Object]string
	lnames     *namer
	funcNames  map[string]string // see freeFuncNames
	recv       *types.Var
	refElem    map[*types.Var]bool     // range variables bound to an element
	nils       map[ast.Expr]types.Type // the type each untyped nil stands for
	hoisted    map[ast.Expr]string     // arguments computed into a variable first
	hoistedIdx map[ast.Expr]string     // assignment indexes computed into a variable first
	letTypes   []string                // the types of a tuple let, when Rust cannot infer them
	// Go comments, written where they stand in the Go source.
	comments     []*ast.CommentGroup
	usedComments map[*ast.CommentGroup]bool
	fnStart      token.Pos // the current function body's opening brace
	lastLine     int       // the last Go line written in the current block

	usedNames map[string]int // how many local objects share each Rust name
	out       *strings.Builder
	ind       int
	// labels for loops and switches, innermost last.
	labels []label
	tmp    int

	// Loop specialization (RFC 0006, ranges.go).
	fnBody      *ast.BlockStmt // the current function's body
	fast        *nestPlan      // set while writing a fast copy
	noPlan      bool           // set while writing a general copy
	quiet       bool           // write no Go comments (the fast copy of a versioned nest)
	specialized bool           // some nest was specialized
	notes       []Note         // decisions for -explain
}

// A label is an enclosing loop or switch, for break and continue.
type label struct {
	isSwitch bool
	// post is a 3-clause loop's post statement, written again before each
	// continue, since Rust's while loop has no post statement.
	post ast.Stmt
}

// source translates pkg to the text of src/main.rs and src/rt.rs.
func source(pkg *load.Package, res *borrow.Result, errs *subset.ErrPlan) (mainRS, rtRS string, notes []Note, modules []Module, api *apiItems) {
	g := &gen{
		pkg: pkg, info: pkg.Info, res: res, errs: errs,
		rt:         map[string]bool{},
		types:      map[*types.TypeName]string{},
		funcs:      map[*types.Func]string{},
		consts:     map[*types.Const]string{},
		fields:     map[*types.Var]string{},
		refElem:    res.RefElems,
		hoisted:    map[ast.Expr]string{},
		hoistedIdx: map[ast.Expr]string{},
		rawCalls:   map[*ast.CallExpr]bool{},
	}
	g.globals = subset.Globals(pkg)
	if subset.Library(pkg) {
		g.api = &apiItems{names: map[*types.Func]string{}, sentinel: map[*types.Var]string{}, notes: map[string][]string{}}
	}
	g.ptrs = subset.Ptrs(pkg)
	borrow.SetHeap(g.ptrs.HeapType)
	g.cellRoot, g.temps = map[ast.Expr]string{}, map[ast.Expr]string{}
	g.raw, g.boxSlot, g.bound = map[ast.Expr]bool{}, map[ast.Expr]bool{}, map[string]boundPtr{}
	g.inCellCall = map[*ast.CallExpr]bool{}
	g.gnames, g.initVarsName = map[*types.Var]string{}, map[*types.Package]string{}
	g.galias, g.callTemps = map[*types.Var]string{}, map[*ast.CallExpr]string{}
	g.nameDecls()
	g.findNils()
	g.initIfaces()
	g.usedComments = map[*ast.CommentGroup]bool{}
	for _, f := range pkg.Files {
		g.comments = append(g.comments, f.Comments...)
	}
	// Each top-level item is written on its own; items are separated by a
	// blank line, except the constants of one const declaration.
	itemsOf := map[*types.Package][]string{}
	item := func(f func(), joinPrev bool) {
		var b strings.Builder
		g.out = &b
		f()
		if b.Len() == 0 {
			return // a blank constant
		}
		items := itemsOf[g.cur]
		if joinPrev && len(items) > 0 {
			items[len(items)-1] += b.String()
			return
		}
		itemsOf[g.cur] = append(items, b.String())
	}
	for _, f := range pkg.Files {
		g.cur = pkg.PkgOf[f]
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.GenDecl:
				for i, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						item(func() { g.typeDecl(d, s) }, false)
					case *ast.ValueSpec:
						if d.Tok == token.CONST {
							item(func() { g.constDecl(d, s, true) }, i > 0)
						} else {
							item(func() { g.globalDecl(s) }, false)
						}
					}
				}
			case *ast.FuncDecl:
				// Methods of struct types are written in their type's impl
				// block; those of other named types are functions (RFC 0007).
				if d.Recv == nil || g.freeMethods(g.recvTypeName(g.info.Defs[d.Name].(*types.Func))) {
					item(func() { g.funcDecl(d) }, false)
				}
			}
		}
	}
	for _, p := range pkg.Pkgs {
		g.cur = p
		item(func() { g.initVars(p) }, false)
	}
	g.cur = pkg.Types
	if g.api != nil && g.api.errorAPI {
		g.helper("iface_is") // Error's is and unwrap (RFC 0014)
		g.helper("iface_unwrap")
	}
	machinery := ""
	if g.api != nil {
		// A library keeps the crate's own machinery (Dyn, Iface, the
		// helpers) in a private module, out of its API (RFC 0014).
		var b strings.Builder
		g.out = &b
		g.ifaceItems()
		machinery = b.String()
		item(func() { g.libraryErrorItems() }, false)
	} else {
		item(func() { g.ifaceItems() }, false)
	}
	var head strings.Builder
	if g.api != nil {
		head.WriteString(fmt.Sprintf("//! Generated by GoPatina from package %s (%s), as a library crate (RFC 0014).\n", pkg.Name, strings.Join(g.fileNames(), ", ")))
		head.WriteString("//! Its API is the package's exported items; api.txt lists them. Go's integers\n")
		head.WriteString("//! wrap: a workspace using this crate sets overflow-checks = false for it in\n")
		head.WriteString("//! its dev profile, as this crate's Cargo.toml shows.\n\n")
	} else {
		head.WriteString(fmt.Sprintf("//! Generated by GoPatina from package %s (%s).\n", pkg.Name, strings.Join(g.fileNames(), ", ")))
		head.WriteString("//! Edit the Go source and regenerate, or keep this crate and continue in Rust.\n\n")
	}
	head.WriteString("// Go allows unused functions, types, fields, parameters and assignments,\n")
	head.WriteString("// wraps integers on overflow, and panics on a division by zero at run time;\n")
	head.WriteString("// Rust warns about the first and rejects the others when it can see them.\n")
	head.WriteString("#![allow(\n")
	head.WriteString("    dead_code,\n")
	head.WriteString("    unused_assignments,\n")
	head.WriteString("    unused_variables,\n")
	head.WriteString("    arithmetic_overflow,\n")
	head.WriteString("    unconditional_panic\n")
	head.WriteString(")]\n")
	// These Clippy lints describe Rust idioms that a faithful Go translation
	// deliberately does not follow: Go source controls the constants,
	// comparisons, returns and names; Go slices are Vec even when an array
	// would do; and fmt.Sprintf/Print map to format!/print!. They are allowed
	// explicitly, not hidden by a blanket allow.
	head.WriteString("#![allow(clippy::approx_constant)] // constants are written by the Go source\n")
	head.WriteString("#![allow(clippy::assign_op_pattern)] // Go's x = x op y stays as written\n")
	head.WriteString("#![allow(clippy::eq_op)] // the Go source may compare a value with itself\n")
	head.WriteString("#![allow(clippy::format_in_format_args)] // fmt.Sprintf maps to format!\n")
	head.WriteString("#![allow(clippy::manual_range_contains)] // the Go source controls the comparison\n")
	head.WriteString("#![allow(clippy::manual_rem_euclid)] // the Go source computes the remainder as written\n")
	head.WriteString("#![allow(clippy::manual_unwrap_or)] // Go's error patterns map to match\n")
	head.WriteString("#![allow(clippy::manual_unwrap_or_default)] // Go's error patterns map to match\n")
	head.WriteString("#![allow(clippy::match_single_binding)] // a Go switch with only a default case\n")
	head.WriteString("#![allow(clippy::mut_range_bound)] // Go allows changing a range bound\n")
	if g.specialized {
		// A specialized loop (RFC 0006) indexes by position, as the Go
		// source does.
		head.WriteString("#![allow(clippy::needless_range_loop)] // the Go source indexes by position\n")
	}
	head.WriteString("#![allow(clippy::needless_return)] // the Go source controls the returns\n")
	head.WriteString("#![allow(clippy::nonminimal_bool)] // the Go source controls the condition\n")
	head.WriteString("#![allow(clippy::only_used_in_recursion)] // Go functions may recurse\n")
	head.WriteString("#![allow(clippy::reversed_empty_ranges)] // the Go source controls the range\n")
	head.WriteString("#![allow(clippy::should_implement_trait)] // Go function names are kept as written\n")
	head.WriteString("#![allow(clippy::single_match)] // the Go source controls the switch\n")
	head.WriteString("#![allow(clippy::useless_format)] // fmt.Sprintf maps to format!\n")
	head.WriteString("#![allow(clippy::useless_vec)] // Go slices are Vec, not arrays\n")
	if g.api != nil {
		// An exported function may return a value of an unexported type,
		// as in Go; and the crate's own items are pub(crate).
		head.WriteString("#![allow(private_interfaces)] // Go exports may name unexported types\n")
	} else if g.ifc.used && len(pkg.Pkgs) > 1 {
		// Dyn is public, for main's exported interface types, and holds
		// the types of the program's other packages, which are pub(crate).
		head.WriteString("#![allow(private_interfaces)] // Dyn holds other packages' types\n")
	}
	head.WriteString("\n")
	rtRS = rtModule(g.rt)
	if rtRS != "" {
		head.WriteString("mod rt;\n\n")
	}
	var modLines []string // rustfmt orders mod items by name
	for _, p := range pkg.Pkgs {
		if p == pkg.Types || len(itemsOf[p]) == 0 {
			continue
		}
		modLines = append(modLines, "mod "+g.mods[p]+";\n")
		body := strings.Join(itemsOf[p], "\n")
		text := "//! Package " + p.Path() + ", translated by GoPatina from its Go source.\n\n"
		if strings.Contains(body, "rt::") {
			text += "use crate::rt;\n\n"
		}
		g.modules = append(g.modules, Module{Path: "src/" + g.mods[p] + ".rs", Text: text + body})
	}
	if machinery != "" {
		modLines = append(modLines, "mod gopatina;\n")
		text := "//! GoPatina's machinery for interface and error values (RFC 0012),\n" +
			"//! private to this library crate (RFC 0014).\n\n" +
			"#[allow(unused_imports)]\nuse super::*;\n\n"
		g.modules = append(g.modules, Module{Path: "src/gopatina.rs", Text: text + machinery})
	}
	sort.Strings(modLines)
	head.WriteString(strings.Join(modLines, ""))
	if machinery != "" {
		head.WriteString("\n#[allow(unused_imports)]\nuse gopatina::*;\n")
	}
	if len(g.modules) > 0 {
		head.WriteString("\n")
	}
	return head.String() + strings.Join(itemsOf[pkg.Types], "\n"), rtRS, g.notes, g.modules, g.api
}

// A Module is the Rust source of one translated package other than main.
type Module struct{ Path, Text string }

// qual qualifies name, an item of package p, for the package being
// written: crate::m::name for another package of the program.
func (g *gen) qual(p *types.Package, name string) string {
	if p == nil || p == g.cur || g.mods[p] == "" {
		return name
	}
	return "crate::" + g.mods[p] + "::" + name
}

// vis is the visibility of an item: pub(crate) everywhere in a package
// other than main (Go has checked who uses what), else pub when exported.
func (g *gen) vis(exported bool) string {
	switch {
	case g.cur != nil && g.cur != g.pkg.Types:
		return "pub(crate) "
	case exported:
		return "pub "
	}
	return ""
}

func (g *gen) fileNames() []string {
	var names []string
	for _, f := range g.pkg.Files {
		if g.pkg.PkgOf[f] != g.pkg.Types {
			continue
		}
		name := g.pkg.Fset.Position(f.Pos()).Filename
		names = append(names, name[strings.LastIndexAny(name, `/\`)+1:])
	}
	return names
}

// nameDecls gives every package-level declaration its Rust name.
func (g *gen) nameDecls() {
	g.structOwner = map[*types.Struct]*types.TypeName{}
	for _, f := range g.pkg.Files {
		for _, d := range f.Decls {
			if gd, ok := d.(*ast.GenDecl); ok {
				for _, spec := range gd.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						if _, ok := ts.Type.(*ast.StructType); ok && !ts.Assign.IsValid() {
							obj := g.info.Defs[ts.Name].(*types.TypeName)
							g.structOwner[obj.Type().Underlying().(*types.Struct)] = obj
						}
					}
				}
			}
		}
	}
	g.mods = map[*types.Package]string{}
	modNames := newNamer("rt", "main", "crate", "self", "super", "std", "core")
	for _, p := range g.pkg.Pkgs {
		if p != g.pkg.Types {
			g.mods[p] = modNames.name(p.Name()+"_"+fmt.Sprint(len(g.mods)), snake)
		}
		g.namePackage(p)
	}
}

// namePackage gives every package-level declaration of p its Rust name,
// in p's own Rust namespace.
func (g *gen) namePackage(p *types.Package) {
	typeNames := newNamer()
	typeNames.sep = ""
	for n := range reservedTypes {
		typeNames.taken[n] = ""
	}
	valueNames := newNamer("main")
	constNames := newNamer()
	lib := g.api != nil && p == g.pkg.Types // a library's own package (RFC 0014)
	if lib {
		typeNames.taken["Error"] = "" // the API's Go error
	}
	// apiFunc names fn, of the library's API: a function lowered to Result
	// keeps a private name, beside its pub wrapper's.
	apiFunc := func(fn *types.Func, names *namer, goName string) {
		if lib && g.inAPI(fn) && g.errs.Funcs[fn] {
			g.api.names[fn] = names.name(goName, snake)
			g.funcs[fn] = names.name(goName+"Go", snake)
			return
		}
		g.funcs[fn] = names.name(goName, snake)
	}
	scope := p.Scope()
	for _, name := range scope.Names() { // sorted
		switch obj := scope.Lookup(name).(type) {
		case *types.TypeName:
			g.types[obj] = typeNames.name(name, camel)
			if obj.IsAlias() {
				continue
			}
			if g.freeMethods(obj) {
				mset := types.NewMethodSet(types.NewPointer(obj.Type()))
				for i := 0; i < mset.Len(); i++ {
					m := mset.At(i).Obj().(*types.Func)
					apiFunc(m, valueNames, name+m.Name())
				}
				continue
			}
			st := obj.Type().Underlying().(*types.Struct)
			fieldNames := newNamer()
			for i := 0; i < st.NumFields(); i++ {
				g.fields[st.Field(i)] = fieldNames.name(st.Field(i).Name(), snake)
			}
			methodNames := newNamer("clone", "default", "eq", "ne", "fmt")
			mset := types.NewMethodSet(types.NewPointer(obj.Type()))
			for i := 0; i < mset.Len(); i++ {
				m := mset.At(i).Obj().(*types.Func)
				apiFunc(m, methodNames, m.Name())
			}
		case *types.Func:
			if name == "main" && p == g.pkg.Types {
				g.funcs[obj] = "main"
				continue
			}
			apiFunc(obj, valueNames, name)
		case *types.Const:
			g.consts[obj] = constNames.name(name, screaming)
		case *types.Var:
			g.gnames[obj] = constNames.name(name, screaming)
			if lib && obj.Exported() {
				g.api.sentinel[obj] = valueNames.name(name, snake) // the checker allows only errors
			}
		}
	}
	g.initVarsName[p] = valueNames.name("initVars", snake)
	// init functions are in no scope: Go calls them, once each, before main.
	for _, f := range g.pkg.Files {
		if g.pkg.PkgOf[f] != p {
			continue
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "init" {
				obj := g.info.Defs[fd.Name].(*types.Func)
				g.inits = append(g.inits, obj)
				g.funcs[obj] = valueNames.name(fmt.Sprintf("init%d", len(g.inits)), snake)
			}
		}
	}
}

// line writes one line at the current indentation.
func (g *gen) line(s string) {
	if s == "" {
		g.out.WriteString("\n")
		return
	}
	g.out.WriteString(strings.Repeat(" ", g.ind*indentWidth) + s + "\n")
}

// stmtLine writes prefix + n + suffix as one statement, laid out to fit.
func (g *gen) stmtLine(prefix string, n node, suffix string) {
	ind := g.ind * indentWidth
	col := ind + len(prefix)
	f := n.flat()
	if strings.HasSuffix(prefix, " = ") && !fits(n, f, col, len(suffix)) && fits(n, f, ind+indentWidth, len(suffix)) {
		// rustfmt moves a right-hand side that fits on the next line there
		// rather than breaking it.
		g.line(strings.TrimSuffix(prefix, " "))
		g.ind++
		g.line(f + suffix)
		g.ind--
		return
	}
	g.line(prefix + layWith(n, col, ind, len(suffix)) + suffix)
}

// comments writes a Go doc comment as Rust doc comments.
func (g *gen) docComment(cg *ast.CommentGroup) {
	if cg == nil {
		return
	}
	g.usedComments[cg] = true
	for _, l := range strings.Split(strings.TrimRight(cg.Text(), "\n"), "\n") {
		if l == "" {
			g.line("///")
			continue
		}
		g.line("/// " + l)
	}
}

// freeMethods reports whether the methods of named type obj are written as
// functions taking the receiver first (RFC 0007): every named type that is
// not declared with a struct literal. Such a type, and an alias, is a Rust
// type alias of its underlying type's Rust type.
func (g *gen) freeMethods(obj *types.TypeName) bool {
	st, ok := obj.Type().Underlying().(*types.Struct)
	return !ok || g.structOwner[st] != obj
}

// recvTypeName returns the named type of method m's receiver.
func (g *gen) recvTypeName(m *types.Func) *types.TypeName {
	t := m.Signature().Recv().Type()
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	return types.Unalias(t).(*types.Named).Obj()
}

func (g *gen) typeDecl(d *ast.GenDecl, s *ast.TypeSpec) {
	obj := g.info.Defs[s.Name].(*types.TypeName)
	doc := s.Doc
	if doc == nil && len(d.Specs) == 1 {
		doc = d.Doc
	}
	g.docComment(doc)
	if obj.IsAlias() || g.freeMethods(obj) {
		vis := g.vis(obj.Exported())
		g.line(vis + "type " + g.types[obj] + " = " + g.rustType(obj.Type()) + ";")
		return
	}
	st := obj.Type().Underlying().(*types.Struct)
	derives := "Clone, Default, PartialEq"
	if borrow.KindOf(obj.Type()) == borrow.Copy {
		derives = "Clone, Copy, Default, PartialEq"
	}
	if g.api != nil && obj.Exported() && obj.Pkg() == g.pkg.Types && debuggable(obj.Type(), map[types.Type]bool{}) {
		derives = "Clone, " + strings.TrimPrefix(derives, "Clone, ") // a library's API type is Debug (RFC 0014)
		derives = strings.Replace(derives, "Default", "Debug, Default", 1)
	}
	g.line("#[derive(" + derives + ")]")
	vis := g.vis(obj.Exported())
	if st.NumFields() == 0 {
		g.line(vis + "struct " + g.types[obj] + " {}")
	} else {
		g.line(vis + "struct " + g.types[obj] + " {")
		g.ind++
		fieldDocs := map[string]*ast.CommentGroup{}
		fieldComments := map[string]*ast.CommentGroup{}
		for _, f := range s.Type.(*ast.StructType).Fields.List {
			for _, n := range f.Names {
				fieldDocs[n.Name] = f.Doc
				fieldComments[n.Name] = f.Comment
			}
		}
		// rustfmt aligns the trailing comments of a run of commented
		// fields (measured): a field without one ends the run, and a field
		// with a doc comment keeps one space and does not count.
		var run []fieldLine
		flush := func() {
			g.alignedComments(run)
			run = nil
		}
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			fvis := g.vis(f.Exported())
			field := fvis + g.fields[f] + ": " + g.rustType(f.Type()) + ","
			if c := fieldComments[f.Name()]; c != nil && len(c.List) == 1 {
				run = append(run, fieldLine{doc: fieldDocs[f.Name()], field: field, comment: c.List[0].Text})
				continue
			}
			flush()
			g.docComment(fieldDocs[f.Name()])
			g.line(field)
		}
		flush()
		g.ind--
		g.line("}")
	}
	g.chainDrop(obj, st)
	g.methods(obj)
}

// dropFields returns the fields of struct type obj that own another
// value of obj's own type, when such chains can be built in a loop
// (subset.PtrPlan.Chained): freeing one must not recurse per link.
func (g *gen) dropFields(obj *types.TypeName) []*types.Var {
	if g.ptrs == nil || !g.ownPtr(types.NewPointer(obj.Type())) || !g.ptrs.Chained(obj.Type()) {
		return nil
	}
	st := obj.Type().Underlying().(*types.Struct)
	var out []*types.Var
	for i := 0; i < st.NumFields(); i++ {
		if p, ok := types.Unalias(st.Field(i).Type()).(*types.Pointer); ok && types.Identical(p.Elem(), obj.Type()) {
			out = append(out, st.Field(i))
		}
	}
	return out
}

// chainDrop writes, for a struct type whose chains can be longer than any
// call depth, the Drop a Rust programmer writes for a linked list: the
// links taken out one at a time in a loop, not freed one call deeper
// each, which would overflow the stack.
func (g *gen) chainDrop(obj *types.TypeName, st *types.Struct) {
	fs := g.dropFields(obj)
	if len(fs) == 0 {
		return
	}
	name := g.types[obj]
	g.line("")
	g.line("/// Frees a chain one link at a time, so a long one cannot overflow")
	g.line("/// the stack.")
	g.line("impl Drop for " + name + " {")
	g.ind++
	g.line("fn drop(&mut self) {")
	g.ind++
	if len(fs) == 1 {
		f := g.fields[fs[0]]
		g.line("let mut next = self." + f + ".take();")
		g.line("while let Some(mut link) = next {")
		g.ind++
		g.line("next = link." + f + ".take();")
		g.ind--
		g.line("}")
	} else {
		g.line("let mut links: Vec<Box<" + name + ">> = Vec::new();")
		for _, f := range fs {
			g.line("links.extend(self." + g.fields[f] + ".take());")
		}
		g.line("while let Some(mut link) = links.pop() {")
		g.ind++
		for _, f := range fs {
			g.line("links.extend(link." + g.fields[f] + ".take());")
		}
		g.ind--
		g.line("}")
	}
	g.ind--
	g.line("}")
	g.ind--
	g.line("}")
}

// A fieldLine is a struct field with a trailing comment, and its doc
// comment, if any.
type fieldLine struct {
	doc            *ast.CommentGroup
	field, comment string
}

// alignedComments writes a run of fields with trailing comments as rustfmt
// lays them out: each comment one space after the widest field of the run
// without a doc comment, unless that would take a line past the maximum
// width; a field with a doc comment keeps one space.
func (g *gen) alignedComments(run []fieldLine) {
	width := 0
	for _, f := range run {
		if f.doc == nil {
			width = max(width, len(f.field))
		}
	}
	align := true
	for _, f := range run {
		if f.doc == nil && g.ind*indentWidth+width+1+len(f.comment) > maxWidth {
			align = false
		}
	}
	for _, f := range run {
		g.docComment(f.doc)
		pad := " "
		if align && f.doc == nil {
			pad = strings.Repeat(" ", width-len(f.field)+1)
		}
		g.line(f.field + pad + f.comment)
	}
}

// methods writes the impl block holding every method of named type obj, in
// source order.
func (g *gen) methods(obj *types.TypeName) {
	var decls []*ast.FuncDecl
	for _, f := range g.pkg.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil {
				continue
			}
			t := g.info.Types[fd.Recv.List[0].Type].Type
			if p, ok := t.(*types.Pointer); ok {
				t = p.Elem()
			}
			if types.Unalias(t).(*types.Named).Obj() == obj {
				decls = append(decls, fd)
			}
		}
	}
	if len(decls) == 0 {
		return
	}
	g.line("")
	g.line("impl " + g.types[obj] + " {")
	g.ind++
	for i, fd := range decls {
		if i > 0 {
			g.line("")
		}
		g.funcDecl(fd)
	}
	g.ind--
	g.line("}")
}

// constDecl writes the constants of one spec of a const declaration.
func (g *gen) constDecl(d *ast.GenDecl, s *ast.ValueSpec, top bool) {
	for _, name := range s.Names {
		if name.Name == "_" {
			continue
		}
		c := g.info.Defs[name].(*types.Const)
		if top && (s.Doc != nil || (len(d.Specs) == 1 && d.Doc != nil)) {
			doc := s.Doc
			if doc == nil {
				doc = d.Doc
			}
			g.docComment(doc)
		}
		t := types.Default(c.Type())
		rt := g.rustType(t)
		if isStringType(t) {
			rt = "&str"
		}
		vis := ""
		if top && g.api != nil && c.Exported() && c.Pkg() == g.pkg.Types {
			vis = "pub " // in a library's API (RFC 0014)
		}
		g.line(fmt.Sprintf("%sconst %s: %s = %s;", vis, g.constName(c), rt, constLit(c.Val(), t)))
	}
}

func (g *gen) constName(c *types.Const) string {
	if n, ok := g.consts[c]; ok {
		return n
	}
	// A local constant.
	return g.localName(c.Name(), c, screaming)
}

// rustType returns the Rust type for a Go value type.
func (g *gen) rustType(t types.Type) string {
	switch t := types.Unalias(t).(type) {
	case *types.Basic:
		switch t.Kind() {
		case types.Int, types.Int64, types.UntypedInt:
			return "i64"
		case types.Int8:
			return "i8"
		case types.Int16:
			return "i16"
		case types.Int32, types.UntypedRune:
			return "i32"
		case types.Uint8:
			return "u8"
		case types.Uint16:
			return "u16"
		case types.Uint32:
			return "u32"
		case types.Uint, types.Uint64, types.Uintptr: // every target is 64-bit
			return "u64"
		case types.Float32:
			return "f32"
		case types.Float64, types.UntypedFloat:
			return "f64"
		case types.Bool, types.UntypedBool:
			return "bool"
		}
		return "String"
	case *types.Slice:
		return "Vec<" + g.rustType(t.Elem()) + ">"
	case *types.Pointer:
		return g.ptrType(t) // a heap pointer; a borrow is a parameter's type
	}
	if subset.IsIface(t) {
		return g.root("Iface")
	}
	n := types.Unalias(t).(*types.Named)
	if n.Obj().Pkg().Path() == "io" {
		return "dyn std::io::Write"
	}
	if st, ok := n.Underlying().(*types.Struct); ok {
		owner := g.structOwner[st]
		return g.qual(owner.Pkg(), g.types[owner])
	}
	// A named non-struct type is its underlying type (RFC 0007).
	return g.rustType(n.Underlying())
}

// paramType returns the Rust type of a parameter passed in mode m.
func (g *gen) paramType(t types.Type, m borrow.Mode) string {
	if p, ok := t.(*types.Pointer); ok && !g.heapPtr(t) {
		// A pointer borrows the pointed-to value itself, so a slice can
		// grow through it.
		if m == borrow.RefMut {
			return "&mut " + g.rustType(p.Elem())
		}
		return "&" + g.rustType(p.Elem())
	}
	if g.ownPtr(t) && m == borrow.Ref {
		// A borrowed owned pointer (RFC 0016): nil, or a borrow of the
		// value, as Rust writes a parameter that may be absent.
		return "Option<&" + g.rustType(types.Unalias(t).(*types.Pointer).Elem()) + ">"
	}
	elem := t
	base := g.rustType(elem)
	switch m {
	case borrow.Ref:
		switch {
		case isStringType(elem):
			return "&str"
		case isSlice(elem):
			return "&[" + g.rustType(elem.Underlying().(*types.Slice).Elem()) + "]"
		}
		return "&" + base
	case borrow.RefMut:
		if isSlice(elem) {
			return "&mut [" + g.rustType(elem.Underlying().(*types.Slice).Elem()) + "]"
		}
		return "&mut " + base
	}
	return base
}

func isStringType(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsString != 0
}

func isSlice(t types.Type) bool {
	_, ok := t.Underlying().(*types.Slice)
	return ok
}

func isFloat(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsFloat != 0
}

func isBool(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsBoolean != 0
}

func isInt(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsInteger != 0
}

func isUnsigned(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsUnsigned != 0
}

// findNils records the type each nil in the package stands for: an empty
// slice, from the place it is assigned, passed or returned to.
func (g *gen) findNils() {
	g.nils = map[ast.Expr]types.Type{}
	set := func(e ast.Expr, t types.Type) {
		if g.info.Types[e].IsNil() {
			g.nils[ast.Unparen(e)] = t
		}
	}
	for _, f := range g.pkg.Files {
		var sigs []*types.Signature
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				sigs = append(sigs[:0], g.info.Defs[n.Name].(*types.Func).Signature())
			case *ast.ReturnStmt:
				res := sigs[0].Results()
				if len(n.Results) == res.Len() {
					for i, r := range n.Results {
						set(r, res.At(i).Type())
					}
				}
			case *ast.AssignStmt:
				if len(n.Lhs) == len(n.Rhs) {
					for i, r := range n.Rhs {
						set(r, g.typeOf(n.Lhs[i]))
					}
				}
			case *ast.ValueSpec:
				for i, v := range n.Values {
					if i < len(n.Names) && g.info.Defs[n.Names[i]] != nil {
						set(v, g.info.Defs[n.Names[i]].Type())
					}
				}
			case *ast.CallExpr:
				if id, ok := ast.Unparen(n.Fun).(*ast.Ident); ok && id.Name == "append" {
					if s, ok := g.typeOf(n.Args[0]).Underlying().(*types.Slice); ok {
						for _, a := range n.Args[1:] {
							set(a, s.Elem())
						}
					}
					return true
				}
				if sig, ok := g.typeOf(n.Fun).(*types.Signature); ok {
					for i, a := range n.Args {
						if i < sig.Params().Len() {
							set(a, sig.Params().At(i).Type())
						}
					}
				}
			case *ast.CompositeLit:
				switch u := g.typeOf(n).Underlying().(type) {
				case *types.Slice:
					for _, el := range n.Elts {
						set(el, u.Elem())
					}
				case *types.Struct:
					for i, el := range n.Elts {
						if kv, ok := el.(*ast.KeyValueExpr); ok {
							for j := 0; j < u.NumFields(); j++ {
								if u.Field(j).Name() == kv.Key.(*ast.Ident).Name {
									set(kv.Value, u.Field(j).Type())
								}
							}
							continue
						}
						set(el, u.Field(i).Type())
					}
				}
			}
			return true
		})
	}
}
