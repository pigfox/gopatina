package borrow

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/diag"
	"github.com/pigfox/gopatina/internal/load"
	"github.com/pigfox/gopatina/internal/subset"
)

var update = flag.Bool("update", false, "rewrite testdata/*/*/explain.txt and testdata/reject/*/want.txt")

func programs(t *testing.T, kind string) []string {
	_, file, _, _ := runtime.Caller(0)
	dirs, err := filepath.Glob(filepath.Join(filepath.Dir(file), "..", "..", "testdata", kind, "*"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no programs under testdata/%s: %v", kind, err)
	}
	sort.Strings(dirs)
	return dirs
}

// run loads the package in dir and returns the -explain report and the
// diagnostics, as gopatina prints them from inside dir.
func run(t *testing.T, dir string) (explain, errs string) {
	t.Helper()
	pkg, err := load.Dir(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if ds := subset.Check(pkg); len(ds) > 0 {
		return "", (&diag.Error{Dir: dir, Diags: ds}).Error() + "\n"
	}
	res, ds := Check(pkg)
	var b strings.Builder
	for _, f := range res.Fixes {
		fmt.Fprintf(&b, "%s:%d:%d: %s\n", diag.RelPath(dir, f.Pos.Filename), f.Pos.Line, f.Pos.Column, f.Text)
	}
	if len(ds) > 0 {
		errs = (&diag.Error{Dir: dir, Diags: ds}).Error() + "\n"
	}
	return b.String(), errs
}

func compare(t *testing.T, path, got string) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test ./internal/borrow -update)", err)
	}
	if got != string(want) {
		t.Errorf("%s differs; run go test ./internal/borrow -update and review the diff\n--- got:\n%s--- want:\n%s", path, got, want)
	}
}

// TestExplain checks the -explain report of every golden program: each
// move, copy and borrow the borrow checker decided on its own.
func TestExplain(t *testing.T) {
	for _, dir := range programs(t, "golden") {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			explain, errs := run(t, dir)
			if errs != "" {
				t.Fatalf("a golden program is rejected:\n%s", errs)
			}
			compare(t, filepath.Join(dir, "explain.txt"), explain)
		})
	}
}

// TestRejected checks the exact Go-style errors for every program under
// testdata/reject, which patina mode must not build.
func TestRejected(t *testing.T) {
	for _, dir := range programs(t, "reject") {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			_, errs := run(t, dir)
			if errs == "" {
				t.Fatal("the program is accepted")
			}
			compare(t, filepath.Join(dir, "want.txt"), errs)
		})
	}
}

// TestOwnershipExample checks the error EXAMPLES.md documents for
// examples/ownership.
func TestOwnershipExample(t *testing.T) {
	for _, dir := range programs(t, "reject") {
		if filepath.Base(dir) != "ownership" {
			continue
		}
		_, errs := run(t, dir)
		want := "./main.go:33:2: m.Body is changed after m was handed to out.Queue on line 32\n"
		if !strings.HasPrefix(errs, want) {
			t.Errorf("errors = %q, want them to start with %q", errs, want)
		}
		return
	}
	t.Fatal("testdata/reject/ownership is missing")
}

func TestResultParam(t *testing.T) {
	pkg, err := load.Source("/p", "main.go", "package main\nfunc f(s string) int { return len(s) }\nfunc main() { f(\"x\") }\n", "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	res, _ := Check(pkg)
	for _, fn := range res.Funcs {
		for _, p := range fn.Params {
			if got := res.Param(p.Var); got != p || got.Mode != Ref {
				t.Errorf("Param(%s) = %+v", p.Var.Name(), got)
			}
		}
	}
	if res.Param(nil) != nil {
		t.Error("Param(nil) is not nil")
	}
}
