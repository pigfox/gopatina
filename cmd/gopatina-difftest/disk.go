package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
)

// Disk use. A long run must not fill the disk it runs on. Each batch
// builds in a directory of its own, crate caches and cargo targets
// included, removed once the batch is compared, so the batches take the
// room of the ones building at the time. What outlives a batch is the
// run's Go build cache (GOCACHE), which keeps the standard library built
// but also gains every batch's own packages, never used again: it is
// cleared whenever it passes -gocache-max, and Go rebuilds what it needs.
// Before each batch the run checks the disk holding its working
// directory: below -min-free percent free it clears the Go build cache,
// and if that is not enough it stops, saying where to resume. A signal
// stops the builds running before the run removes its files
// (commands.go). A run
// killed outright (SIGKILL) cannot clean up after itself: the next run
// removes the working directory of any run whose process is gone.

// guard bounds a run's disk use.
type guard struct {
	// mu is held shared by each batch, and alone to clear the Go build
	// cache, which no build may be using.
	mu      sync.RWMutex
	work    string
	gocache string
	max     int64   // bytes the Go build cache may take
	minFree float64 // percent of the disk that must stay free
	stopped atomic.Bool
	logf    func(format string, args ...any)
}

// errSkipped is the error of a batch not run because the run stopped.
var errSkipped = errors.New("skipped")

// before checks the disk before batch first: errSkipped once the run has
// stopped, an error saying so when the run must stop now.
func (g *guard) before(first int) error {
	if g.stopped.Load() {
		return errSkipped
	}
	low := func() bool { return freeSpace(g.work) < g.minFree }
	if !low() {
		return nil
	}
	g.clear(low, fmt.Sprintf("only %.1f%% of the disk is free", freeSpace(g.work)))
	if free := freeSpace(g.work); free < g.minFree {
		g.stopped.Store(true)
		return fmt.Errorf("batch %d: only %.1f%% of the disk holding %s is free, below -min-free %g%%, with the Go build cache cleared: stopped (resume with -start %d)",
			first, free, g.work, g.minFree, first)
	}
	return nil
}

// after clears the Go build cache once it passes its limit.
func (g *guard) after() {
	over := func() bool { return dirSize(g.gocache) > g.max }
	if over() {
		g.clear(over, fmt.Sprintf("the Go build cache passed -gocache-max %d MiB", g.max>>20))
	}
}

// clear removes the Go build cache, with no batch building, if need still
// holds then.
func (g *guard) clear(need func() bool, why string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !need() {
		return // another worker cleared it meanwhile
	}
	os.RemoveAll(g.gocache)
	g.logf("gopatina-difftest: %s: cleared the run's Go build cache\n", why)
}

// dirSize returns the bytes the files under dir take.
func dirSize(dir string) int64 {
	var n int64
	filepath.Walk(dir, func(_ string, info fs.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			n += info.Size()
		}
		return nil // a file removed while walking is skipped
	})
	return n
}

// freeSpace returns the percent of the disk holding a directory that is
// free; tests replace it.
var freeSpace = freePercent

// pidFile names the file holding a run's process id in its working
// directory.
const pidFile = "pid"

// sweep removes the working directories under tmp of runs whose process
// is gone, and returns them.
func sweep(tmp string) []string {
	var gone []string
	dirs, _ := filepath.Glob(filepath.Join(tmp, "gopatina-difftest-*"))
	for _, d := range dirs {
		data, err := os.ReadFile(filepath.Join(d, pidFile))
		if err != nil {
			continue // not a run's directory, or one of a run made before pid files
		}
		if pid, err := strconv.Atoi(string(data)); err == nil && !alive(pid) {
			os.RemoveAll(d)
			gone = append(gone, d)
		}
	}
	return gone
}
