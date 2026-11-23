// Command gopatina-bench tests whether two programs run at the same speed,
// to a stated confidence, and prints the result as JSON.
//
// usage: gopatina-bench [-runs N] [-warmup N] [-margin M] [-sigma S]
//
//	[-metric wall|perf] [-case NAME] [-go GO] [-memruns N] CANDIDATE BASELINE
//
// It runs CANDIDATE and BASELINE (two binaries that take no arguments) N
// times each, in ABBA order so slow drift affects both alike, and measures
// each run's wall time. With -go, the same Go program built with plain go
// build is a third arm (ABCCBA order), and every pair is tested:
// GoPatina / Rust, Go / Rust and GoPatina / Go. Each arm is also reported on
// its own (each metric's geometric mean with its interval, its binary size),
// and -memruns more runs outside perf measure each arm's peak resident
// memory under GNU time, with a test per pair. With -metric perf, where perf can read the CPU's
// counters, each run also records user instructions (then the primary,
// lowest-noise metric) and user cycles; the counters are off by default.
// For each metric it runs benchstat's TOST equivalence test on the paired
// log ratios CANDIDATE / BASELINE with the given margin (0.005, ±0.5%, by default) and
// sigma (5 is a one-sided α of about 2.87e-7 per test). See bench/README.md.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pigfox/gopatina/internal/benchstat"
)

var (
	exit = os.Exit
	// perfCommand is the perf invocation that counts bin's user
	// instructions and cycles.
	perfCommand = func(bin string) *exec.Cmd {
		return exec.Command("perf", "stat", "-x,", "-e", "instructions:u,cycles:u", "--", bin)
	}
	now = time.Now
	// timeCommand is the GNU time invocation that writes bin's peak
	// resident memory, in kilobytes, to out. A child of this process cannot
	// measure it itself: Go starts children sharing its memory until they
	// exec, and Linux carries that high-water mark across exec, so every
	// child would read at least this process's own size. GNU time forks
	// first, and its floor is a small C program's.
	timeCommand = func(out, bin string) *exec.Cmd {
		return exec.Command("/usr/bin/time", "-f", "%M", "-o", out, bin)
	}
)

