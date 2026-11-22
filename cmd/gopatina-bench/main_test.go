package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// helper returns a program in the given mode ("ok" exits 0, anything else
// exits 1) and makes perf act out a perf mode: "perf-ok" counts cycles,
// "perf-bad" reports them not counted, "perf-none" prints no cycles line,
// and anything else fails.
func helper(t *testing.T, prog, perf string) string {
	t.Helper()
	name := "false"
	if prog == "ok" {
		name = "true"
	}
	bin, err := exec.LookPath(name)
	if err != nil {
		t.Skip(err)
	}
	script := map[string]string{
		"perf-ok":   "echo 654321,,instructions:u,1000,100.00,, >&2; echo 123456,,cycles:u,1000,100.00,, >&2; echo 1,,task-clock,1,1,, >&2",
		"perf-bad":  "echo '<not counted>,,cycles:u,0,100.00,,' >&2",
		"perf-none": "echo 1,,task-clock,1,1,, >&2",
	}[perf]
	if script == "" {
		script = "exit 1"
	}
	old := perfCommand
	t.Cleanup(func() { perfCommand = old })
	perfCommand = func(string) *exec.Cmd { return exec.Command("sh", "-c", script) }
	return bin
}

func runBench(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestRunCycles(t *testing.T) {
	bin := helper(t, "ok", "perf-ok")
	code, out, errs := runBench("-runs", "10", "-metric", "perf", "-case", "c", bin, bin)
	if code != 0 {
		t.Fatalf("code %d: %s", code, errs)
	}
	var r struct {
		Primary string
		Case    string
		Results map[string]struct{ Verdict string }
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	// Every run counts the same instructions and cycles, so their ratio is
	// exactly 1; wall time is measured too.
	if r.Primary != "instructions" || r.Case != "c" || len(r.Results) != 3 ||
		r.Results["instructions"].Verdict != "equivalent" || r.Results["cycles"].Verdict != "equivalent" {
		t.Errorf("got %s", out)
	}
}

func TestRunWallFallback(t *testing.T) {
	bin := helper(t, "ok", "fail")
	code, out, errs := runBench("-runs", "10", "-metric", "perf", "-warmup", "1", bin, bin)
	if code != 0 {
		t.Fatalf("code %d: %s", code, errs)
	}
	if !strings.Contains(out, `"primary": "wall"`) || !strings.Contains(out, "perf cannot count") {
		t.Errorf("got %s", out)
	}
	code, out, _ = runBench("-runs", "10", bin, bin)
	if code != 0 || strings.Contains(out, "notes") {
		t.Errorf("explicit wall: %d %s", code, out)
	}
}

func TestRunErrors(t *testing.T) {
	bin := helper(t, "fail", "fail")
	for _, tc := range []struct {
		args []string
		code int
		msg  string
	}{
		{[]string{"-bogus"}, 2, "flag provided but not defined"},
		{[]string{bin}, 2, "usage"},
		{[]string{"-metric", "x", bin, bin}, 2, "unknown -metric"},
		{[]string{"-metric", "wall", bin, bin}, 1, "exit status 1"},
		{[]string{"-metric", "wall", "-warmup", "0", bin, bin}, 1, "exit status 1"},
	} {
		code, _, errs := runBench(tc.args...)
		if code != tc.code || !strings.Contains(errs, tc.msg) {
			t.Errorf("%v: code %d, stderr %q", tc.args, code, errs)
		}
	}
	ok := helper(t, "ok", "perf-ok")
	if code, _, errs := runBench("-runs", "5", "-metric", "perf", ok, ok); code != 1 || !strings.Contains(errs, "at least 10") {
		t.Errorf("few runs: %d %q", code, errs)
	}
}

func TestCyclesParse(t *testing.T) {
	for _, mode := range []string{"perf-bad", "perf-none", "fail"} {
		bin := helper(t, "ok", mode)
		if _, err := measurePerf(bin); err == nil {
			t.Errorf("%s: no error", mode)
		}
	}
}

func TestMainExit(t *testing.T) {
	bin := helper(t, "ok", "perf-ok")
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
	os.Args = []string{"gopatina-bench", "-runs", "10", "-metric", "perf", bin, bin}
	main()
	if got != 0 {
		t.Errorf("exit %d", got)
	}
}

var defaultPerf = perfCommand

func TestDefaultPerfCommand(t *testing.T) {
	got := strings.Join(defaultPerf("prog").Args, " ")
	if got != "perf stat -x, -e instructions:u,cycles:u -- prog" {
		t.Errorf("perf command %q", got)
	}
}
