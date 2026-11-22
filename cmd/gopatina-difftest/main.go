// Command gopatina-difftest measures how often GoPatina miscompiles: it
// builds random programs from internal/progen with the Go toolchain and with
// gopatina build -patina, runs both, and counts each program whose output
// differs, or that patina mode fails to build, as a defect. Each batch also
// ends with a panic case (internal/progen), which must match Go's output,
// the first line of Go's standard error and Go's exit status.
//
// usage: gopatina-difftest [-seed S] [-start K] [-programs N] [-per P]
//
//	[-jobs J] [-gopatina PATH] [-go PATH] [-work DIR] [-failures DIR]
//	[-timeout D]
//
// Programs are numbered start, start+1, ...; program k depends only on the
// seed and k, so a run can be split across machines by -start and resumed.
// They are built P to a batch (one Go package, one crate), J batches at a
// time. Failing batches are kept under -failures with both outputs. The
// report on standard output is JSON: programs, panic cases, tests (the two
// together), defects, defects per million tests, and, with no defects, the
// upper bounds on the defect rate at 95% and at 6σ confidence,
// -ln(alpha) / tests. Defect-free tests bound it at 3.4 per million from
// 881,098 tests at 95% and from 6,099,050 at 6σ (docs/QUALITY.md).
// The exit status is 0 with no defects, 3 with defects, 1 on an error (such
// as a program the Go toolchain itself does not build or run cleanly, which
// is a generator bug) and 2 on bad usage. See docs/QUALITY.md.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pigfox/gopatina/internal/progen"
)

var (
	exit = os.Exit
	now  = time.Now
)

func main() {
	exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type config struct {
	seed                          uint64
	start, programs, per, jobs    int
	gopatina, goCmd, work, failed string
	timeout                       time.Duration
}

// outcome is one batch's result.
type outcome struct {
	first, n    int
	mismatched  []int // programs whose output differs
	buildFailed bool  // patina mode did not build the batch
	panicBad    bool  // the panic case (program first+n) differs
}

type report struct {
	Seed              uint64   `json:"seed"`
	Start             int      `json:"start"`
	Programs          int      `json:"programs"`
	PanicCases        int      `json:"panic_cases"`
	Tests             int      `json:"tests"`
	PerBatch          int      `json:"per_batch"`
	Defects           int      `json:"defects"`
	Mismatched        int      `json:"mismatched"`
	PanicMismatched   int      `json:"panic_mismatched"`
	BuildFailed       int      `json:"build_failed"`
	DefectsPerMillion float64  `json:"defects_per_million"`
	Upper95           *float64 `json:"upper_bound_95_per_million,omitempty"`
	Upper6Sigma       *float64 `json:"upper_bound_6sigma_per_million,omitempty"`
	FailedPrograms    []int    `json:"failed_programs,omitempty"`
	Seconds           float64  `json:"seconds"`
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gopatina-difftest", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c config
	fs.Uint64Var(&c.seed, "seed", 1, "generator seed")
	fs.IntVar(&c.start, "start", 0, "number of the first program")
	fs.IntVar(&c.programs, "programs", 1000, "programs to test")
	fs.IntVar(&c.per, "per", 500, "programs per batch")
	fs.IntVar(&c.jobs, "jobs", 1, "batches built at a time")
	fs.StringVar(&c.gopatina, "gopatina", "gopatina", "the gopatina command")
	fs.StringVar(&c.goCmd, "go", "go", "the go command")
	fs.StringVar(&c.work, "work", "", "working directory (default: a temporary one, removed after)")
	fs.StringVar(&c.failed, "failures", "difftest-failures", "where failing batches are kept")
	fs.DurationVar(&c.timeout, "timeout", time.Minute, "limit for running one batch binary")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 || c.programs < 1 || c.per < 1 || c.jobs < 1 || c.start < 0 {
		fmt.Fprintln(stderr, "usage: gopatina-difftest [flags]; -programs, -per and -jobs must be positive")
		return 2
	}
	// Builds run in each batch's directory, so a relative path to a tool
	// would not be found there; a bare name is looked up in PATH.
	for _, tool := range []*string{&c.gopatina, &c.goCmd} {
		if strings.ContainsRune(*tool, filepath.Separator) {
			abs, _ := filepath.Abs(*tool)
			*tool = abs
		}
	}
	begin := now()
	if c.work == "" {
		dir, err := os.MkdirTemp("", "gopatina-difftest-")
		if err != nil {
			fmt.Fprintln(stderr, "gopatina-difftest:", err)
			return 1
		}
		defer os.RemoveAll(dir)
		c.work = dir
	}
	type job struct{ first, n int }
	var batches []job
	for first := c.start; first < c.start+c.programs; first += c.per {
		batches = append(batches, job{first, min(c.per, c.start+c.programs-first)})
	}
	results := make([]outcome, len(batches))
	errs := make([]error, len(batches))
	next := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex // guards stderr
	for w := range c.jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				results[i], errs[i] = batch(c, w, batches[i].first, batches[i].n)
				if errs[i] == nil {
					mu.Lock()
					fmt.Fprintf(stderr, "batch %d-%d: %d mismatched, build failed %t\n",
						batches[i].first, batches[i].first+batches[i].n-1, len(results[i].mismatched), results[i].buildFailed)
					mu.Unlock()
				}
			}
		}()
	}
	for i := range batches {
		next <- i
	}
	close(next)
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		fmt.Fprintln(stderr, "gopatina-difftest:", err)
		return 1
	}
	r := summarize(c, results)
	r.Seconds = now().Sub(begin).Seconds()
	out, _ := json.MarshalIndent(r, "", "  ")
	fmt.Fprintf(stdout, "%s\n", out)
	if r.Defects > 0 {
		return 3
	}
	return 0
}

