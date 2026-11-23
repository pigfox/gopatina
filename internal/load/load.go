// Package load reads one Go package for the patina modes: it selects the
// package's files with the patina build tag set, parses them with go/parser
// and type-checks them with go/types, at the Go language version named by
// the module's go.mod.
package load

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/pigfox/gopatina/internal/diag"
)

// BuildTag is the build tag both patina flags set, so a package can carry a
// patina implementation (//go:build patina) beside a plain one.
const BuildTag = "patina"

// A Package is a parsed and type-checked Go package.
type Package struct {
	Dir  string // absolute directory of the package
	Name string // package name, from the package clause
	// ImportPath is the package's import path, when it was loaded as part
	// of a module ("" for a package loaded from source alone).
	ImportPath string
	GoVersion  string // language version, such as "go1.25"; empty if no go.mod
	Fset       *token.FileSet
	Files      []*ast.File // sorted by file name
	Types      *types.Package
	Info       *types.Info
	Sizes      types.Sizes       // word sizes of the GOOS/GOARCH target
	Src        map[string][]byte // file contents, by absolute file name

	// A program of several packages (RFC 0009) is loaded as one Package:
	// Files and Info hold every translated package's, and these say which
	// is which. For one package, Pkgs is just Types.
	Pkgs  []*types.Package             // translated packages, in initialization order, main last
	PkgOf map[*ast.File]*types.Package // the package of each file
	Local map[*types.Package]bool      // the translated packages
	// Via records, for each declaration of a package other than main that
	// the program reaches, the declaration that first reached it (an init
	// function and main's declarations reach without one).
	Via map[ast.Node]ast.Node
}

// stdImporter imports standard-library packages from export data. It is
// shared, so repeated loads do not import fmt and its dependencies again.
var (
	importerMu  sync.Mutex
	stdImporter = importer.Default()
)

type lockedImporter struct{}

func (lockedImporter) Import(path string) (*types.Package, error) {
	importerMu.Lock()
	defer importerMu.Unlock()
	return stdImporter.Import(path)
}

// Dir loads the package in directory dir. Errors that point at Go source
// (syntax and type errors) are returned as a *diag.Error, with file names
// printed relative to cwd.
func Dir(dir, cwd string) (*Package, error) {
	abs := dir
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(cwd, dir)
	}
	ctx := build.Default
	ctx.BuildTags = append(append([]string(nil), ctx.BuildTags...), BuildTag)
	ctx.CgoEnabled = true
	bp, err := ctx.ImportDir(abs, 0)
	if err != nil {
		var noGo *build.NoGoError
		if errors.As(err, &noGo) {
			return nil, fmt.Errorf("no Go files in %s", diag.RelPath(cwd, abs))
		}
		return nil, err
	}
	if len(bp.CgoFiles) > 0 {
		return nil, fmt.Errorf("%s: cgo (import \"C\") is not supported in patina mode", diag.RelPath(cwd, filepath.Join(abs, bp.CgoFiles[0])))
	}
	for _, imp := range bp.Imports {
		if !Provided(imp) && imp != "unsafe" {
			return program(abs, cwd) // a program of several packages
		}
	}
	goVersion, srcs, err := sources(abs, bp.GoFiles)
	if err != nil {
		return nil, err
	}
	pkg, err := check(abs, cwd, goVersion, srcs)
	if err != nil {
		return nil, err
	}
	pkg.ImportPath = moduleImportPath(abs)
	return pkg, nil
}

