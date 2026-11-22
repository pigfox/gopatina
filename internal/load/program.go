package load

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pigfox/gopatina/internal/diag"
)

// Bridged are the packages patina mode maps straight to Rust (fmt, and the
// functions of the others that phase 1 supports). They are imported from
// export data and never translated; every other package a program imports
// is translated from its Go source (RFC 0009).
var Bridged = map[string]bool{
	"fmt": true, "strings": true, "errors": true, "os": true, "io": true, "slices": true, "bytes": true,
}

// Intrinsics are the functions of translated packages that patina mode
// writes as Rust instead of translating, as gc writes them as machine
// instructions (RFC 0009): their results follow gc, also where Go's
// documentation leaves them undefined.
var Intrinsics = map[string]bool{}

func init() {
	for _, n := range []string{
		"Add", "Add64", "Sub", "Sub64", "Mul", "Mul64",
		"RotateLeft", "RotateLeft8", "RotateLeft16", "RotateLeft32", "RotateLeft64",
		"LeadingZeros", "LeadingZeros8", "LeadingZeros16", "LeadingZeros32", "LeadingZeros64",
		"TrailingZeros", "TrailingZeros8", "TrailingZeros16", "TrailingZeros32", "TrailingZeros64",
		"OnesCount", "OnesCount8", "OnesCount16", "OnesCount32", "OnesCount64",
		"Len", "Len8", "Len16", "Len32", "Len64",
		"ReverseBytes", "ReverseBytes16", "ReverseBytes32", "ReverseBytes64",
	} {
		Intrinsics["math/bits."+n] = true
	}
}

// IsIntrinsic reports whether obj is one of the Intrinsics.
func IsIntrinsic(obj types.Object) bool {
	return obj != nil && obj.Pkg() != nil && Intrinsics[obj.Pkg().Path()+"."+obj.Name()]
}

// listed is one package of go list -deps -json.
type listed struct {
	ImportPath string
	Dir        string
	Name       string
	Standard   bool
	GoFiles    []string
	Imports    []string
	ImportMap  map[string]string
	Module     *struct{ GoVersion string }
	Error      *struct{ Err string }
}

