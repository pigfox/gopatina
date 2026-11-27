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

// twoGo writes a stand-in for go whose reference build (-gcflags=all=-N -l)
// behaves like ref, and whose plain build behaves like opt; each is a
// fakeTool.
func twoGo(t *testing.T, ref, opt string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "go")
	script := "#!/bin/sh\ncase \"$*\" in\n*-gcflags=all=-N\\ -l*) exec '" + ref + "' \"$@\" ;;\n*) exec '" + opt + "' \"$@\" ;;\nesac\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestGoDisagreements checks that patina is compared with Go's reference
// build, and that a plain Go build that prints something else is recorded
// as a Go disagreement, not a defect.
func TestGoDisagreements(t *testing.T) {
	ref := fakeTool(t, "goref", false, "x", "true")
	p := fakeTool(t, "gopatina", false, "x", "true")
	fail := t.TempDir()
	for _, tc := range []struct {
		name      string
		opt       string
		disagreed int
	}{
		{"different output", fakeTool(t, "goopt", false, "y", "true"), 6},
		{"different exit", fakeTool(t, "goopt", false, "x", "echo panic: x >&2; exit 2"), 2},
		{"agreeing", fakeTool(t, "goopt", false, "x", "true"), 0},
	} {
		code, out, errs := difftest("-programs", "4", "-per", "2", "-go", twoGo(t, ref, tc.opt), "-gopatina", p, "-failures", fail)
		var r report
		json.Unmarshal([]byte(out), &r)
		if code != 0 || r.Defects != 0 || r.GoDisagreements != tc.disagreed || !strings.Contains(errs, "Go disagreements") {
			t.Errorf("%s: code %d, report %s, stderr %s", tc.name, code, out, errs)
		}
	}
	if _, err := os.Stat(filepath.Join(fail, "seed1-b0", "goref.out")); err != nil {
		t.Error("a disagreeing batch is not kept:", err)
	}
	code, _, errs := difftest("-programs", "1", "-go", twoGo(t, fakeTool(t, "goref", true, "", ""), p), "-gopatina", p)
	if code != 1 || !strings.Contains(errs, "go build (reference)") {
		t.Errorf("reference build failure: %d %s", code, errs)
	}
	code, _, errs = difftest("-programs", "1", "-go", twoGo(t, ref, fakeTool(t, "goopt", true, "", "")), "-gopatina", p)
	if code != 1 || !strings.Contains(errs, "go build:") {
		t.Errorf("plain build failure: %d %s", code, errs)
	}
}

// difftest runs the harness without the lint and library checks, which
// extra_test.go tests with a fake cargo.
func difftest(args ...string) (int, string, string) {
	return difftestWith(append([]string{"-lint", "0", "-libraries", "0"}, args...)...)
}