func main() {
	exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// sample is one run's measurements. Instructions and cycles are user-space
// counts from perf; they are zero when only wall time is measured.
type sample struct {
	instructions, cycles, wall float64
	peakKB                     float64 // peak resident memory, from GNU time runs only
}

// report is the JSON gopatina-bench prints. Results holds one TOST result
// per metric measured; Primary names the one to read first.
type report struct {
	Case      string                      `json:"case,omitempty"`
	Primary   string                      `json:"primary"`
	Candidate string                      `json:"candidate"`
	Baseline  string                      `json:"baseline"`
	Warmup    int                         `json:"warmup"`
	Results   map[string]benchstat.Result `json:"results"`
	Notes     []string                    `json:"notes,omitempty"`
	// With -go: the third arm, every pair's results (results is
	// gopatina_over_rust), and each arm's own estimates.
	Go    string                                 `json:"go,omitempty"`
	Pairs map[string]map[string]benchstat.Result `json:"pairs,omitempty"`
	Arms  []arm                                  `json:"arms"`
}

// An arm is one program's own measurements: the geometric mean of each
// metric with its interval, peak memory in kilobytes, and binary size.
type arm struct {
	Name      string                        `json:"name"`
	Path      string                        `json:"path"`
	SizeBytes int64                         `json:"size_bytes"`
	Estimates map[string]benchstat.Estimate `json:"estimates"`
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gopatina-bench", flag.ContinueOnError)
	fs.SetOutput(stderr)
	runs := fs.Int("runs", 200, "runs of each binary (pairs)")
	warmup := fs.Int("warmup", 2, "unmeasured runs of each binary first")
	margin := fs.Float64("margin", 0.005, "equivalence margin (0.005 is ±0.5%)")
	sigma := fs.Float64("sigma", 5, "confidence in standard deviations")
	metric := fs.String("metric", "wall", "wall, or perf to also count user instructions and cycles")
	name := fs.String("case", "", "case name recorded in the report")
	goBin := fs.String("go", "", "a third arm, the program built with plain go build")
	memRuns := fs.Int("memruns", 20, "runs of each arm outside perf for peak memory (0: none)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(stderr, "usage: gopatina-bench [flags] CANDIDATE BASELINE")
		return 2
	}
	cand, base := fs.Arg(0), fs.Arg(1)
	arms := []string{cand, base}
	if *goBin != "" {
		arms = append(arms, *goBin)
	}
	var notes []string
	measure := measureWall
	switch *metric {
	case "perf":
		if _, err := measurePerf(cand); err == nil {
			measure = measurePerf
		} else {
			notes = append(notes, "perf cannot count instructions and cycles here ("+err.Error()+"); measured wall time only")
		}
	case "wall":
	default:
		fmt.Fprintf(stderr, "gopatina-bench: unknown -metric %q\n", *metric)
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintln(stderr, "gopatina-bench:", err)
		return 1
	}
	for range *warmup {
		for _, bin := range arms {
			if _, err := measure(bin); err != nil {
				return fail(err)
			}
		}
	}
	// Each run measures every arm, in an order that reverses every other
	// run (ABBA for two arms, ABCCBA for three), so slow drift affects all
	// alike.
	samples := make([][]sample, len(arms))
	for i := range *runs {
		for k := range arms {
			j := k
			if i%2 == 1 {
				j = len(arms) - 1 - k
			}
			v, err := measure(arms[j])
			if err != nil {
				return fail(err)
			}
			samples[j] = append(samples[j], v)
		}
	}
	pick := map[string]func(sample) float64{
		"wall": func(s sample) float64 { return s.wall },
	}
	primary := "wall"
	if *runs > 0 && samples[0][0].instructions > 0 {
		primary = "instructions"
		pick["instructions"] = func(s sample) float64 { return s.instructions }
		pick["cycles"] = func(s sample) float64 { return s.cycles }
	}
	// Peak memory: the arms' own resident set, from runs under GNU time
	// (not under perf, whose usage is its own), in the same alternating
	// order.
	if *memRuns > 0 {
		if _, err := measurePeak(cand); err != nil {
			notes = append(notes, "GNU time cannot measure peak memory here ("+err.Error()+"); no memory figures")
			*memRuns = 0
		}
	}
	mem := make([][]sample, len(arms))
	for i := range *memRuns {
		for k := range arms {
			j := k
			if i%2 == 1 {
				j = len(arms) - 1 - k
			}
			v, err := measurePeak(arms[j])
			if err != nil {
				return fail(err)
			}
			mem[j] = append(mem[j], v)
		}
	}
	peak := func(s sample) float64 { return s.peakKB }
	var errs []error // the statistics' errors: too few runs
	compare := func(a, b int) map[string]benchstat.Result {
		res := map[string]benchstat.Result{}
		for m, f := range pick {
			r, err := benchstat.Equivalence(values(samples[a], f), values(samples[b], f), *margin, *sigma)
			errs = append(errs, err)
			res[m] = r
		}
		if *memRuns > 0 {
			r, err := benchstat.Equivalence(values(mem[a], peak), values(mem[b], peak), *margin, *sigma)
			errs = append(errs, err)
			res["peak_memory"] = r
		}
		return res
	}
	results := compare(0, 1)
	rep := report{
		Case: *name, Primary: primary, Candidate: cand, Baseline: base,
		Warmup: *warmup, Results: results, Notes: notes,
	}
	if *goBin != "" {
		rep.Go = *goBin
		rep.Pairs = map[string]map[string]benchstat.Result{
			"gopatina_over_rust": results, "go_over_rust": compare(2, 1), "gopatina_over_go": compare(0, 2),
		}
	}
	for j, bin := range arms {
		a := arm{Name: armNames[j], Path: bin, Estimates: map[string]benchstat.Estimate{}}
		if fi, err := os.Stat(bin); err == nil {
			a.SizeBytes = fi.Size()
		}
		for m, f := range pick {
			e, err := benchstat.Interval(values(samples[j], f), *sigma)
			errs = append(errs, err)
			a.Estimates[m] = e
		}
		if *memRuns > 0 {
			e, err := benchstat.Interval(values(mem[j], peak), *sigma)
			errs = append(errs, err)
			a.Estimates["peak_kb"] = e
		}
		rep.Arms = append(rep.Arms, a)
	}
	if err := errors.Join(errs...); err != nil {
		return fail(err)
	}
	out, _ := json.MarshalIndent(rep, "", "  ")
	fmt.Fprintf(stdout, "%s\n", out)
	pairs := []string{"gopatina_over_rust"}
	all := map[string]map[string]benchstat.Result{"gopatina_over_rust": results}
	if rep.Pairs != nil {
		pairs, all = []string{"gopatina_over_rust", "go_over_rust", "gopatina_over_go"}, rep.Pairs
	}
	for _, p := range pairs {
		for _, m := range []string{"instructions", "cycles", "wall", "peak_memory"} {
			if r, ok := all[p][m]; ok {
				fmt.Fprintf(stderr, "%s %s %s: %s, ratio %.4f [%.4f, %.4f], %d pairs, margin %g, %g sigma\n",
					*name, p, m, r.Verdict, r.Ratio, r.Low, r.High, r.Pairs, r.Margin, r.Sigma)
			}
		}
	}
	return 0
}

// armNames name the arms in the report, in argument order.
var armNames = []string{"gopatina", "rust", "go"}

// values picks one metric from each sample.
func values(ss []sample, f func(sample) float64) []float64 {
	xs := make([]float64, len(ss))
	for i, s := range ss {
		xs[i] = f(s)
	}
	return xs
}

// measureWall runs bin once, discarding its output, and returns its wall
// time in nanoseconds.
func measureWall(bin string) (sample, error) {
	cmd := exec.Command(bin)
	start := now()
	if err := cmd.Run(); err != nil {
		return sample{}, fmt.Errorf("%s: %w", bin, err)
	}
	return sample{wall: float64(now().Sub(start).Nanoseconds())}, nil
}

// measurePeak runs bin once under GNU time and returns its peak resident
// memory in kilobytes.
func measurePeak(bin string) (sample, error) {
	f, err := os.CreateTemp("", "gopatina-bench-peak-")
	if err != nil {
		return sample{}, err
	}
	f.Close()
	defer os.Remove(f.Name())
	if err := timeCommand(f.Name(), bin).Run(); err != nil {
		return sample{}, fmt.Errorf("time %s: %w", bin, err)
	}
	data, _ := os.ReadFile(f.Name())
	kb, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil || !(kb > 0) {
		return sample{}, fmt.Errorf("time %s: no peak memory in %q", bin, data)
	}
	return sample{peakKB: kb}, nil
}

// measurePerf runs bin once under perf stat and returns the user
// instructions and cycles it counted, and the wall time of the perf run.
func measurePerf(bin string) (sample, error) {
	cmd := perfCommand(bin)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	start := now()
	if err := cmd.Run(); err != nil {
		return sample{}, fmt.Errorf("perf stat %s: %w", bin, err)
	}
	s := sample{wall: float64(now().Sub(start).Nanoseconds())}
	for line := range strings.SplitSeq(errb.String(), "\n") {
		f := strings.Split(line, ",")
		if len(f) < 3 {
			continue
		}
		var dst *float64
		switch {
		case strings.HasPrefix(f[2], "instructions"):
			dst = &s.instructions
		case strings.HasPrefix(f[2], "cycles"):
			dst = &s.cycles
		default:
			continue
		}
		v, err := strconv.ParseFloat(f[0], 64)
		if err != nil || !(v > 0) {
			return sample{}, fmt.Errorf("perf stat %s: %s not counted (%q)", bin, f[2], f[0])
		}
		*dst = v
	}
	if s.instructions == 0 || s.cycles == 0 {
		return sample{}, errors.New("perf stat " + bin + ": no instructions and cycles lines")
	}
	return s, nil
}
