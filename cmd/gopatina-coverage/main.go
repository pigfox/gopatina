// Command gopatina-coverage measures how much real Go code the patina
// subset accepts (docs/COVERAGE.md).
//
// usage:
//
//	gopatina-coverage resolve CORPUS >LOCK
//	gopatina-coverage measure [-work DIR] [-boot N] [-seed S] LOCK >REPORT.json
//
// A corpus file lists, one per line, "std" (the standard library of the Go
// toolchain that runs the command; "std PATTERN..." for some of its
// packages only) or a module path with a version or "latest"; # starts a
// comment. resolve pins each module to the version
// the Go proxy resolves now and records its go.sum hash and, when the proxy
// knows it, its VCS commit: the lock file is the corpus, reproducibly.
// measure lists every package of each locked module (and of std) with go
// list, type-checks it against the export data go list builds, runs
// GoPatina's subset checker on it, and prints a JSON report: every
// package, every function with what keeps it out, the packages it could
// not measure and why, and the summary with 95% bootstrap intervals.
// Nothing is built with Rust.
//
// stdlib asks TinyGo's two questions of every package in TABLE (TinyGo's
// published table, docs/stdlib/): can a program import it in patina mode,
// and does it pass its own tests (docs/COVERAGE.md). It builds with Rust.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/pigfox/gopatina/internal/coverage"
	"github.com/pigfox/gopatina/internal/load"
	"github.com/pigfox/gopatina/internal/subset"
)

var (
	exit  = os.Exit
	check = subset.Check // replaceable in tests
	// goCmd runs the go command in dir and returns its standard output.
	goCmd = func(dir string, args ...string) ([]byte, error) {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
		var errb bytes.Buffer
		cmd.Stderr = &errb
		out, err := cmd.Output()
		if err != nil {
			return out, fmt.Errorf("go %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
		}
		return out, nil
	}
)

func main() {
	exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usage = "usage: gopatina-coverage resolve CORPUS >LOCK\n       gopatina-coverage measure [-work DIR] [-boot N] [-seed S] LOCK >REPORT.json\n       gopatina-coverage stdlib [-gopatina CMD] [-work DIR] [-coverage REPORT] TABLE >REPORT.json\n"

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "resolve":
		if len(args) != 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := resolve(args[1], stdout); err != nil {
			fmt.Fprintln(stderr, "gopatina-coverage:", err)
			return 1
		}
		return 0
	case "measure":
		fs := flag.NewFlagSet("measure", flag.ContinueOnError)
		fs.SetOutput(stderr)
		work := fs.String("work", "", "working directory (default: a temporary one)")
		boot := fs.Int("boot", 2000, "bootstrap draws")
		seed := fs.Uint64("seed", 1, "bootstrap seed")
		if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 1 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := measure(fs.Arg(0), *work, *boot, *seed, stdout, stderr); err != nil {
			fmt.Fprintln(stderr, "gopatina-coverage:", err)
			return 1
		}
		return 0
	case "stdlib":
		fs := flag.NewFlagSet("stdlib", flag.ContinueOnError)
		fs.SetOutput(stderr)
		gopatina := fs.String("gopatina", "gopatina", "the gopatina command")
		work := fs.String("work", "", "working directory (default: a temporary one)")
		covReport := fs.String("coverage", "", "a measure report with the standard library, for each package's accepted functions")
		if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 1 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := stdlib(fs.Arg(0), *gopatina, *work, *covReport, stdout, stderr); err != nil {
			fmt.Fprintln(stderr, "gopatina-coverage:", err)
			return 1
		}
		return 0
	}
	fmt.Fprint(stderr, usage)
	return 2
}

// entry is one corpus or lock line.
type entry struct {
	Module, Version, Sum, Commit string
	Patterns                     []string `json:",omitempty"` // std only
}

// readList reads a corpus or lock file: fields per line, # comments.
func readList(path string) ([]entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var es []entry
	sc := bufio.NewScanner(bytes.NewReader(data))
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		f := strings.Fields(line)
		switch {
		case len(f) == 0:
			continue
		case f[0] == "std":
			es = append(es, entry{Module: "std", Patterns: f[1:]})
		case len(f) >= 2 && len(f) <= 4:
			e := entry{Module: f[0], Version: f[1]}
			if len(f) > 2 {
				e.Sum = f[2]
			}
			if len(f) > 3 {
				e.Commit = f[3]
			}
			es = append(es, e)
		default:
			return nil, fmt.Errorf("%s:%d: want \"std\" or \"module version [sum [commit]]\"", path, n)
		}
	}
	return es, nil
}