func summarize(c config, results []outcome) report {
	r := report{Seed: c.seed, Start: c.start, Programs: c.programs, PerBatch: c.per}
	for _, o := range results {
		r.PanicCases++
		if o.buildFailed {
			r.BuildFailed += o.n + 1
			for k := o.first; k <= o.first+o.n; k++ {
				r.FailedPrograms = append(r.FailedPrograms, k)
			}
			continue
		}
		r.Mismatched += len(o.mismatched)
		r.FailedPrograms = append(r.FailedPrograms, o.mismatched...)
		if o.panicBad {
			r.PanicMismatched++
			r.FailedPrograms = append(r.FailedPrograms, o.first+o.n)
		}
	}
	r.Tests = r.Programs + r.PanicCases
	r.Defects = r.Mismatched + r.PanicMismatched + r.BuildFailed
	if len(r.FailedPrograms) > 100 {
		r.FailedPrograms = r.FailedPrograms[:100]
	}
	r.DefectsPerMillion = float64(r.Defects) / float64(r.Tests) * 1e6
	if r.Defects == 0 {
		u := -math.Log(0.05) / float64(r.Tests) * 1e6
		r.Upper95 = &u
		// alpha = P(Z > 6), the one-sided normal tail at 6σ.
		u6 := -math.Log(0.5*math.Erfc(6/math.Sqrt2)) / float64(r.Tests) * 1e6
		r.Upper6Sigma = &u6
	}
	return r
}

