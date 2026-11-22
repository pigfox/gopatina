// Package coverage measures how much of real Go code the patina subset
// accepts, from the subset checker's own diagnostics (docs/COVERAGE.md).
//
// A function or method is accepted when the checker reports nothing inside
// it (its signature and body) and every declaration of its own package that
// it uses (a type, a function, a variable or a constant) is accepted too.
// Diagnostics outside any function (a type, a package-level variable, an
// import) reject the declaration they are in, and through it every function
// that uses it. The rule that only package main is supported, and the need
// for a func main, are left out: they reject every library, and the
// question here is which code the subset can express. For the same reason
// a library is type-checked as package main (cmd/gopatina-coverage), since
// the checker treats the named types of any other package as foreign.
package coverage

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pigfox/gopatina/internal/diag"
	"github.com/pigfox/gopatina/internal/load"
)

// A Func is one function or method of a package.
type Func struct {
	Name     string   `json:"name"`     // Recv.Name for a method
	File     string   `json:"file"`     // base name of its file
	Lines    int      `json:"lines"`    // lines from func to the closing brace
	Accepted bool     `json:"accepted"` // inside the subset, with what it uses
	Excluded bool     `json:"excluded"` // uses reflect or unsafe, itself or through what it uses
	Features []string `json:"features"` // what keeps it out, its own and inherited, sorted
}

// A Package is the measurement of one package.
type Package struct {
	Path      string `json:"path"`
	Module    string `json:"module"` // "std" for the standard library
	Funcs     []Func `json:"funcs"`
	Files     int    `json:"files"`          // files measured (not generated)
	FilesOK   int    `json:"files_accepted"` // with no diagnostic at all
	Accepted  bool   `json:"accepted"`       // every file, declaration and function
	Generated int    `json:"generated_files"`
	// Unclassified counts the diagnostics Feature puts in "other", by
	// message, so the categories can be checked against real code.
	Unclassified map[string]int `json:"unclassified,omitempty"`
	// Messages counts every diagnostic by message, for the summary's top
	// messages of each feature.
	Messages map[string]int `json:"messages,omitempty"`

	decls      []*decl // for Resolve
	filesClean bool    // no file has a diagnostic outside a declaration
}

// Excluded is the feature of code that uses package reflect or unsafe: out
// of scope (the project lead, 2026-11-21), so such functions are counted
// apart and left out of the shares.
const Excluded = "reflect and unsafe (excluded)"

// Feature names the construct a diagnostic is about, from its message.
// Unknown messages fall into "other"; every message the checker can give
// is covered by a test.
func Feature(msg string) string {
	// A named type or alias is rejected for what it is made of (RFC 0007):
	// "type Code: type uint32 ..." needs uint32.
	msg = declPrefixRE.ReplaceAllString(msg, "")
	if f := fmtFeature(msg); f != "" {
		return f
	}
	for _, r := range rules {
		if r.re.MatchString(msg) {
			return r.name
		}
	}
	return "other"
}

// The fmt features: what a printing diagnostic needs, from the verb and the
// type it names.
const (
	FmtVerbs       = "fmt verbs and flags"
	FmtMayPanic    = "fmt: String or Error method that may panic"
	interfacesName = "interfaces"
	foreignName    = "types from other packages"
)

var (
	declPrefixRE  = regexp.MustCompile(`^type [^ ]+: (an alias of )?`)
	printTypeRE   = regexp.MustCompile(`^(?:printing a value|the format (%.*?)) (?:of|with a value of) type (.+?)(?: is not supported|$)`)
	ifaceTypeRE   = regexp.MustCompile(`^(\[\])*(interface \{\}|any)$`)
	foreignTypeRE = regexp.MustCompile(`^[*\[\]]*[a-z][a-z0-9_./-]*\.[A-Za-z_]`)
)

// fmtFeature classifies a diagnostic about printing, or returns "": a value
// of an interface type needs interfaces; one of another package's type
// needs that type; a %# verb, a flag or verb the type lacks, or printing a
// struct or pointer of the package needs fmt work.
func fmtFeature(msg string) string {
	if strings.HasPrefix(msg, "printing a value whose ") {
		return FmtMayPanic
	}
	if strings.HasPrefix(msg, "printing a slice of a type with") {
		return FmtVerbs
	}
	m := printTypeRE.FindStringSubmatch(msg)
	if m == nil {
		return ""
	}
	verb, t := m[1], m[2]
	switch {
	case strings.Contains(verb, "#"):
		return FmtVerbs
	case ifaceTypeRE.MatchString(t):
		return interfacesName
	case foreignTypeRE.MatchString(t) && !strings.HasPrefix(strings.TrimLeft(t, "*[]"), "main."):
		return foreignName
	}
	return FmtVerbs
}

