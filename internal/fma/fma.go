// Package fma finds where gc fuses a float multiply and an add or subtract
// into one rounding (FMADD on arm64, VFMADD231SD on amd64 from GOAMD64=v3),
// as the Go specification allows, so that GoPatina's Rust fuses exactly
// there (mul_add) and nowhere else: Rust never fuses on its own.
//
// gc is the oracle: the same Go toolchain, for the same target, reports
// each site it fuses through its fmahash debug hook
// (-gcflags=all=-d=fmahash=vy). That hook is not a stable interface, so
// every use first checks it on a probe whose fusion is known; when the
// probe fails, the sites are unknown, and the caller must refuse what gc
// might fuse rather than guess.
package fma

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// A Target is what gc's fusion depends on: the Go toolchain and the
// platform it builds for.
type Target struct {
	GoVersion string // go1.27.1
	GOOS      string
	GOARCH    string
	GOAMD64   string // v1 to v4; "" off amd64
}

func (t Target) String() string {
	s := t.GoVersion + " " + t.GOOS + "/" + t.GOARCH
	if t.GOARCH == "amd64" {
		s += " GOAMD64=" + t.GOAMD64
	}
	return s
}

// Fuses reports whether gc fuses float multiply-adds for t at all: on
// arm64 always, on amd64 from GOAMD64=v3 (AMD64.rules), never on the
// other targets patina mode builds for.
func (t Target) Fuses() bool {
	switch t.GOARCH {
	case "arm64":
		return true
	case "amd64":
		level, err := strconv.Atoi(strings.TrimPrefix(t.GOAMD64, "v"))
		return err == nil && level >= 3
	}
	return false
}

// A Pos is a source position, as gc prints it: an absolute file name, a
// line and a column (in bytes, from 1).
type Pos struct {
	File      string
	Line, Col int
}

func (p Pos) String() string { return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col) }

// A Site is one fused operation: the position of its + or -, and, when gc
// fused it in a function body inlined into another, the call sites it was
// inlined at, outermost first (Inlined[0] is in the function compiled).
type Site struct {
	Pos     Pos
	Inlined []Pos
}

func (s Site) String() string {
	var parts []string
	for _, p := range s.Inlined {
		parts = append(parts, p.String())
	}
	return strings.Join(append(parts, s.Pos.String()), "; ")
}

// Sites are the operations gc fuses in a program, for a target.
type Sites struct {
	Target Target
	List   []Site // sorted by String
}

// Has reports whether gc fuses the operation at p where it is compiled in
// its own function, not inlined.
func (s *Sites) Has(p Pos) bool {
	for _, x := range s.List {
		if x.Pos == p && len(x.Inlined) == 0 {
			return true
		}
	}
	return false
}

// triggered matches one line of the fmahash report: "fmahash triggered"
// and the positions, inline call sites first, separated by "; ", then
// the hash bits.
var triggered = regexp.MustCompile(`^fmahash triggered (\S.*) [01]+$`)

// posRE matches one position of a report line.
var posRE = regexp.MustCompile(`^(.+):(\d+):(\d+)$`)

// Parse reads gc's fmahash report from out, resolving relative file names
// against dir. Lines that are not reports (go build's package headers,
// "[bisect-match ...]") are skipped; a report line it cannot read is an
// error, since it means the report's format changed.
func Parse(out []byte, dir string) ([]Site, error) {
	seen := map[string]bool{}
	var sites []Site
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "fmahash triggered") {
			continue
		}
		m := triggered.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("unreadable fmahash report %q", line)
		}
		var ps []Pos
		for _, f := range strings.Split(m[1], "; ") {
			pm := posRE.FindStringSubmatch(f)
			if pm == nil {
				return nil, fmt.Errorf("unreadable position %q in fmahash report %q", f, line)
			}
			file := pm[1]
			if !filepath.IsAbs(file) {
				file = filepath.Join(dir, file)
			}
			l, _ := strconv.Atoi(pm[2])
			c, _ := strconv.Atoi(pm[3])
			ps = append(ps, Pos{filepath.Clean(file), l, c})
		}
		s := Site{Pos: ps[len(ps)-1], Inlined: ps[:len(ps)-1]}
		if k := s.String(); !seen[k] {
			seen[k] = true
			sites = append(sites, s)
		}
	}
	sort.Slice(sites, func(i, j int) bool { return sites[i].String() < sites[j].String() })
	return sites, nil
}

