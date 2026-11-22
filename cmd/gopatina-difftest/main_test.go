package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// fakeTool writes a stand-in for go or gopatina: "build ... -o OUT ." either
// fails (if fail) or writes OUT, a program that prints the batch's markers,
// each followed by line, then runs tail.
func fakeTool(t *testing.T, name string, fail bool, line, tail string) string {
	t.Helper()
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip(err)
	}
	path := filepath.Join(t.TempDir(), name)
	failCmd := ""
	if fail {
		failCmd = "echo build broke; exit 1"
	}
	script := `#!/bin/sh
while [ $# -gt 0 ]; do if [ "$1" = -o ]; then out="$2"; fi; shift; done
` + failCmd + `
grep -o 'fmt.Println("#[0-9]*")' main.go | sed 's/.*"\(#[0-9]*\)".*/\1/' > "$out.markers"
{
  echo '#!/bin/sh'
  echo "while read m; do echo \"\$m\"; echo '` + line + `'; done < '$out.markers'"
  echo '` + tail + `'
} > "$out"
chmod +x "$out"
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func difftest(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestRunClean(t *testing.T) {
	g := fakeTool(t, "go", false, "x", "true")
	p := fakeTool(t, "gopatina", false, "x", "true")
	code, out, errs := difftest("-programs", "7", "-per", "3", "-jobs", "2", "-go", g, "-gopatina", p)
	if code != 0 {
		t.Fatalf("code %d: %s", code, errs)
	}
	var r report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	// With 10 clean tests the 6σ bound is -ln(Φ(-6)) / 10 per test.
	if r.Upper6Sigma == nil || *r.Upper6Sigma < 2.07e6 || *r.Upper6Sigma > 2.08e6 {
		t.Errorf("6σ bound %v", r.Upper6Sigma)
	}
	if r.Programs != 7 || r.PanicCases != 3 || r.Tests != 10 || r.Defects != 0 || r.Upper95 == nil ||
		strings.Count(errs, "0 mismatched") != 3 {
		t.Errorf("report %s, stderr %s", out, errs)
	}
}

// TestRunGoPanics: a Go panic in the panic case is expected; patina must
// match its status and first stderr line.
func TestRunGoPanics(t *testing.T) {
	tail := "echo panic: boom >&2; echo trace >&2; exit 2"
	g := fakeTool(t, "go", false, "x", tail)
	same := fakeTool(t, "gopatina", false, "x", "echo panic: boom >&2; exit 2")
	code, out, errs := difftest("-programs", "2", "-go", g, "-gopatina", same)
	if code != 0 {
		t.Errorf("same panic: code %d, %s %s", code, out, errs)
	}
	other := fakeTool(t, "gopatina", false, "x", "echo panic: other >&2; exit 2")
	code, out, _ = difftest("-programs", "2", "-go", g, "-gopatina", other, "-failures", t.TempDir())
	var r report
	json.Unmarshal([]byte(out), &r)
	if code != 3 || r.PanicMismatched != 1 || r.Mismatched != 0 || r.FailedPrograms[0] != 2 {
		t.Errorf("other panic: code %d, %s", code, out)
	}
}

func TestRunDefects(t *testing.T) {
	g := fakeTool(t, "go", false, "x", "true")
	fail := t.TempDir()
	for _, tc := range []struct {
		name    string
		patina  string
		defects int
	}{
		{"wrong output", fakeTool(t, "gopatina", false, "y", "true"), 6},
		{"bad exit", fakeTool(t, "gopatina", false, "x", "exit 2"), 2},
		{"build fails", fakeTool(t, "gopatina", true, "", ""), 6},
	} {
		code, out, errs := difftest("-programs", "4", "-per", "2", "-go", g, "-gopatina", tc.patina,
			"-work", t.TempDir(), "-failures", fail)
		var r report
		json.Unmarshal([]byte(out), &r)
		if code != 3 || r.Defects != tc.defects || r.Upper95 != nil || r.Upper6Sigma != nil {
			t.Errorf("%s: code %d, report %s, stderr %s", tc.name, code, out, errs)
		}
	}
	if _, err := os.Stat(filepath.Join(fail, "seed1-b0", "main.go")); err != nil {
		t.Error("failing batch not kept:", err)
	}
}

func TestRunErrors(t *testing.T) {
	ok := fakeTool(t, "go", false, "x", "true")
	for _, tc := range []struct {
		name string
		args []string
		code int
		msg  string
	}{
		{"flag", []string{"-bogus"}, 2, "not defined"},
		{"arg", []string{"extra"}, 2, "usage"},
		{"zero", []string{"-programs", "0"}, 2, "usage"},
		{"go build", []string{"-programs", "1", "-go", fakeTool(t, "go", true, "", "")}, 1, "go build"},
		{"go stderr", []string{"-programs", "1", "-go", fakeTool(t, "go", false, "x", "echo oops >&2"), "-gopatina", ok}, 1, "oops"},
		{"go fails", []string{"-programs", "1", "-go", fakeTool(t, "go", false, "x", "echo fatal >&2; exit 2"), "-gopatina", ok}, 1, "fatal"},
		{"work is a file", []string{"-programs", "1", "-work", "/dev/null"}, 1, "not a directory"},
	} {
		code, _, errs := difftest(tc.args...)
		if code != tc.code || !strings.Contains(errs, tc.msg) {
			t.Errorf("%s: code %d, stderr %q", tc.name, code, errs)
		}
	}
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if code, _, errs := difftest("-programs", "1"); code != 1 {
		t.Errorf("no temp dir: %d %q", code, errs)
	}
}

func TestUnwritableBatch(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere")
	}
	work := t.TempDir()
	b0 := filepath.Join(work, "b0")
	os.Mkdir(b0, 0o555)
	t.Cleanup(func() { os.Chmod(b0, 0o755) })
	if code, _, errs := difftest("-programs", "1", "-work", work); code != 1 || !strings.Contains(errs, "permission denied") {
		t.Errorf("code %d, stderr %q", code, errs)
	}
}

func TestExecuteMissing(t *testing.T) {
	if _, _, code := execute(filepath.Join(t.TempDir(), "none"), time.Second); code != -1 {
		t.Errorf("code %d", code)
	}
}

func TestCompare(t *testing.T) {
	want := segments([]byte("pre\n#3\na\n#4\nb\n"))
	if got := compare(want, want, 3, 2); len(got) != 0 {
		t.Errorf("same output: %v", got)
	}
	if got := compare(want, segments([]byte("#3\na\n#4\nc\n")), 3, 2); !slices.Equal(got, []int{4}) {
		t.Errorf("one differs: %v", got)
	}
	if got := compare(want, segments([]byte("#3\na\n")), 3, 2); !slices.Equal(got, []int{4}) {
		t.Errorf("one missing: %v", got)
	}
	if got := compare(want, segments([]byte("pre\n#3\na\n#4\nb")), 3, 2); !slices.Equal(got, []int{4}) {
		t.Errorf("final newline: %v", got)
	}
	if got := segments([]byte("#x\n#5\n#y\n")); got[5] != "#y\n" || len(got) != 1 {
		t.Errorf("non-numeric markers are output: %q", got)
	}
}

func TestSummarizeTruncates(t *testing.T) {
	var bad []int
	for k := range 150 {
		bad = append(bad, k)
	}
	r := summarize(config{programs: 999}, []outcome{{first: 0, n: 150, mismatched: bad}})
	if r.Defects != 150 || r.Tests != 1000 || len(r.FailedPrograms) != 100 || r.DefectsPerMillion != 150000 {
		t.Errorf("%+v", r)
	}
}

func TestMainExit(t *testing.T) {
	g := fakeTool(t, "go", false, "x", "true")
	oldExit, oldArgs, oldOut := exit, os.Args, os.Stdout
	t.Cleanup(func() { exit, os.Args, os.Stdout = oldExit, oldArgs, oldOut })
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()
	os.Stdout = devnull
	got := -1
	exit = func(c int) { got = c }
	os.Args = []string{"gopatina-difftest", "-programs", "1", "-go", g, "-gopatina", g}
	main()
	if got != 0 {
		t.Errorf("exit %d", got)
	}
}