type rule struct {
	name string
	re   *regexp.Regexp
}

var rules = func() []rule {
	list := [][2]string{
		{"package-main rule", `only package main is supported|has no func main\(\)`},
		{Excluded, `\b(reflect|unsafe)\.[A-Za-z_]|^package (reflect|unsafe) is not supported|\bunsafe\.Pointer\b`},
		{"arrays", `^range over (an array|a pointer to an array)`},
		{"goroutines and channels", `go statement|^select|channel|\bchan\b|the <- operator`},
		{"maps", `map type|range over (a )?map|make of map`},
		{"interfaces", `interface type|type assertion|type switch|io\.Writer|^calling methods on|^type any\b`},
		{"closures and function values", `function literal|function value|function type|as a value|method value|method expression|range over a function`},
		{"generics", `generic|Go 1\.18`},
		{"defer", `^defer`},
		{"package-level variables", `package-level variable`},
		{"imports of other packages", `^package [^ ]+ is not supported|renamed or dot import`},
		{"standard library (calls and variables)", `^[a-z][a-z0-9_]*(/[a-z0-9_.]+)*\.[A-Za-z_][A-Za-z0-9_]* is not supported|^vendor/`},
		{"calls into other modules", `^[a-z0-9.-]+\.[a-z]+/[^ ]*\.[A-Za-z_][A-Za-z0-9_]* is not supported`},
		{"other integer and float types", `^type (\[\])*(u?int(8|16)?|uint32|uint64|uintptr|float32|complex(64|128))\b|complex number`},
		{"types from other packages", `^type (\[\])*\*?[a-z][^ ]*\.[A-Za-z_]|struct type from another package`},
		{"pointers", `pointer|address of|passing nil`},
		{"slicing", `slice expression|s\[i:j\]`},
		{"arrays", `array type|range over \[[0-9]`},
		{"interfaces", `^a type assertion in an assignment target`},
		{"statement forms", `^this expression statement|^this assignment target|range loop that assigns to existing`},
		{"named non-struct types", `named type that is not a struct`},
		{"type aliases", `type alias`},
		{"embedded and promoted fields", `embedded field|promoted field`},
		{"struct fields of type error", `struct field`},
		{"anonymous structs and local types", `anonymous struct|type declared inside a function`},
		{"error handling patterns", `error|err != nil|holds no value|used outside a return`},
		{"variadic functions", `variadic|\.\.\. arguments`},
		{"labels, goto and fallthrough", `label|fallthrough|break that leaves`},
		{"built-ins", `built-in|^make of|^append whose|^panic with`},
		{"strings and formatting", `format string|converting|string constant|comparing a slice with nil`},
		{"shifts by a variable count", `non-constant count`},
		{"functions without a body (assembly)", `func init|without a body`},
	}
	var rs []rule
	for _, l := range list {
		rs = append(rs, rule{l[0], regexp.MustCompile(l[1])})
	}
	return rs
}()

// excluded reports a diagnostic that applies to every library alike.
func excluded(msg string) bool { return Feature(msg) == "package-main rule" }

// generatedRE is the marker go generate tools write (golang.org/s/generatedcode).
var generatedRE = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

var packageRE = regexp.MustCompile(`(?m)^package `)

// Generated reports whether src is a generated Go file: the marker must
// come before the package clause.
func Generated(src []byte) bool {
	if loc := packageRE.FindIndex(src); loc != nil {
		src = src[:loc[0]]
	}
	return generatedRE.Match(src)
}

// decl is a top-level declaration: a function, or the spec of a type, a
// variable or a constant.
type decl struct {
	node     ast.Node
	fn       *ast.FuncDecl // nil for a spec
	key      string        // its key across the corpus (Key)
	features map[string]bool
	uses     map[*decl]bool  // in its own package
	foreign  map[string]bool // in other packages of the corpus, by key
	module   string          // its package's module ("std" for the standard library)
	ok       bool
	funcIdx  int // its index in Package.Funcs, or -1
}

// Outside is the feature of a declaration that uses one of a package the
// measurement has not measured (skipped, or outside the corpus).
const Outside = "outside the measured corpus"

