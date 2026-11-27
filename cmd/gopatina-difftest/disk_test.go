package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// cacheFiller wraps tool: each build first writes a 1 MiB file into its
// GOCACHE, as the batches' own packages fill the real one.
func cacheFiller(t *testing.T, tool string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "filler")
	script := "#!/bin/sh\nmkdir -p \"$GOCACHE\"\nhead -c 1048576 /dev/zero > \"$GOCACHE/obj-$$\"\nexec '" + tool + "' \"$@\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestGoCacheBounded checks that the run's Go build cache is cleared once
// it passes -gocache-max, and the run goes on.
func TestGoCacheBounded(t *testing.T) {
	g := cacheFiller(t, fakeTool(t, "go", false, "x", "true"))
	p := fakeTool(t, "gopatina", false, "x", "true")
	code, _, errs := difftest("-programs", "6", "-per", "2", "-gocache-max", "1", "-go", g, "-gopatina", p)
	if code != 0 || strings.Count(errs, "passed -gocache-max 1 MiB: cleared the run's Go build cache") != 3 {
		t.Errorf("code %d, stderr %s", code, errs)
	}
	if code, _, errs := difftest("-programs", "2", "-go", g, "-gopatina", p); code != 0 || strings.Contains(errs, "cleared") {
		t.Errorf("under the default limit: code %d, stderr %s", code, errs)
	}
}

// TestLowDisk checks that a run short of disk clears the Go build cache
// and goes on when that frees enough, and otherwise stops before the
// batch, saying where to resume, and skips the rest.
func TestLowDisk(t *testing.T) {
	defer func(f func(string) float64) { freeSpace = f }(freeSpace)
	g := fakeTool(t, "go", false, "x", "true")
	p := fakeTool(t, "gopatina", false, "x", "true")
	work := t.TempDir()
	cache := filepath.Join(work, "gocache")
	os.MkdirAll(cache, 0o755)
	// Free enough once the Go build cache is gone.
	freeSpace = func(string) float64 {
		if _, err := os.Stat(cache); err == nil {
			return 10
		}
		return 50
	}
	code, _, errs := difftest("-programs", "4", "-per", "2", "-work", work, "-go", g, "-gopatina", p)
	if code != 0 || !strings.Contains(errs, "only 10.0% of the disk is free: cleared the run's Go build cache") {
		t.Errorf("cleared: code %d, stderr %s", code, errs)
	}
	// Never enough.
	freeSpace = func(string) float64 { return 3 }
	code, out, errs := difftest("-programs", "6", "-per", "2", "-start", "4", "-go", g, "-gopatina", p)
	if code != 1 || out != "" || strings.Count(errs, "below -min-free 15%") != 1 || !strings.Contains(errs, "stopped (resume with -start 4)") {
		t.Errorf("stopped: code %d, stdout %q, stderr %s", code, out, errs)
	}
}

// TestGuardClearRace checks that a worker finding the cache already
// cleared by another leaves it.
func TestGuardClearRace(t *testing.T) {
	var logged []string
	g := &guard{gocache: t.TempDir(), logf: func(f string, a ...any) { logged = append(logged, f) }}
	g.clear(func() bool { return false }, "x")
	if _, err := os.Stat(g.gocache); err != nil || len(logged) != 0 {
		t.Errorf("cleared anyway: %v %v", err, logged)
	}
}

// TestSweep checks that a run removes the working directories of runs
// whose process is gone, and only those.
func TestSweep(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	mk := func(name, pid string) string {
		d := filepath.Join(tmp, name)
		os.MkdirAll(filepath.Join(d, "gocache"), 0o755)
		if pid != "" {
			os.WriteFile(filepath.Join(d, pidFile), []byte(pid), 0o644)
		}
		return d
	}
	dead := mk("gopatina-difftest-1", "2147483646")
	live := mk("gopatina-difftest-2", strconv.Itoa(os.Getpid()))
	old := mk("gopatina-difftest-3", "")
	bad := mk("gopatina-difftest-4", "x")
	other := mk("other-1", "2147483646")
	g := fakeTool(t, "go", false, "x", "true")
	code, _, errs := difftest("-programs", "1", "-go", g, "-gopatina", g)
	if code != 0 || !strings.Contains(errs, "removed "+dead+", left by a run that was killed") {
		t.Errorf("code %d, stderr %s", code, errs)
	}
	if _, err := os.Stat(dead); !os.IsNotExist(err) {
		t.Error("a dead run's directory stays")
	}
	for _, d := range []string{live, old, bad, other} {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("%s removed: %v", d, err)
		}
	}
}