// ErrOracle is the error of a gc whose fmahash report cannot be trusted:
// the hook is missing, renamed, or reports differently.
var ErrOracle = errors.New("gc's fused multiply-add report (-d=fmahash) is not available")

// Find returns the operations gc fuses building the package in dir for
// target t, running goCmd (the go command) with env. It first checks the
// report on a probe of known fusion (Probe), and returns ErrOracle,
// wrapped, if the probe's report is not what it must be.
func Find(goCmd string, env []string, dir string, t Target) (*Sites, error) {
	if err := checked(goCmd, env, t); err != nil {
		return nil, err
	}
	sites, err := report(goCmd, env, dir, t)
	if err != nil {
		return nil, err
	}
	return &Sites{Target: t, List: sites}, nil
}

// report builds the package in dir for t with the fmahash report on, for
// every package, discarding the binary. gc's output is replayed from the
// build cache, so a cached build reports too.
func report(goCmd string, env []string, dir string, t Target) ([]Site, error) {
	cmd := exec.Command(goCmd, "build", "-gcflags=all=-d=fmahash=vy", "-o", os.DevNull, ".")
	cmd.Dir = dir
	cmd.Env = append(env, "GOOS="+t.GOOS, "GOARCH="+t.GOARCH, "GOFLAGS=")
	if t.GOARCH == "amd64" {
		cmd.Env = append(cmd.Env, "GOAMD64="+t.GOAMD64)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: go build for %s: %v\n%s", ErrOracle, t, err, out)
	}
	sites, err := Parse(out, dir)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOracle, err)
	}
	return sites, nil
}

// Probe is the self-test program: gc fuses the + of line 7 (column 13)
// on every fusing target, and nothing else in package main.
const Probe = `package main

import "os"

//go:noinline
func f(a, b, c float64) float64 {
	return a*b + c
}

func main() { os.Exit(int(f(float64(len(os.Args)), 2, 3)) - 5) }
`

// probeSite is where gc fuses in Probe.
var probeSite = Pos{Line: 7, Col: 13}

// passed are the go commands and targets whose report passed the self-test
// in this process.
var passed sync.Map

// checked runs the self-test for goCmd and t, once per process when it
// passes.
func checked(goCmd string, env []string, t Target) error {
	key := goCmd + " " + t.String()
	if _, ok := passed.Load(key); ok {
		return nil
	}
	if err := selfTest(goCmd, env, t); err != nil {
		return err
	}
	passed.Store(key, true)
	return nil
}

// selfTest checks the fmahash report on Probe for t: exactly the known
// site in package main.
func selfTest(goCmd string, env []string, t Target) error {
	dir, err := os.MkdirTemp("", "gopatina-fma-probe-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module probe\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(Probe), 0o644)
	sites, err := report(goCmd, env, dir, t)
	if err != nil {
		return err
	}
	main := filepath.Join(dir, "main.go")
	var got []Site
	for _, s := range sites {
		if s.Pos.File == main {
			got = append(got, s)
		}
	}
	want := Site{Pos: Pos{main, probeSite.Line, probeSite.Col}}
	if len(got) != 1 || got[0].String() != want.String() {
		return fmt.Errorf("%w: the probe reported %v for %s, want %v", ErrOracle, got, t, want)
	}
	return nil
}

// A Form is the operation of a fused multiply-add, x*y rounded once with
// the addend a.
type Form int

const (
	Add    Form = iota // a + x*y (FMADD, VFMADD231SD)
	SubMul             // a - x*y (FMSUB)
	MulSub             // x*y - a (FNMSUB)
)

// A Fusion is one float + or - of the program that gc fuses: Mul is its
// multiply operand, Addend the other.
type Fusion struct {
	Mul    *ast.BinaryExpr
	Addend ast.Expr
	Form   Form
}