// ViaDeps is the feature of a declaration that reaches reflect or unsafe
// only through another module (the standard library included): out of
// scope is a program that uses reflect or unsafe itself, so this is a
// surface GoPatina has to provide, not an exclusion.
const ViaDeps = "reflect or unsafe inside dependencies"

// Key names a package-level object across the corpus: path.Name, or
// path.Type.Method for a method.
func Key(obj types.Object) string {
	if f, ok := obj.(*types.Func); ok {
		if r := f.Signature().Recv(); r != nil {
			t := r.Type()
			if p, ok := t.(*types.Pointer); ok {
				t = p.Elem()
			}
			if n, ok := types.Unalias(t).(*types.Named); ok {
				return obj.Pkg().Path() + "." + n.Obj().Name() + "." + obj.Name()
			}
		}
	}
	return obj.Pkg().Path() + "." + obj.Name()
}

// Measure turns the checker's diagnostics for pkg into a Package. Files
// whose source is generated are measured for their declarations' sake (a
// function may use them) but their functions are not counted.
func Measure(pkg *load.Package, path, module string, diags []diag.Diagnostic) Package {
	p := Package{Path: path, Module: module}
	var decls []*decl
	byObj := map[types.Object]*decl{}
	fileOf := map[*decl]*ast.File{}
	generated := map[*ast.File]bool{}
	fileDiags := map[string]int{}
	for _, f := range pkg.Files {
		name := pkg.Fset.Position(f.Pos()).Filename
		generated[f] = Generated(pkg.Src[name])
		add := func(n ast.Node, fn *ast.FuncDecl, names ...*ast.Ident) {
			d := &decl{node: n, fn: fn, features: map[string]bool{}, uses: map[*decl]bool{}, foreign: map[string]bool{}, module: module, funcIdx: -1}
			decls = append(decls, d)
			fileOf[d] = f
			for _, id := range names {
				if obj := pkg.Info.Defs[id]; obj != nil {
					byObj[obj] = d
					if d.key == "" && obj.Pkg() != nil {
						d.key = Key(obj)
					}
				}
			}
		}
		for _, x := range f.Decls {
			switch x := x.(type) {
			case *ast.FuncDecl:
				add(x, x, x.Name)
			case *ast.GenDecl:
				for _, s := range x.Specs {
					switch s := s.(type) {
					case *ast.TypeSpec:
						add(s, nil, s.Name)
					case *ast.ValueSpec:
						add(s, nil, s.Names...)
					}
				}
			}
		}
	}
	// Each diagnostic belongs to the declaration that contains it; one
	// outside every declaration (an import) belongs to its file.
	for _, dg := range diags {
		if excluded(dg.Msg) {
			continue
		}
		fileDiags[dg.Pos.Filename]++
		if p.Messages == nil {
			p.Messages = map[string]int{}
		}
		p.Messages[dg.Msg]++
		if Feature(dg.Msg) == "other" {
			if p.Unclassified == nil {
				p.Unclassified = map[string]int{}
			}
			p.Unclassified[dg.Msg]++
		}
		for _, d := range decls {
			start, end := pkg.Fset.Position(d.node.Pos()), pkg.Fset.Position(d.node.End())
			if start.Filename == dg.Pos.Filename && start.Offset <= dg.Pos.Offset && dg.Pos.Offset < end.Offset {
				d.features[Feature(dg.Msg)] = true
			}
		}
	}
	// What each declaration uses of its own package, and of the other
	// packages of the program (RFC 0009), which Resolve looks up.
	for _, d := range decls {
		ast.Inspect(d.node, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				obj := pkg.Info.Uses[id]
				if u := byObj[obj]; u != nil && u != d {
					d.uses[u] = true
				} else if foreign(pkg, obj) {
					d.foreign[Key(obj)] = true
				}
			}
			return true
		})
		if d.fn != nil && d.fn.Recv != nil {
			if u := byObj[recvType(pkg, d.fn)]; u != nil {
				d.uses[u] = true
			}
		}
	}
	// A declaration is accepted when it has no feature of its own and all
	// it uses is accepted; features are inherited along the uses.
	for _, d := range decls {
		d.ok = len(d.features) == 0
	}
	for changed := true; changed; {
		changed = false
		for _, d := range decls {
			for u := range d.uses {
				for f := range u.features {
					if !d.features[f] {
						d.features[f] = true
						changed = true
					}
				}
				if !u.ok && d.ok {
					d.ok = false
					changed = true
				}
			}
		}
	}
	p.decls = decls
	for _, d := range decls {
		if d.fn == nil || generated[fileOf[d]] {
			continue
		}
		d.funcIdx = len(p.Funcs)
		start, end := pkg.Fset.Position(d.fn.Pos()), pkg.Fset.Position(d.fn.End())
		var fs []string
		for f := range d.features {
			fs = append(fs, f)
		}
		sort.Strings(fs)
		p.Funcs = append(p.Funcs, Func{
			Name: funcName(d.fn), File: baseName(start.Filename), Lines: end.Line - start.Line + 1,
			Accepted: d.ok, Excluded: d.features[Excluded], Features: fs,
		})
	}
	for _, f := range pkg.Files {
		if generated[f] {
			p.Generated++
			continue
		}
		p.Files++
		if fileDiags[pkg.Fset.Position(f.Pos()).Filename] == 0 {
			p.FilesOK++
		}
	}
	p.filesClean = true
	for _, n := range fileDiags {
		if n > 0 {
			p.filesClean = false
		}
	}
	p.settle()
	return p
}

