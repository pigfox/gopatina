package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/diag"
	"github.com/pigfox/gopatina/internal/load"
)

func cov(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func write(t *testing.T, name, data string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// fakeGo answers go commands from a table keyed by their first arguments.
func fakeGo(t *testing.T, answers map[string]string, fail string) {
	t.Helper()
	old := goCmd
	t.Cleanup(func() { goCmd = old })
	goCmd = func(dir string, args ...string) ([]byte, error) {
		key := strings.Join(args, " ")
		if fail != "" && strings.HasPrefix(key, fail) {
			return nil, errors.New("failed: " + key)
		}
		for k, v := range answers {
			if strings.HasPrefix(key, k) {
				return []byte(v), nil
			}
		}
		return nil, nil
	}
}

// TestMeasureStd measures two small standard-library packages for real:
// go list, export data, type-checking and the subset checker.
func TestMeasureStd(t *testing.T) {
	lock := write(t, "lock", "# a comment\nstd errors unicode/utf8 # inline comment\n")
	code, out, errs := cov("measure", "-boot", "20", "-work", t.TempDir(), lock)
	if code != 0 {
		t.Fatalf("code %d: %s", code, errs)
	}
	var r report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Packages) != 2 || r.Summary.All.Functions.Den == 0 || !strings.Contains(errs, "functions accepted:") {
		t.Errorf("report: %d packages, %+v; stderr %s", len(r.Packages), r.Summary.All.Functions, errs)
	}
}

func TestResolve(t *testing.T) {
	fakeGo(t, map[string]string{
		"env GOVERSION":                     "go1.27.1\n",
		"mod download -json a.org/m@latest": `{"Version":"v1.2.3","Sum":"h1:abc=","Origin":{"Hash":"deadbeef"}}`,
		"mod download -json b.org/n@v0.1.0": `{"Version":"v0.1.0","Sum":"h1:def="}`,
	}, "")
	corpus := write(t, "corpus", "std\nstd strings\na.org/m latest\nb.org/n v0.1.0\n")
	code, out, errs := cov("resolve", corpus)
	want := "std\nstd strings\na.org/m v1.2.3 h1:abc= deadbeef\nb.org/n v0.1.0 h1:def= -\n"
	if code != 0 || !strings.HasSuffix(out, want) || !strings.HasPrefix(out, "# gopatina-coverage lock, resolved with go1.27.1") {
		t.Errorf("code %d, out %q, stderr %s", code, out, errs)
	}
}

func TestResolveErrors(t *testing.T) {
	corpus := write(t, "corpus", "a.org/m latest\n")
	fakeGo(t, map[string]string{"env": "go1\n", "mod download": "not json"}, "")
	if code, _, errs := cov("resolve", corpus); code != 1 || !strings.Contains(errs, "a.org/m") {
		t.Errorf("bad json: %d %s", code, errs)
	}
	fakeGo(t, map[string]string{"env": "go1\n"}, "mod download")
	if code, _, _ := cov("resolve", corpus); code != 1 {
		t.Error("download failure")
	}
	fakeGo(t, nil, "env")
	if code, _, _ := cov("resolve", corpus); code != 1 {
		t.Error("env failure")
	}
	if code, _, _ := cov("resolve", filepath.Join(t.TempDir(), "none")); code != 1 {
		t.Error("missing corpus")
	}
	if code, _, errs := cov("resolve", write(t, "bad", "a b c d e\n")); code != 1 || !strings.Contains(errs, "bad:1:") {
		t.Errorf("bad line: %d %s", code, errs)
	}
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if code, _, _ := cov("resolve", corpus); code != 1 {
		t.Error("no temp dir")
	}
}

func TestUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"what"}, {"resolve"}, {"measure"}, {"measure", "-bogus", "x"}} {
		if code, _, errs := cov(args...); code != 2 || !strings.Contains(errs, "usage") {
			t.Errorf("%v: %d %q", args, code, errs)
		}
	}
}

// listJSON is a go list stream for a module with every kind of package
// the measurement skips, and one it measures.
func listJSON(t *testing.T) string {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nfunc F() int { return 1 }\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "bad.go"), []byte("package a\n\nfunc {\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "typo.go"), []byte("package a\n\nfunc G() int { return undefined }\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "imp.go"), []byte("package a\n\nimport \"zz/none\"\n\nvar _ = none.X\n"), 0o644)
	ps := []listed{
		{ImportPath: "a.org/m/ok", Dir: dir, Name: "a", GoFiles: []string{"a.go"}, Module: &struct{ Path, GoVersion string }{"a.org/m", "1.25"}},
		{ImportPath: "a.org/m/vendor/x", Dir: dir, GoFiles: []string{"a.go"}},
		{ImportPath: "a.org/m/broken", Dir: dir, Error: &struct{ Err string }{"no such package"}},
		{ImportPath: "a.org/m/cgo", Dir: dir, GoFiles: []string{"a.go"}, CgoFiles: []string{"c.go"}},
		{ImportPath: "a.org/m/empty", Dir: dir},
		{ImportPath: "a.org/m/missing", Dir: dir, GoFiles: []string{"gone.go"}},
		{ImportPath: "a.org/m/parse", Dir: dir, GoFiles: []string{"bad.go"}},
		{ImportPath: "a.org/m/types", Dir: dir, GoFiles: []string{"typo.go"}},
		{ImportPath: "a.org/m/imp", Dir: dir, GoFiles: []string{"imp.go"}},
		{ImportPath: "fmt", Standard: true, DepOnly: true},
		{ImportPath: "a.org/m/dep", DepOnly: true},
	}
	var b strings.Builder
	for _, p := range ps {
		j, _ := json.Marshal(p)
		b.Write(j)
		b.WriteString("\n")
	}
	return b.String()
}