// TestFreePercent checks the free space of a real disk, and of a path
// that has none.
func TestFreePercent(t *testing.T) {
	if f := freePercent(t.TempDir()); f <= 0 || f > 100 {
		t.Errorf("free %v%%", f)
	}
	if f := freePercent(filepath.Join(t.TempDir(), "missing")); f != 100 {
		t.Errorf("missing dir: %v%%", f)
	}
	if f := freePercent(os.DevNull); f != 100 {
		t.Errorf("%s: %v%%", os.DevNull, f)
	}
	if !alive(os.Getpid()) || alive(2147483646) {
		t.Error("alive")
	}
}

// TestSignalStopsBuilds checks that a signal stops the builds running at
// once, with what they started, and removes the run's directory only
// then: a build left running would fill it again. A build failing because
// it was stopped is not kept as a failure.
func TestSignalStopsBuilds(t *testing.T) {
	defer func(f func(int)) { exit = f }(exit)
	defer func(f func(chan<- os.Signal)) { notify = f }(notify)
	exited := make(chan int, 1)
	exit = func(code int) { exited <- code }
	sigc := make(chan chan<- os.Signal, 1)
	notify = func(c chan<- os.Signal) { sigc <- c }
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	started := filepath.Join(t.TempDir(), "started")
	slow := filepath.Join(t.TempDir(), "slow")
	// The marker comes once the late writer is forked, so the signal finds
	// it in the group.
	script := "#!/bin/sh\nmkdir -p \"$GOCACHE\"\n(sleep 2; mkdir -p \"$GOCACHE/late\") &\ntouch '" + started + "'\nwait\n"
	if err := os.WriteFile(slow, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	failures := filepath.Join(t.TempDir(), "failures")
	done := make(chan string)
	go func() {
		_, _, errs := difftest("-programs", "1", "-failures", failures, "-go", slow, "-gopatina", slow)
		done <- errs
	}()
	c := <-sigc
	for _, err := os.Stat(started); err != nil; _, err = os.Stat(started) {
		time.Sleep(10 * time.Millisecond)
	}
	at := time.Now()
	c <- os.Interrupt
	errs := <-done
	if took := time.Since(at); took > time.Second || !strings.Contains(errs, "stopped by interrupt") {
		t.Errorf("stopped after %v (the build takes 2s), stderr %s", took, errs)
	}
	if code := <-exited; code != 1 {
		t.Errorf("exit %d", code)
	}
	for _, wait := range []time.Duration{0, 3 * time.Second} {
		time.Sleep(wait)
		if left, _ := os.ReadDir(tmp); len(left) != 0 {
			t.Errorf("after %v, left behind in TMPDIR: %v", wait, left)
		}
	}
	if _, err := os.Stat(failures); !os.IsNotExist(err) {
		t.Errorf("a stopped build kept as a failure: %v", err)
	}
}

// TestHaltedStartsNothing checks that once a run is stopped no build or
// program starts, and no failure is kept.
func TestHaltedStartsNothing(t *testing.T) {
	c := config{cmds: newCommands()}
	c.cmds.halt()
	marker := filepath.Join(t.TempDir(), "ran")
	if _, err := c.build(time.Second, t.TempDir(), nil, "/bin/sh", "-c", "touch "+marker); err != errStopped {
		t.Errorf("build: %v", err)
	}
	if _, _, code := c.execute("/bin/true", time.Second); code != -1 {
		t.Errorf("execute: code %d", code)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("a build ran after the halt")
	}
	c.failed = filepath.Join(t.TempDir(), "failures")
	keep(c, 0, 1, "package main", nil)
	keepLibrary(c, 0, map[string][]byte{"x": nil})
	if _, err := os.Stat(c.failed); !os.IsNotExist(err) {
		t.Errorf("kept after the halt: %v", err)
	}
}