// resolve pins every module of the corpus.
func resolve(corpus string, out io.Writer) error {
	es, err := readList(corpus)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "gopatina-coverage-resolve-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	v, err := goCmd(tmp, "env", "GOVERSION")
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "# gopatina-coverage lock, resolved with %s; fields: module version go.sum-hash commit\n", strings.TrimSpace(string(v)))
	for _, e := range es {
		if e.Module == "std" {
			fmt.Fprintln(out, strings.Join(append([]string{"std"}, e.Patterns...), " "))
			continue
		}
		data, err := goCmd(tmp, "mod", "download", "-json", e.Module+"@"+e.Version)
		if err != nil {
			return err
		}
		var m struct {
			Version, Sum string
			Origin       struct{ Hash string }
		}
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("%s: %v", e.Module, err)
		}
		commit := m.Origin.Hash
		if commit == "" {
			commit = "-"
		}
		fmt.Fprintln(out, e.Module, m.Version, m.Sum, commit)
	}
	return nil
}

// listed is one package as go list -json prints it.
type listed struct {
	ImportPath string
	Dir        string
	Name       string
	GoFiles    []string
	CgoFiles   []string
	Export     string
	ImportMap  map[string]string
	DepOnly    bool
	Standard   bool
	Module     *struct{ Path, GoVersion string }
	Error      *struct{ Err string }
}

// skip records a package that could not be measured.
type skip struct {
	Path   string `json:"path"`
	Module string `json:"module"`
	Reason string `json:"reason"`
}

type report struct {
	Note     string             `json:"note"`
	Go       string             `json:"go"`
	GOOS     string             `json:"goos"`
	GOARCH   string             `json:"goarch"`
	Corpus   []entry            `json:"corpus"`
	Boot     int                `json:"bootstrap_draws"`
	Seed     uint64             `json:"bootstrap_seed"`
	Summary  coverage.Summary   `json:"summary"`
	Skipped  []skip             `json:"skipped"`
	Packages []coverage.Package `json:"packages"`
}

func measure(lock, work string, boot int, seed uint64, stdout, stderr io.Writer) error {
	es, err := readList(lock)
	if err != nil {
		return err
	}
	if work == "" {
		if work, err = os.MkdirTemp("", "gopatina-coverage-"); err != nil {
			return err
		}
		defer os.RemoveAll(work)
	}
	v, err := goCmd(work, "env", "GOVERSION")
	if err != nil {
		return err
	}
	r := report{
		Note: "patina subset coverage (cmd/gopatina-coverage, docs/COVERAGE.md); not for publication without review",
		Go:   strings.TrimSpace(string(v)), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Corpus: es, Boot: boot, Seed: seed,
	}
	for i, e := range es {
		dir := filepath.Join(work, fmt.Sprintf("m%d", i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		patterns := []string{"std"}
		if len(e.Patterns) > 0 {
			patterns = e.Patterns
		}
		if e.Module != "std" {
			patterns = []string{e.Module + "/..."}
			if _, err := goCmd(dir, "mod", "init", "gopatinacoverage"); err != nil {
				return err
			}
			if _, err := goCmd(dir, "get", e.Module+"@"+e.Version); err != nil {
				return err
			}
		}
		out, err := goCmd(dir, append([]string{"list", "-e", "-export", "-deps", "-json"}, patterns...)...)
		if err != nil {
			return err
		}
		pkgs, err := decodeList(out)
		if err != nil {
			return fmt.Errorf("%s: %v", e.Module, err)
		}
		exports := map[string]string{}
		for _, p := range pkgs {
			exports[p.ImportPath] = p.Export
		}
		measured := 0
		for _, p := range pkgs {
			if p.DepOnly || (e.Module == "std") != p.Standard {
				continue
			}
			res, why := one(p, e.Module, exports)
			if why != "" {
				r.Skipped = append(r.Skipped, skip{Path: p.ImportPath, Module: e.Module, Reason: why})
				continue
			}
			r.Packages = append(r.Packages, res)
			measured++
		}
		fmt.Fprintf(stderr, "%s: %d packages measured\n", e.Module, measured)
	}
	coverage.Resolve(r.Packages)
	r.Summary = coverage.Summarize(r.Packages, boot, seed)
	out, _ := json.MarshalIndent(r, "", " ")
	fmt.Fprintf(stdout, "%s\n", out)
	a := r.Summary.All.Functions
	fmt.Fprintf(stderr, "functions accepted: %d of %d (%.4f, 95%% bootstrap interval %.4f to %.4f); %d packages skipped\n",
		a.Num, a.Den, a.Value, a.Low, a.High, len(r.Skipped))
	return nil
}

// decodeList decodes go list -json's stream of objects.
func decodeList(data []byte) ([]listed, error) {
	var ps []listed
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var p listed
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			return ps, nil
		} else if err != nil {
			return nil, err
		}
		ps = append(ps, p)
	}
}