func TestMeasureModuleSkips(t *testing.T) {
	fakeGo(t, map[string]string{"env": "go1.27.1\n", "list": listJSON(t)}, "")
	lock := write(t, "lock", "a.org/m v1.0.0 h1:x= abc\n")
	code, out, errs := cov("measure", "-boot", "10", lock)
	if code != 0 {
		t.Fatalf("code %d: %s", code, errs)
	}
	var r report
	json.Unmarshal([]byte(out), &r)
	reasons := map[string]string{}
	for _, s := range r.Skipped {
		reasons[s.Path] = s.Reason
	}
	for path, want := range map[string]string{
		"a.org/m/vendor/x": "vendored", "a.org/m/broken": "go list: no such package", "a.org/m/cgo": "cgo",
		"a.org/m/empty": "no Go files", "a.org/m/missing": "no such file", "a.org/m/parse": "parse:", "a.org/m/types": "type-check:", "a.org/m/imp": "no export data for zz/none",
	} {
		if !strings.Contains(reasons[path], want) {
			t.Errorf("%s skipped as %q, want %q", path, reasons[path], want)
		}
	}
	if len(r.Packages) != 1 || r.Packages[0].Path != "a.org/m/ok" || !r.Packages[0].Funcs[0].Accepted || r.Corpus[0].Commit != "abc" {
		t.Errorf("measured %+v", r.Packages)
	}
}

func TestMeasureErrors(t *testing.T) {
	lock := write(t, "lock", "a.org/m v1.0.0\n")
	for _, fail := range []string{"env", "mod init", "get", "list"} {
		fakeGo(t, map[string]string{"env": "go1\n", "list": "{}"}, fail)
		if code, _, errs := cov("measure", lock); code != 1 || !strings.Contains(errs, "failed: "+fail) {
			t.Errorf("%s: %d %s", fail, code, errs)
		}
	}
	fakeGo(t, map[string]string{"env": "go1\n", "list": "{not json"}, "")
	if code, _, errs := cov("measure", lock); code != 1 || !strings.Contains(errs, "a.org/m:") {
		t.Errorf("bad list json: %d %s", code, errs)
	}
	if code, _, _ := cov("measure", filepath.Join(t.TempDir(), "none")); code != 1 {
		t.Error("missing lock")
	}
	file := write(t, "file", "")
	if code, _, _ := cov("measure", "-work", file, lock); code != 1 {
		t.Error("work is a file")
	}
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if code, _, _ := cov("measure", lock); code != 1 {
		t.Error("no temp dir")
	}
}

func TestCheckerPanic(t *testing.T) {
	old := check
	t.Cleanup(func() { check = old })
	check = func(*load.Package) []diag.Diagnostic { panic("boom") }
	fakeGo(t, map[string]string{"env": "go1\n", "list": listJSON(t)}, "")
	code, out, _ := cov("measure", "-boot", "1", write(t, "lock", "a.org/m v1.0.0\n"))
	if code != 0 || !strings.Contains(out, "subset checker or measurement panicked at ") || !strings.Contains(out, ": boom") {
		t.Errorf("code %d, out %s", code, out)
	}
}

func TestPanicSite(t *testing.T) {
	stack := "goroutine 1 [running]:\nruntime/debug.Stack()\n\t/go/src/runtime/debug/stack.go:26 +0x5e\n" +
		"github.com/pigfox/gopatina/internal/subset.(*checker).stmt(0xc0, {0x1})\n\t/src/gopatina/internal/subset/subset.go:501 +0x2a\n" +
		"github.com/pigfox/gopatina/internal/subset.Check(0x1)\n\t/src/gopatina/internal/subset/subset.go:40 +0x2a\n"
	if got := panicSite([]byte(stack)); got != "github.com/pigfox/gopatina/internal/subset.(*checker).stmt subset.go:501" {
		t.Errorf("panicSite = %q", got)
	}
	if got := panicSite([]byte("goroutine 1 [running]:\nmain.f()\n\t/x/main.go:3 +0x1\n")); got != "unknown" {
		t.Errorf("panicSite = %q", got)
	}
}

func TestMapped(t *testing.T) {
	m := mapped{fakeImporter{}, map[string]string{"x": "vendor/x"}}
	if p, _ := m.Import("x"); p != nil || seen != "vendor/x" {
		t.Errorf("mapped to %q", seen)
	}
	m.Import("y")
	if seen != "y" {
		t.Errorf("unmapped to %q", seen)
	}
}

func TestMainExit(t *testing.T) {
	oldExit, oldArgs, oldErr := exit, os.Args, os.Stderr
	t.Cleanup(func() { exit, os.Args, os.Stderr = oldExit, oldArgs, oldErr })
	devnull, _ := os.Open(os.DevNull)
	defer devnull.Close()
	os.Stderr = devnull
	got := -1
	exit = func(c int) { got = c }
	os.Args = []string{"gopatina-coverage"}
	main()
	if got != 2 {
		t.Errorf("exit %d", got)
	}
}

var seen string

type fakeImporter struct{}

func (fakeImporter) Import(p string) (*types.Package, error) {
	seen = p
	return nil, nil
}

var realGo = goCmd

func TestGoCmdError(t *testing.T) {
	if _, err := realGo(t.TempDir(), "no-such-subcommand"); err == nil || !strings.Contains(err.Error(), "go no-such-subcommand") {
		t.Errorf("err = %v", err)
	}
}