// foreign reports whether obj is a package-level declaration of another
// package of the program: one Resolve looks up. Fields belong to their
// struct type, and intrinsics are written as Rust, not looked up.
func foreign(pkg *load.Package, obj types.Object) bool {
	if obj == nil || obj.Pkg() == nil || obj.Pkg() == pkg.Types || !pkg.Local[obj.Pkg()] || load.IsIntrinsic(obj) {
		return false
	}
	if v, ok := obj.(*types.Var); ok && v.IsField() {
		return false
	}
	_, method := obj.(*types.Func)
	return method || obj.Parent() == obj.Pkg().Scope()
}

// settle writes the declarations' acceptance and features into p.
func (p *Package) settle() {
	p.Accepted = p.filesClean
	for _, d := range p.decls {
		if !d.ok {
			p.Accepted = false
		}
		if d.funcIdx < 0 {
			continue
		}
		var fs []string
		for f := range d.features {
			fs = append(fs, f)
		}
		sort.Strings(fs)
		f := &p.Funcs[d.funcIdx]
		f.Accepted, f.Excluded, f.Features = d.ok, d.features[Excluded], fs
	}
}

// Resolve applies the rule across packages (RFC 0009): a declaration is
// accepted when it and everything it uses, in its own package and in
// every other package of the corpus, is accepted, and it inherits the
// features of what it uses. A use of a package that was not measured is
// the feature Outside.
func Resolve(pkgs []Package) {
	index := map[string]*decl{}
	for i := range pkgs {
		for _, d := range pkgs[i].decls {
			if d.key != "" {
				index[d.key] = d
			}
		}
	}
	inherit := func(d, u *decl) bool {
		changed := false
		for f := range u.features {
			if f == Excluded && u.module != d.module {
				f = ViaDeps // reflect or unsafe of another module
			}
			if !d.features[f] {
				d.features[f] = true
				changed = true
			}
		}
		if !u.ok && d.ok {
			d.ok = false
			changed = true
		}
		return changed
	}
	outside := &decl{features: map[string]bool{Outside: true}}
	for changed := true; changed; {
		changed = false
		for i := range pkgs {
			for _, d := range pkgs[i].decls {
				for k := range d.foreign {
					u := index[k]
					if u == nil {
						u = outside
					}
					changed = inherit(d, u) || changed
				}
				for u := range d.uses {
					changed = inherit(d, u) || changed
				}
			}
		}
	}
	for i := range pkgs {
		pkgs[i].settle()
	}
}

// recvType returns the named type a method is declared on.
func recvType(pkg *load.Package, fn *ast.FuncDecl) types.Object {
	t := pkg.Info.Defs[fn.Name].(*types.Func).Signature().Recv().Type()
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	return t.(*types.Named).Obj()
}

func funcName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	t := fn.Recv.List[0].Type
	for {
		switch x := t.(type) {
		case *ast.StarExpr:
			t = x.X
			continue
		case *ast.IndexExpr:
			t = x.X
			continue
		case *ast.IndexListExpr:
			t = x.X
			continue
		case *ast.Ident:
			return x.Name + "." + fn.Name.Name
		}
		return fn.Name.Name
	}
}

func baseName(p string) string { return filepath.Base(p) }