func difftestWith(args ...string) (int, string, string) {
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

// TestBuildTimeout checks that a gopatina build past -build-timeout is
// stopped, fails its batch (a defect for each of its programs), says so on
// stderr with the batch's programs, and keeps the batch with the note.
func TestBuildTimeout(t *testing.T) {
	g := fakeTool(t, "go", false, "x", "true")
	hang := filepath.Join(t.TempDir(), "gopatina")
	if err := os.WriteFile(hang, []byte("#!/bin/sh\nexec sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	fail := t.TempDir()
	start := time.Now()
	code, out, errs := difftest("-programs", "2", "-per", "2", "-go", g, "-gopatina", hang,
		"-build-timeout", "400ms", "-work", t.TempDir(), "-failures", fail)
	if took := time.Since(start); took > 20*time.Second {
		t.Errorf("the hanging build was not stopped: %v", took)
	}
	var r report
	json.Unmarshal([]byte(out), &r)
	if code != 3 || r.Defects == 0 || !strings.Contains(errs, "build timed out after 1.2s (400ms per program; programs 0-1") {
		t.Errorf("code %d, report %s, stderr %s", code, out, errs)
	}
	kept, err := os.ReadFile(filepath.Join(fail, "seed1-b0", "patina-build.txt"))
	if err != nil || !strings.Contains(string(kept), "ran past 1.2s (-build-timeout 400ms per program)") {
		t.Errorf("kept %q, %v", kept, err)
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
		{"no Go cache", []string{"-gocache-max", "0"}, 2, "usage"},
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

// TestToolPaths checks that a relative tool path works although builds run
// in other directories, and that a tool that cannot run stops the run
// instead of counting every test as a defect.
func TestToolPaths(t *testing.T) {
	g := fakeTool(t, "go", false, "x", "true")
	dir := filepath.Dir(g)
	t.Chdir(dir)
	if code, out, errs := difftest("-programs", "2", "-go", "./go", "-gopatina", "./go"); code != 0 {
		t.Errorf("relative paths: code %d, %s %s", code, out, errs)
	}
	code, _, errs := difftest("-programs", "2", "-go", "./go", "-gopatina", "./missing")
	if code != 1 || !strings.Contains(errs, "could not run") {
		t.Errorf("missing gopatina: code %d, stderr %q", code, errs)
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
	if _, _, code := (config{cmds: newCommands()}).execute(filepath.Join(t.TempDir(), "none"), time.Second); code != -1 {
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
	r := summarize(config{programs: 999}, []outcome{{first: 0, n: 150, mismatched: bad, goDisagree: bad}})
	if r.Defects != 150 || r.Tests != 1000 || len(r.FailedPrograms) != 100 || r.DefectsPerMillion != 150000 ||
		r.GoDisagreements != 150 || len(r.GoDisagreementPrograms) != 100 {
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
	os.Args = []string{"gopatina-difftest", "-lint", "0", "-libraries", "0", "-programs", "1", "-go", g, "-gopatina", g}
	main()
	if got != 0 {
		t.Errorf("exit %d", got)
	}
}

// recordCache wraps tool: it appends the GOCACHE each build sees to log.
func recordCache(t *testing.T, tool, log string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rec")
	script := "#!/bin/sh\necho \"$GOCACHE\" >> '" + log + "'\nexec '" + tool + "' \"$@\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRunLeavesNothing checks that a clean run removes its working
// directory, batches and caches, and that every build uses the run's own
// Go build cache, not the shared one.
func TestRunLeavesNothing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	log := filepath.Join(t.TempDir(), "gocache.log")
	g := recordCache(t, fakeTool(t, "go", false, "x", "true"), log)
	p := recordCache(t, fakeTool(t, "gopatina", false, "x", "true"), log)
	if code, _, errs := difftest("-programs", "4", "-per", "2", "-jobs", "2", "-go", g, "-gopatina", p); code != 0 {
		t.Fatalf("code %d: %s", code, errs)
	}
	if left, _ := os.ReadDir(tmp); len(left) != 0 {
		t.Errorf("left behind in TMPDIR: %v", left)
	}
	data, _ := os.ReadFile(log)
	caches := strings.Fields(string(data))
	if len(caches) != 6 { // two batches: reference, plain and patina builds
		t.Fatalf("builds saw GOCACHE %q", caches)
	}
	for _, c := range caches {
		if !strings.HasPrefix(c, filepath.Join(tmp, "gopatina-difftest-")) || filepath.Base(c) != "gocache" {
			t.Errorf("GOCACHE %s is not the run's own", c)
		}
		if _, err := os.Stat(c); !os.IsNotExist(err) {
			t.Errorf("GOCACHE %s is still there", c)
		}
	}
	// With -work, the directory is the caller's: it stays, empty.
	work := t.TempDir()
	os.MkdirAll(filepath.Join(work, "gocache", "x"), 0o755)
	if code, _, errs := difftest("-programs", "2", "-work", work, "-go", g, "-gopatina", p); code != 0 {
		t.Fatalf("code %d: %s", code, errs)
	}
	if left, err := os.ReadDir(work); err != nil || len(left) != 0 {
		t.Errorf("left behind in -work: %v %v", left, err)
	}
}

// TestSignalCleans checks that a signal removes what the run made, then
// exits with status 1, and that the end of a run stops the wait.
func TestSignalCleans(t *testing.T) {
	defer func(f func(int)) { exit = f }(exit)
	code := -1
	exit = func(c int) { code = c }
	sigs := make(chan os.Signal, 1)
	sigs <- os.Interrupt
	cleaned := false
	var errb bytes.Buffer
	onSignal(sigs, nil, func() { cleaned = true }, &errb)
	if !cleaned || code != 1 || !strings.Contains(errb.String(), "stopped by interrupt") {
		t.Errorf("cleaned %v, exit %d, stderr %q", cleaned, code, errb.String())
	}
	done := make(chan struct{})
	close(done)
	onSignal(make(chan os.Signal), done, func() { t.Error("cleaned at the end of a run") }, &errb)
}

// TestTargetFuses checks which targets make gc's default build the
// reference: those where gc fuses float multiply-adds.
func TestTargetFuses(t *testing.T) {
	answer := func(out string, code int) string {
		path := filepath.Join(t.TempDir(), "go")
		os.WriteFile(path, []byte("#!/bin/sh\nprintf '"+out+"'\nexit "+string(rune('0'+code))+"\n"), 0o755)
		return path
	}
	for out, want := range map[string]bool{
		"arm64\\n\\n": true, "amd64\\nv3\\n": true, "amd64\\nv1\\n": false, "arm64": false, "": false,
	} {
		if got := targetFuses(answer(out, 0)); got != want {
			t.Errorf("%q: %v", out, got)
		}
	}
	if targetFuses(answer("arm64\\n\\n", 1)) {
		t.Error("a failing go env")
	}
}

// TestFusingReference checks that, where gc fuses, the reference build
// is the default one and the other is -N -l.
func TestFusingReference(t *testing.T) {
	log := filepath.Join(t.TempDir(), "builds")
	path := filepath.Join(t.TempDir(), "go")
	real := fakeTool(t, "go", false, "x", "true")
	script := "#!/bin/sh\nif [ \"$1\" = env ]; then printf 'arm64\\n\\n'; exit 0; fi\necho \"$@\" >> '" + log + "'\nexec '" + real + "' \"$@\"\n"
	os.WriteFile(path, []byte(script), 0o755)
	p := fakeTool(t, "gopatina", false, "x", "true")
	if code, _, errs := difftest("-programs", "1", "-go", path, "-gopatina", p); code != 0 {
		t.Fatalf("code %d: %s", code, errs)
	}
	data, _ := os.ReadFile(log)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 || strings.Contains(lines[0], "-N -l") || !strings.Contains(lines[1], "-N -l") || !strings.Contains(lines[0], "goref.bin") {
		t.Errorf("builds:\n%s", data)
	}
}