// one measures one package, or says why it cannot.
func one(p listed, module string, exports map[string]string) (res coverage.Package, why string) {
	switch {
	case strings.HasPrefix(p.ImportPath, "vendor/") || strings.Contains(p.ImportPath, "/vendor/"):
		return res, "vendored"
	case p.Error != nil:
		return res, "go list: " + p.Error.Err
	case len(p.CgoFiles) > 0:
		return res, "cgo"
	case len(p.GoFiles) == 0:
		return res, "no Go files for this platform"
	}
	pkg := &load.Package{Dir: p.Dir, Name: "main", Fset: token.NewFileSet(), Src: map[string][]byte{}}
	for _, name := range p.GoFiles {
		path := filepath.Join(p.Dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			return res, err.Error()
		}
		f, err := parser.ParseFile(pkg.Fset, path, src, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return res, "parse: " + err.Error()
		}
		// Measured as package main: the subset supports only package main
		// today, and treats a named type of any other package as foreign,
		// so a library is checked as if it were the program it could be.
		f.Name.Name = "main"
		pkg.Src[path] = src
		pkg.Files = append(pkg.Files, f)
	}
	sort.Slice(pkg.Files, func(i, j int) bool {
		return pkg.Fset.Position(pkg.Files[i].Pos()).Filename < pkg.Fset.Position(pkg.Files[j].Pos()).Filename
	})
	if p.Module != nil && p.Module.GoVersion != "" {
		pkg.GoVersion = "go" + p.Module.GoVersion
	}
	pkg.Info = &types.Info{
		Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{},
		Implicits: map[ast.Node]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}, Scopes: map[ast.Node]*types.Scope{},
	}
	pkg.Sizes = types.SizesFor("gc", runtime.GOARCH)
	gc := importer.ForCompiler(pkg.Fset, "gc", func(path string) (io.ReadCloser, error) {
		if exports[path] == "" {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(exports[path])
	})
	var typeErr error
	conf := types.Config{
		Importer:  mapped{gc, p.ImportMap},
		GoVersion: pkg.GoVersion,
		Sizes:     pkg.Sizes,
		Error: func(err error) {
			if typeErr == nil {
				typeErr = err
			}
		},
	}
	pkg.Types, _ = conf.Check(p.ImportPath, pkg.Fset, pkg.Files, pkg.Info)
	if typeErr != nil {
		return res, "type-check: " + typeErr.Error()
	}
	// The packages it imports, other than the bridged ones, are packages of
	// the program (RFC 0009): the checker accepts their types and calls,
	// and coverage.Resolve decides by what they are.
	pkg.Pkgs = []*types.Package{pkg.Types}
	pkg.Local = map[*types.Package]bool{pkg.Types: true}
	var mark func(*types.Package)
	mark = func(t *types.Package) {
		for _, imp := range t.Imports() {
			// reflect and unsafe stay reported, so the functions that use
			// them stay excluded; a provided package (RFC 0011) is decided
			// by its API, and nothing below it is reached.
			if !pkg.Local[imp] && !load.Provided(imp.Path()) && imp.Path() != "unsafe" && imp.Path() != "reflect" {
				pkg.Local[imp] = true
				mark(imp)
			}
		}
	}
	mark(pkg.Types)
	pkg.PkgOf = map[*ast.File]*types.Package{}
	for _, f := range pkg.Files {
		pkg.PkgOf[f] = pkg.Types
	}
	defer func() {
		if v := recover(); v != nil {
			why = fmt.Sprintf("subset checker or measurement panicked at %s: %v", panicSite(debug.Stack()), v)
		}
	}()
	res = coverage.Measure(pkg, p.ImportPath, module, check(pkg))
	res.Provided = load.Provided(p.ImportPath)
	return res, ""
}

// panicSite returns file:line of the innermost GoPatina frame of a panic's
// stack (from debug.Stack inside the deferred recover), or "unknown".
func panicSite(stack []byte) string {
	lines := strings.Split(string(stack), "\n")
	for i, l := range lines {
		if !strings.HasPrefix(l, "\t") || i == 0 {
			continue
		}
		fn := lines[i-1]
		if !strings.Contains(fn, "gopatina/internal/") {
			continue
		}
		loc := strings.Fields(strings.TrimSpace(l))[0]
		return fn[:strings.LastIndex(fn, "(")] + " " + filepath.Base(loc)
	}
	return "unknown"
}

// mapped resolves import paths through go list's ImportMap (vendored
// packages) before importing them.
type mapped struct {
	types.Importer
	m map[string]string
}

func (m mapped) Import(path string) (*types.Package, error) {
	if to, ok := m.m[path]; ok {
		path = to
	}
	return m.Importer.Import(path)
}
