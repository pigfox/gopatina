// Command gopatina-bench tests whether two programs run at the same speed,
// to a stated confidence, and prints the result as JSON.
//
// usage: gopatina-bench [-runs N] [-warmup N] [-margin M] [-sigma S]
//
//	[-metric wall|perf] [-case NAME] CANDIDATE BASELINE
//
// It runs CANDIDATE and BASELINE (two binaries that take no arguments) N
// times each, in ABBA order so slow drift affects both alike, and measures
// each run's wall time. With -metric perf, where perf can read the CPU's
// counters, each run also records user instructions (then the primary,
// lowest-noise metric) and user cycles; the counters are off by default.
// For each metric it runs benchstat's TOST equivalence test on the paired
// log ratios CANDIDATE / BASELINE with the given margin (0.01 is ±1%) and
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
)

func main() {
	exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// sample is one run's measurements. Instructions and cycles are user-space
// counts from perf; they are zero when only wall time is measured.
type sample struct {
	instructions, cycles, wall float64
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
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gopatina-bench", flag.ContinueOnError)
	fs.SetOutput(stderr)
	runs := fs.Int("runs", 200, "runs of each binary (pairs)")
	warmup := fs.Int("warmup", 2, "unmeasured runs of each binary first")
	margin := fs.Float64("margin", 0.01, "equivalence margin (0.01 is ±1%)")
	sigma := fs.Float64("sigma", 5, "confidence in standard deviations")
	metric := fs.String("metric", "wall", "wall, or perf to also count user instructions and cycles")
	name := fs.String("case", "", "case name recorded in the report")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(stderr, "usage: gopatina-bench [flags] CANDIDATE BASELINE")
		return 2
	}
	cand, base := fs.Arg(0), fs.Arg(1)
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
	for range *warmup {
		for _, bin := range []string{cand, base} {
			if _, err := measure(bin); err != nil {
				fmt.Fprintln(stderr, "gopatina-bench:", err)
				return 1
			}
		}
	}
	a := make([]sample, 0, *runs)
	b := make([]sample, 0, *runs)
	for i := range *runs {
		order := []string{cand, base}
		if i%2 == 1 {
			order = []string{base, cand}
		}
		var got [2]sample
		for j, bin := range order {
			v, err := measure(bin)
			if err != nil {
				fmt.Fprintln(stderr, "gopatina-bench:", err)
				return 1
			}
			got[j] = v
		}
		if i%2 == 1 {
			got[0], got[1] = got[1], got[0]
		}
		a = append(a, got[0])
		b = append(b, got[1])
	}
	pick := map[string]func(sample) float64{
		"wall": func(s sample) float64 { return s.wall },
	}
	primary := "wall"
	if len(a) > 0 && a[0].instructions > 0 {
		primary = "instructions"
		pick["instructions"] = func(s sample) float64 { return s.instructions }
		pick["cycles"] = func(s sample) float64 { return s.cycles }
	}
	results := map[string]benchstat.Result{}
	for m, f := range pick {
		xa := make([]float64, len(a))
		xb := make([]float64, len(b))
		for i := range a {
			xa[i], xb[i] = f(a[i]), f(b[i])
		}
		res, err := benchstat.Equivalence(xa, xb, *margin, *sigma)
		if err != nil {
			fmt.Fprintln(stderr, "gopatina-bench:", err)
			return 1
		}
		results[m] = res
	}
	out, _ := json.MarshalIndent(report{
		Case: *name, Primary: primary, Candidate: cand, Baseline: base,
		Warmup: *warmup, Results: results, Notes: notes,
	}, "", "  ")
	fmt.Fprintf(stdout, "%s\n", out)
	for _, m := range []string{"instructions", "cycles", "wall"} {
		if r, ok := results[m]; ok {
			fmt.Fprintf(stderr, "%s %s: %s, ratio %.4f [%.4f, %.4f], %d pairs, margin %g, %g sigma\n",
				*name, m, r.Verdict, r.Ratio, r.Low, r.High, r.Pairs, r.Margin, r.Sigma)
		}
	}
	return 0
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
