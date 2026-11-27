package fma

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFuses(t *testing.T) {
	for _, c := range []struct {
		t    Target
		want bool
	}{
		{Target{GOARCH: "arm64"}, true},
		{Target{GOARCH: "amd64", GOAMD64: "v1"}, false},
		{Target{GOARCH: "amd64", GOAMD64: "v2"}, false},
		{Target{GOARCH: "amd64", GOAMD64: "v3"}, true},
		{Target{GOARCH: "amd64", GOAMD64: "v4"}, true},
		{Target{GOARCH: "amd64", GOAMD64: "x"}, false},
		{Target{GOARCH: "386"}, false},
	} {
		if got := c.t.Fuses(); got != c.want {
			t.Errorf("%v.Fuses() = %v", c.t, got)
		}
	}
	if s := (Target{"go1.27.1", "linux", "amd64", "v3"}).String(); s != "go1.27.1 linux/amd64 GOAMD64=v3" {
		t.Errorf("String() = %q", s)
	}
	if s := (Target{"go1.27.1", "darwin", "arm64", ""}).String(); s != "go1.27.1 darwin/arm64" {
		t.Errorf("String() = %q", s)
	}
}

// TestParseReport pins the parser to a report gc go1.27.1 printed for
// arm64 (testdata/inline.go.txt): madd's own + and its two inlined copies.
func TestParseReport(t *testing.T) {
	out, err := os.ReadFile("testdata/report-go1.27.1-arm64.txt")
	if err != nil {
		t.Fatal(err)
	}
	sites, err := Parse(out, "/src/p")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"/src/p/main.go:11:43; /src/p/main.go:8:49",
		"/src/p/main.go:11:59; /src/p/main.go:8:49",
		"/src/p/main.go:8:49",
	}
	if len(sites) != len(want) {
		t.Fatalf("sites %v", sites)
	}
	for i, s := range sites {
		if s.String() != want[i] {
			t.Errorf("site %d = %s, want %s", i, s, want[i])
		}
	}
	ss := &Sites{List: sites}
	if !ss.Has(Pos{"/src/p/main.go", 8, 49}) || ss.Has(Pos{"/src/p/main.go", 11, 43}) {
		t.Error("Has")
	}
	// A repeated line is one site; an absolute path stays.
	again, _ := Parse([]byte("fmahash triggered /g/x.go:1:2 01\nfmahash triggered /g/x.go:1:2 01\n"), "/src/p")
	if len(again) != 1 || again[0].String() != "/g/x.go:1:2" {
		t.Errorf("repeated: %v", again)
	}
}

func TestParseErrors(t *testing.T) {
	for _, out := range []string{
		"fmahash triggered nonsense\n",
		"fmahash triggered ./main.go:x:2 0101\n",
	} {
		if _, err := Parse([]byte(out), "/d"); err == nil {
			t.Errorf("Parse(%q) succeeded", out)
		}
	}
}

// fakeGo writes a go command that prints report and exits with code.
func fakeGo(t *testing.T, report string, code int) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script")
	}
	path := filepath.Join(t.TempDir(), "go")
	script := "#!/bin/sh\ncat <<'EOF'\n" + report + "EOF\nexit " + string(rune('0'+code)) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestFindFailsClosed checks that a report gc did not print as expected
// is an ErrOracle, never an empty list of sites.
func TestFindFailsClosed(t *testing.T) {
	arm := Target{"go1.27.1", "linux", "arm64", ""}
	for name, goCmd := range map[string]string{
		"no report":       fakeGo(t, "# probe\n", 0),
		"build fails":     fakeGo(t, "flag provided but not defined: -d=fmahash\n", 1),
		"format changed":  fakeGo(t, "fmahash triggered main.go line 7\n", 0),
		"wrong site":      fakeGo(t, "fmahash triggered ./main.go:7:12 0101\n", 0),
		"missing command": filepath.Join(t.TempDir(), "none"),
	} {
		if _, err := Find(goCmd, os.Environ(), t.TempDir(), arm); !errors.Is(err, ErrOracle) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestFindReal runs the Go toolchain's own report, cross-compiling for
// arm64 and for amd64 v3, on testdata/inline.go.txt.
func TestFindReal(t *testing.T) {
	goCmd, err := exec.LookPath("go")
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	src, _ := os.ReadFile("testdata/inline.go.txt")
	os.WriteFile(filepath.Join(dir, "main.go"), src, 0o644)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module inline\n\ngo 1.21\n"), 0o644)
	main := filepath.Join(dir, "main.go")
	for _, target := range []Target{{GOOS: "linux", GOARCH: "arm64"}, {GOOS: "linux", GOARCH: "amd64", GOAMD64: "v3"}} {
		sites, err := Find(goCmd, os.Environ(), dir, target)
		if err != nil {
			t.Fatalf("%v: %v", target, err)
		}
		var mine []string
		for _, s := range sites.List {
			if s.Pos.File == main {
				mine = append(mine, strings.ReplaceAll(s.String(), dir, "D"))
			}
		}
		if strings.Join(mine, ", ") != "D/main.go:11:43; D/main.go:8:49, D/main.go:11:59; D/main.go:8:49, D/main.go:8:49" {
			t.Errorf("%v: %v", target, mine)
		}
	}
	// A package that does not build has no report.
	bad := t.TempDir()
	os.WriteFile(filepath.Join(bad, "main.go"), []byte("package main\nfunc main() { x }\n"), 0o644)
	os.WriteFile(filepath.Join(bad, "go.mod"), []byte("module bad\n\ngo 1.21\n"), 0o644)
	if _, err := Find(goCmd, os.Environ(), bad, Target{GOOS: "linux", GOARCH: "arm64"}); !errors.Is(err, ErrOracle) {
		t.Errorf("broken package: %v", err)
	}
	// No directory for the probe.
	tmp := os.TempDir()
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "none"))
	if _, err := Find(goCmd, os.Environ(), dir, Target{GOOS: "darwin", GOARCH: "arm64"}); err == nil {
		t.Error("no TMPDIR: no error") // a target whose self-test has not passed yet
	}
	t.Setenv("TMPDIR", tmp)
	// amd64 v1 never fuses: its report on the probe is empty.
	if _, err := Find(goCmd, os.Environ(), dir, Target{GOOS: "linux", GOARCH: "amd64", GOAMD64: "v1"}); !errors.Is(err, ErrOracle) {
		t.Errorf("v1: %v", err)
	}
}
