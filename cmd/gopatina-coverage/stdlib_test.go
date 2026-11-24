package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/coverage"
)

const table = `# TinyGo's table, for the test.
archive/tar yes yes
bufio yes no

plugin no no
`

// fakeBuild makes buildRun fail for the packages in fail, with their
// messages, and records the programs it was given.
func fakeBuild(t *testing.T, fail map[string]string) *[]string {
	t.Helper()
	old := buildRun
	t.Cleanup(func() { buildRun = old })
	var seen []string
	buildRun = func(gopatina, dir string) string {
		src, err := os.ReadFile(filepath.Join(dir, "main.go"))
		if err != nil {
			t.Fatal(err)
		}
		seen = append(seen, gopatina+": "+string(src))
		for p, msg := range fail {
			if strings.Contains(string(src), `"`+p+`"`) {
				return msg
			}
		}
		return ""
	}
	return &seen
}

func TestStdlib(t *testing.T) {
	seen := fakeBuild(t, map[string]string{"plugin": "package plugin is not supported"})
	tab := write(t, "table", table)
	cr := report{Packages: []coverage.Package{
		{Path: "bufio", Accepted: true, Funcs: []coverage.Func{{Name: "A", Accepted: true}, {Name: "B"}}},
	}}
	data, _ := json.Marshal(cr)
	covFile := write(t, "cov.json", string(data))
	code, out, errs := cov("stdlib", "-gopatina", "gp", "-work", t.TempDir(), "-coverage", covFile, tab)
	if code != 0 {
		t.Fatalf("code %d: %s", code, errs)
	}
	var r stdReport
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	s := r.Summary
	if s.Packages != 3 || s.Importable != 2 || s.AllAccepted != 1 || s.Tests != 0 || s.TinyGoImportable != 2 || s.TinyGoTests != 1 {
		t.Errorf("summary %+v", s)
	}
	if len(r.Table) != 1 || !strings.Contains(r.Table[0], "TinyGo's table") {
		t.Errorf("table comments %q", r.Table)
	}
	b := r.Packages[1]
	if b.Path != "bufio" || !b.Importable || b.Funcs != 2 || b.FuncsAccepted != 1 || !b.AllAccepted || b.TestsWhy != noTestMode {
		t.Errorf("bufio %+v", b)
	}
	if p := r.Packages[2]; p.Importable || p.Why != "package plugin is not supported" || p.TinyGoImportable {
		t.Errorf("plugin %+v", p)
	}
	if len(*seen) != 3 || !strings.HasPrefix((*seen)[0], "gp: package main\n\nimport _ \"archive/tar\"\n") {
		t.Errorf("programs %q", *seen)
	}
	if !strings.Contains(errs, "plugin: importable false") {
		t.Errorf("progress %q", errs)
	}
}

func TestStdlibErrors(t *testing.T) {
	fakeBuild(t, nil)
	dir := t.TempDir()
	bad := write(t, "bad", "fmt yes maybe\n")
	notJSON := write(t, "cov", "{")
	tab := write(t, "table", table)
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"stdlib"}, 2, "usage"},
		{[]string{"stdlib", "-nope", tab}, 2, "usage"},
		{[]string{"stdlib", filepath.Join(dir, "missing")}, 1, "no such file"},
		{[]string{"stdlib", bad}, 1, `want "path yes|no yes|no", got "fmt yes maybe"`},
		{[]string{"stdlib", "-coverage", filepath.Join(dir, "missing"), tab}, 1, "no such file"},
		{[]string{"stdlib", "-coverage", notJSON, tab}, 1, "unexpected end of JSON"},
		{[]string{"stdlib", "-work", write(t, "file", ""), tab}, 1, "not a directory"},
	} {
		code, _, errs := cov(tc.args...)
		if code != tc.code || !strings.Contains(errs, tc.want) {
			t.Errorf("%v: code %d, %q; want %d, %q", tc.args, code, errs, tc.code, tc.want)
		}
	}
}

// TestStdlibTempWork measures in a temporary directory, removed after.
func TestStdlibTempWork(t *testing.T) {
	fakeBuild(t, nil)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	if code, _, errs := cov("stdlib", write(t, "table", table)); code != 0 {
		t.Fatalf("code %d: %s", code, errs)
	}
	if left, _ := filepath.Glob(filepath.Join(tmp, "gopatina-stdlib-*")); len(left) != 0 {
		t.Errorf("left behind: %v", left)
	}
	t.Setenv("TMPDIR", filepath.Join(tmp, "missing"))
	if code, _, errs := cov("stdlib", write(t, "table", table)); code != 1 || !strings.Contains(errs, "no such file") {
		t.Errorf("code %d: %s", code, errs)
	}
}

// TestBuildRun builds and runs real programs with a fake gopatina: one
// that builds a working binary, one whose build fails and one whose
// binary fails.
func TestBuildRun(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	dir := t.TempDir()
	fake := func(name, script string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	ok := fake("ok", "printf '#!/bin/sh\\nexit 0\\n' > \"$4\"; chmod +x \"$4\"\n")
	buildFails := fake("buildfails", "echo; echo './main.go:3:8: package x is not supported'; exit 1\n")
	runFails := fake("runfails", "printf '#!/bin/sh\\nexit 3\\n' > \"$4\"; chmod +x \"$4\"\n")
	for _, tc := range []struct{ gopatina, want string }{
		{ok, ""},
		{buildFails, "./main.go:3:8: package x is not supported"},
		{runFails, "exit status 3"},
	} {
		prog := t.TempDir()
		if got := buildRun(tc.gopatina, prog); got != tc.want {
			t.Errorf("%s: %q, want %q", filepath.Base(tc.gopatina), got, tc.want)
		}
	}
	if got := firstLine("\n\n", errors.New("e")); got != "e" {
		t.Errorf("firstLine: %q", got)
	}
}