// goList runs go list -deps -json for the package in dir, with the patina
// build tag set and cgo off, as patina builds use them.
var goList = func(dir string) ([]byte, error) {
	cmd := exec.Command("go", "list", "-deps", "-json", "-tags", BuildTag, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// program loads the package in dir together with every package it imports
// that is not bridged, from source, and returns one Package that holds them
// all: their files (the main package's whole, the others' only with what
// the program reaches), one Info, and the packages in Go's initialization
// order. dir must be absolute.
func program(dir, cwd string) (*Package, error) {
	out, err := goList(dir)
	if err != nil {
		return nil, err
	}
	var all []*listed
	byPath := map[string]*listed{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var l listed
		if err := dec.Decode(&l); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("go list: %v", err)
		}
		if l.Error != nil {
			return nil, fmt.Errorf("%s", l.Error.Err)
		}
		all = append(all, &l)
		byPath[l.ImportPath] = &l
	}
	main := all[len(all)-1] // go list -deps lists the named package last
	// The packages to translate: the main package and, from it, every import
	// that is not bridged (or unsafe, which the subset checker rejects).
	translated := map[string]bool{}
	var mark func(l *listed)
	mark = func(l *listed) {
		if translated[l.ImportPath] {
			return
		}
		translated[l.ImportPath] = true
		for _, imp := range l.Imports {
			if to, ok := l.ImportMap[imp]; ok {
				imp = to
			}
			if Bridged[imp] || imp == "unsafe" || byPath[imp] == nil {
				continue
			}
			mark(byPath[imp])
		}
	}
	mark(main)

	fset := token.NewFileSet()
	checked := map[string]*Package{}
	var order []*Package // dependency order, from go list
	for _, l := range all {
		if !translated[l.ImportPath] {
			continue
		}
		// With cgo off, go list leaves a cgo file out of GoFiles, so a
		// package that needs one fails to type-check, as with go build.
		goVersion := ""
		if l.Module != nil && l.Module.GoVersion != "" {
			goVersion = "go" + l.Module.GoVersion
		}
		srcs := map[string][]byte{}
		for _, name := range l.GoFiles {
			path := filepath.Join(l.Dir, name)
			src, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			srcs[path] = src
		}
		path := l.ImportPath
		if l == main {
			path = "main"
		}
		pkg, err := checkWith(l.Dir, cwd, path, goVersion, srcs, fset, importFrom{checked, l.ImportMap})
		if err != nil {
			return nil, err
		}
		checked[l.ImportPath] = pkg
		order = append(order, pkg)
	}
	return merge(initOrder(order, byPath), order[len(order)-1]), nil
}

// importFrom imports a translated package already checked from source,
// and any other from export data.
type importFrom struct {
	checked map[string]*Package
	m       map[string]string
}

func (im importFrom) Import(path string) (*types.Package, error) {
	if to, ok := im.m[path]; ok {
		path = to
	}
	if p, ok := im.checked[path]; ok {
		return p.Types, nil
	}
	return lockedImporter{}.Import(path)
}

// initOrder sorts pkgs (in dependency order, main last) into Go's
// initialization order: repeatedly the first package by import path whose
// translated imports are all initialized (the Go 1.21 rule).
func initOrder(pkgs []*Package, byPath map[string]*listed) []*Package {
	done := map[*types.Package]bool{}
	var out []*Package
	rest := append([]*Package(nil), pkgs[:len(pkgs)-1]...)
	sort.Slice(rest, func(i, j int) bool { return rest[i].Types.Path() < rest[j].Types.Path() })
	for len(rest) > 0 {
		for i, p := range rest {
			ready := true
			for _, imp := range p.Types.Imports() {
				if isIn(imp, pkgs) && !done[imp] {
					ready = false
				}
			}
			if ready {
				done[p.Types] = true
				out = append(out, p)
				rest = append(rest[:i], rest[i+1:]...)
				break
			}
		}
	}
	return append(out, pkgs[len(pkgs)-1])
}

func isIn(t *types.Package, pkgs []*Package) bool {
	for _, p := range pkgs {
		if p.Types == t {
			return true
		}
	}
	return false
}

// merge returns one Package for the program: the main package's own
// fields, every package's files (main's whole, the others' with only the
// declarations the program reaches), and all their type information.
func merge(pkgs []*Package, main *Package) *Package {
	m := &Package{
		Dir: main.Dir, Name: main.Name, GoVersion: main.GoVersion, Fset: main.Fset,
		Types: main.Types, Sizes: main.Sizes, Src: map[string][]byte{},
		Info:  newInfo(),
		PkgOf: map[*ast.File]*types.Package{},
		Local: map[*types.Package]bool{},
	}
	for _, p := range pkgs {
		m.Pkgs = append(m.Pkgs, p.Types)
		m.Local[p.Types] = true
		for k, v := range p.Src {
			m.Src[k] = v
		}
		mergeInfo(m.Info, p.Info)
	}
	keep, via := reach(pkgs, main, m.Info)
	m.Via = via
	for _, p := range pkgs {
		for _, f := range p.Files {
			if p != main {
				f = filtered(f, keep, m.Info)
			}
			m.Files = append(m.Files, f)
			m.PkgOf[f] = p.Types
		}
	}
	return m
}

func newInfo() *types.Info {
	return &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Implicits:  map[ast.Node]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
		Scopes:     map[ast.Node]*types.Scope{},
	}
}

func mergeInfo(to, from *types.Info) {
	for k, v := range from.Types {
		to.Types[k] = v
	}
	for k, v := range from.Defs {
		to.Defs[k] = v
	}
	for k, v := range from.Uses {
		to.Uses[k] = v
	}
	for k, v := range from.Implicits {
		to.Implicits[k] = v
	}
	for k, v := range from.Selections {
		to.Selections[k] = v
	}
	for k, v := range from.Scopes {
		to.Scopes[k] = v
	}
}

// reach returns the declarations of packages other than main that the
// program reaches: from main's declarations and every package's init
// functions, through the objects they use, and the String and Error
// methods of the types they reach, which fmt may call.
func reach(pkgs []*Package, main *Package, info *types.Info) (map[ast.Node]bool, map[ast.Node]ast.Node) {
	decl := map[types.Object]ast.Node{} // in packages other than main
	methods := map[*types.TypeName][]ast.Node{}
	var work []ast.Node
	for _, p := range pkgs {
		for _, f := range p.Files {
			for _, d := range f.Decls {
				if p == main {
					work = append(work, d)
					continue
				}
				switch d := d.(type) {
				case *ast.FuncDecl:
					obj := info.Defs[d.Name]
					decl[obj] = d
					if d.Recv == nil && d.Name.Name == "init" {
						work = append(work, d)
					}
					if d.Recv != nil && (d.Name.Name == "String" || d.Name.Name == "Error") {
						tn := recvName(info, d)
						methods[tn] = append(methods[tn], d)
					}
				case *ast.GenDecl:
					for _, s := range d.Specs {
						switch s := s.(type) {
						case *ast.TypeSpec:
							decl[info.Defs[s.Name]] = s
						case *ast.ValueSpec:
							for _, n := range s.Names {
								decl[info.Defs[n]] = s
							}
						}
					}
				}
			}
		}
	}
	keep := map[ast.Node]bool{}
	via := map[ast.Node]ast.Node{}
	for len(work) > 0 {
		n := work[len(work)-1]
		work = work[:len(work)-1]
		ast.Inspect(n, func(x ast.Node) bool {
			id, ok := x.(*ast.Ident)
			if !ok {
				return true
			}
			obj := info.Uses[id]
			if IsIntrinsic(obj) {
				return true // written as Rust, not translated
			}
			if d, ok := decl[obj]; ok && !keep[d] {
				keep[d] = true
				via[d] = n
				work = append(work, d)
				if tn, ok := obj.(*types.TypeName); ok {
					work = append(work, methods[tn]...)
					for _, m := range methods[tn] {
						keep[m] = true
						via[m] = d
					}
				}
			}
			return true
		})
		if fd, ok := n.(*ast.FuncDecl); ok {
			keep[fd] = true
		}
	}
	return keep, via
}