// batch generates, builds, runs and compares one batch. An error means the
// Go side failed, which no generated program may do.
func batch(c config, worker, first, n int) (outcome, error) {
	o := outcome{first: first, n: n}
	dir := filepath.Join(c.work, fmt.Sprintf("b%d", first))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return o, err
	}
	defer os.RemoveAll(dir)
	src := progen.Batch(c.seed, first, n)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module difftest\n\ngo 1.25\n"), 0o644)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		return o, err
	}
	goBin, pBin := filepath.Join(dir, "go.bin"), filepath.Join(dir, "patina.bin")
	if out, err := build(dir, nil, c.goCmd, "build", "-o", goBin, "."); err != nil {
		return o, fmt.Errorf("batch %d: go build: %v\n%s", first, err, out)
	}
	cache := "GOPATINA_CACHE=" + filepath.Join(c.work, "cache-"+strconv.Itoa(worker))
	pOut, err := build(dir, []string{cache}, c.gopatina, "build", "-patina", "-o", pBin, ".")
	if _, ran := err.(*exec.ExitError); err != nil && !ran {
		return o, fmt.Errorf("batch %d: %s could not run: %v", first, c.gopatina, err)
	}
	if err != nil {
		o.buildFailed = true
		keep(c, first, src, map[string][]byte{"patina-build.txt": pOut})
		return o, nil
	}
	goStdout, goStderr, goCode := execute(goBin, c.timeout)
	// Only the panic case may fail, and only with a Go panic (status 2).
	if !(goCode == 0 && len(goStderr) == 0 || goCode == 2 && bytes.HasPrefix(goStderr, []byte("panic: "))) {
		return o, fmt.Errorf("batch %d: the Go binary exited %d: %s", first, goCode, goStderr)
	}
	pStdout, pStderr, pCode := execute(pBin, c.timeout)
	w, g := segments(goStdout), segments(pStdout)
	o.mismatched = compare(w, g, first, n)
	last := first + n
	o.panicBad = len(compare(w, g, last, 1)) > 0 || pCode != goCode ||
		firstLine(pStderr) != firstLine(goStderr)
	if len(o.mismatched) > 0 || o.panicBad {
		keep(c, first, src, map[string][]byte{
			"go.out": goStdout, "patina.out": pStdout, "patina.err": pStderr,
			"patina.exit": []byte(strconv.Itoa(pCode) + "\n"),
		})
	}
	return o, nil
}

// build runs a build command in dir and returns its combined output.
func build(dir string, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	return cmd.CombinedOutput()
}

// execute runs bin with a time limit and returns its output and exit
// status (-1 if it could not run or was killed).
func execute(bin string, limit time.Duration) (stdout, stderr []byte, code int) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	cmd.Run()
	code = -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	return o.Bytes(), e.Bytes(), code
}

// compare returns the programs among first..first+n-1 whose output
// segments differ between want and got, or are missing from either.
func compare(want, got map[int]string, first, n int) []int {
	var bad []int
	for k := first; k < first+n; k++ {
		ws, wok := want[k]
		gs, gok := got[k]
		if !wok || !gok || ws != gs {
			bad = append(bad, k)
		}
	}
	return bad
}

// segments splits output into each program's output, keyed by the number
// on the marker line before it. Each segment keeps its text exactly,
// newlines included.
func segments(out []byte) map[int]string {
	seg := map[int]string{}
	cur := -1
	for line := range strings.SplitAfterSeq(string(out), "\n") {
		if rest, ok := strings.CutPrefix(line, progen.Marker); ok {
			if k, err := strconv.Atoi(strings.TrimSuffix(rest, "\n")); err == nil {
				cur = k
				seg[k] = ""
				continue
			}
		}
		if cur >= 0 {
			seg[cur] += line
		}
	}
	return seg
}

// firstLine is the first line of standard error, the part of a panic that
// GoPatina reproduces (Go's stack trace after it is not).
func firstLine(b []byte) string {
	line, _, _ := strings.Cut(string(b), "\n")
	return line
}

// keep saves a failing batch's source and outputs under the failures
// directory.
func keep(c config, first int, src string, files map[string][]byte) {
	dir := filepath.Join(c.failed, fmt.Sprintf("seed%d-b%d", c.seed, first))
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644)
	for name, data := range files {
		os.WriteFile(filepath.Join(dir, name), data, 0o644)
	}
}