// moduleImportPath returns the import path of the package in dir, from the
// module path of the go.mod above it, or "" outside a module.
func moduleImportPath(dir string) string {
	for d := dir; ; {
		data, err := os.ReadFile(filepath.Join(d, "go.mod"))
		if err == nil {
			mod := moduleDirective(data)
			rel, _ := filepath.Rel(d, dir) // both absolute, d above dir
			if mod == "" || rel == "." {
				return mod
			}
			return mod + "/" + filepath.ToSlash(rel)
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

// moduleDirective extracts the module path from a go.mod file's "module"
// line.
func moduleDirective(data []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		if fields := strings.Fields(sc.Text()); len(fields) >= 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`)
		}
	}
	return ""
}

// sources reads the named files in dir, and the language version of the
// module dir is in.
func sources(dir string, names []string) (string, map[string][]byte, error) {
	goVersion, err := moduleGoVersion(dir)
	if err != nil {
		return "", nil, err
	}
	srcs := map[string][]byte{}
	for _, name := range names {
		path := filepath.Join(dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			return "", nil, err
		}
		srcs[path] = src
	}
	return goVersion, srcs, nil
}

// Source loads a package of one file, named name in directory dir, from
// src in memory, at language version goVersion (such as "go1.25"). It is
// for tests and fuzzing.
func Source(dir, name, src, goVersion string) (*Package, error) {
	return check(dir, dir, goVersion, map[string][]byte{filepath.Join(dir, name): []byte(src)})
}

// targetSizes returns the go/types sizes for the GOARCH gopatina is
// building for. An unset or unknown GOARCH falls back to the host, which is
// what go/types does when no sizes are configured.
func targetSizes() types.Sizes {
	arch := os.Getenv("GOARCH")
	if arch == "" {
		arch = runtime.GOARCH
	}
	if sizes := types.SizesFor("gc", arch); sizes != nil {
		return sizes
	}
	return types.SizesFor("gc", runtime.GOARCH)
}

// check parses and type-checks the files of a package.
func check(dir, cwd, goVersion string, srcs map[string][]byte) (*Package, error) {
	pkg := &Package{Dir: dir, GoVersion: goVersion, Fset: token.NewFileSet(), Src: srcs}
	var paths []string
	for p := range srcs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var diags []diag.Diagnostic
	for _, path := range paths {
		f, err := parser.ParseFile(pkg.Fset, path, srcs[path], parser.ParseComments|parser.SkipObjectResolution)
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
	pkg.Info = &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Implicits:  map[ast.Node]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
		Scopes:     map[ast.Node]*types.Scope{},
	}
	pkg.Sizes = targetSizes()
	conf := types.Config{
		Importer:  lockedImporter{},
		GoVersion: goVersion,
		Sizes:     pkg.Sizes,
		Error: func(err error) {
			te := err.(types.Error)
			diags = append(diags, diag.Diagnostic{Pos: te.Fset.Position(te.Pos), Msg: te.Msg})
		},
	}
	pkg.Types, _ = conf.Check(pkg.Name, pkg.Fset, pkg.Files, pkg.Info)
	if len(diags) > 0 {
		return nil, &diag.Error{Dir: cwd, Diags: diags}
	}
	pkg.Pkgs = []*types.Package{pkg.Types}
	pkg.Local = map[*types.Package]bool{pkg.Types: true}
	pkg.PkgOf = map[*ast.File]*types.Package{}
	for _, f := range pkg.Files {
		pkg.PkgOf[f] = pkg.Types
	}
	return pkg, nil
}

// scanErrors converts a go/parser error into diagnostics. With the source
// already read, ParseFile returns only a scanner.ErrorList.
func scanErrors(err error) []diag.Diagnostic {
	list := err.(scanner.ErrorList)
	ds := make([]diag.Diagnostic, len(list))
	for i, e := range list {
		ds[i] = diag.Diagnostic{Pos: e.Pos, Msg: e.Msg}
	}
	return ds
}

// moduleGoVersion returns the language version from the go directive of
// the go.mod file that governs dir, as "go1.N", or "" if there is none.
func moduleGoVersion(dir string) (string, error) {
	for d := dir; ; {
		data, err := os.ReadFile(filepath.Join(d, "go.mod"))
		if err == nil {
			return goDirective(data), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", nil
		}
		d = parent
	}
}

// goDirective extracts the version from a go.mod file's "go" line.
func goDirective(data []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 && fields[0] == "go" {
			return "go" + fields[1]
		}
	}
	return ""
}