// Chain describes how the program reaches declaration d of pkg, a package
// other than main: "main -> a.F -> a.T", or "" for a declaration it does
// not reach through others (main's own, and init functions).
func Chain(pkg *Package, d ast.Node) string {
	var names []string
	for n, seen := d, map[ast.Node]bool{}; n != nil && !seen[n]; n = pkg.Via[n] {
		seen[n] = true
		names = append([]string{declName(pkg, n)}, names...)
	}
	if len(names) < 2 {
		return ""
	}
	return strings.Join(names, " -> ")
}

// declName names declaration n for Chain: pkg.Name, or pkg.Type.Method.
func declName(pkg *Package, n ast.Node) string {
	var id *ast.Ident
	prefix := ""
	switch n := n.(type) {
	case *ast.FuncDecl:
		id = n.Name
		if n.Recv != nil {
			prefix = recvName(pkg.Info, n).Name() + "."
		}
	case *ast.TypeSpec:
		id = n.Name
	case *ast.ValueSpec:
		id = n.Names[0]
	default: // a declaration of main other than a function: a type, a constant
		return "main"
	}
	obj := pkg.Info.Defs[id]
	return obj.Pkg().Name() + "." + prefix + id.Name
}

// recvName returns the named type of method d's receiver.
func recvName(info *types.Info, d *ast.FuncDecl) *types.TypeName {
	t := info.Types[d.Recv.List[0].Type].Type
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	return types.Unalias(t).(*types.Named).Obj() // a receiver is a named type
}

// filtered returns a copy of f with only the declarations in keep, and no
// imports: they are not code, and an import the reached code does not use
// is no reason to reject the program.
func filtered(f *ast.File, keep map[ast.Node]bool, info *types.Info) *ast.File {
	c := *f
	c.Decls = nil
	c.Imports = nil
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if keep[d] {
				c.Decls = append(c.Decls, d)
			}
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				continue
			}
			g := *d
			g.Specs = nil
			for _, s := range d.Specs {
				if keep[s] {
					g.Specs = append(g.Specs, s)
				}
			}
			if len(g.Specs) > 0 {
				c.Decls = append(c.Decls, &g)
			}
		}
	}
	return &c
}

// checkWith parses and type-checks the files of one package of a program,
// as import path path, with fset and imp shared by the program's packages.
func checkWith(dir, cwd, path, goVersion string, srcs map[string][]byte, fset *token.FileSet, imp types.Importer) (*Package, error) {
	pkg := &Package{Dir: dir, GoVersion: goVersion, Fset: fset, Src: srcs, Info: newInfo(), Sizes: targetSizes()}
	var paths []string
	for p := range srcs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var diags []diag.Diagnostic
	for _, p := range paths {
		f, err := parser.ParseFile(fset, p, srcs[p], parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			diags = append(diags, scanErrors(err)...)
			continue
		}
		pkg.Files = append(pkg.Files, f)
	}
	if len(diags) > 0 {
		return nil, &diag.Error{Dir: cwd, Diags: diags}
	}
	pkg.Name = pkg.Files[0].Name.Name
	conf := types.Config{
		Importer:  imp,
		GoVersion: goVersion,
		Sizes:     pkg.Sizes,
		Error: func(err error) {
			te := err.(types.Error)
			diags = append(diags, diag.Diagnostic{Pos: te.Fset.Position(te.Pos), Msg: te.Msg})
		},
	}
	pkg.Types, _ = conf.Check(path, fset, pkg.Files, pkg.Info)
	if len(diags) > 0 {
		return nil, &diag.Error{Dir: cwd, Diags: diags}
	}
	return pkg, nil
}
