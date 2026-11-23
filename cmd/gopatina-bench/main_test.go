package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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
	old, oldTime := perfCommand, timeCommand
	t.Cleanup(func() { perfCommand, timeCommand = old, oldTime })
	perfCommand = func(string) *exec.Cmd { return exec.Command("sh", "-c", script) }
	// GNU time, faked: it runs the program and records 1234 kilobytes.
	timeCommand = func(out, bin string) *exec.Cmd {
		return exec.Command("sh", "-c", `"$2" && echo 1234 > "$1"`, "sh", out, bin)
	}
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
	if r.Primary != "instructions" || r.Case != "c" || len(r.Results) != 4 ||
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
	if code, _, errs := runBench("-runs", "10", "-memruns", "5", ok, ok); code != 1 || !strings.Contains(errs, "at least 10") {
		t.Errorf("few memory runs: %d %q", code, errs)
	}
	// perf succeeds (helper fakes it) but the binary itself fails, which
	// only the memory runs outside perf see.
	if code, _, errs := runBench("-runs", "10", "-metric", "perf", ok, bin); code != 1 || !strings.Contains(errs, "exit status 1") {
		t.Errorf("failing memory run: %d %q", code, errs)
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

// TestRunThreeArms checks the plain Go arm: every pair is tested, every arm
// is reported with its estimates and size, and -memruns 0 leaves memory out.
func TestRunThreeArms(t *testing.T) {
	bin := helper(t, "ok", "perf-ok")
	code, out, errs := runBench("-runs", "10", "-metric", "perf", "-go", bin, bin, bin)
	if code != 0 {
		t.Fatalf("code %d: %s", code, errs)
	}
	var r struct {
		Go    string
		Pairs map[string]map[string]struct{ Verdict string }
		Arms  []struct {
			Name      string
			SizeBytes int64 `json:"size_bytes"`
			Estimates map[string]struct{ Runs int }
		}
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	if r.Go != bin || len(r.Pairs) != 3 || r.Pairs["gopatina_over_go"]["instructions"].Verdict != "equivalent" ||
		len(r.Pairs["go_over_rust"]) != 4 || len(r.Arms) != 3 || r.Arms[2].Name != "go" ||
		r.Arms[0].SizeBytes == 0 || r.Arms[1].Estimates["peak_kb"].Runs != 20 || r.Arms[2].Estimates["instructions"].Runs != 10 {
		t.Errorf("got %s", out)
	}
	if !strings.Contains(errs, "gopatina_over_go peak_memory") {
		t.Errorf("stderr %s", errs)
	}
	code, out, _ = runBench("-runs", "10", "-memruns", "0", "-go", bin, bin, filepath.Join(t.TempDir(), "gone")+"x")
	if code != 1 {
		t.Errorf("missing baseline: %d %s", code, out)
	}
	code, out, _ = runBench("-runs", "10", "-memruns", "0", bin, bin)
	if code != 0 || strings.Contains(out, "peak") {
		t.Errorf("-memruns 0: %d %s", code, out)
	}
}

// TestPeak checks the memory runs: GNU time's figure, a missing or garbled
// one (no memory figures, with a note), and the real GNU time where the
// machine has it.
func TestPeak(t *testing.T) {
	bin := helper(t, "ok", "perf-ok")
	if v, err := measurePeak(bin); err != nil || v.peakKB != 1234 {
		t.Errorf("faked: %v %v", v, err)
	}
	timeCommand = func(out, bin string) *exec.Cmd { return exec.Command("sh", "-c", "echo x > \"$1\"", "sh", out) }
	if _, err := measurePeak(bin); err == nil || !strings.Contains(err.Error(), "no peak memory") {
		t.Errorf("garbled: %v", err)
	}
	code, out, _ := runBench("-runs", "10", bin, bin)
	if code != 0 || !strings.Contains(out, "GNU time cannot measure") || strings.Contains(out, "peak_kb") {
		t.Errorf("no GNU time: %d %s", code, out)
	}
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if _, err := measurePeak(bin); err == nil {
		t.Error("no temporary directory: no error")
	}
}

func TestDefaultTimeCommand(t *testing.T) {
	if got := strings.Join(defaultTime("out", "prog").Args, " "); got != "/usr/bin/time -f %M -o out prog" {
		t.Errorf("time command %q", got)
	}
}

var defaultTime = timeCommand
