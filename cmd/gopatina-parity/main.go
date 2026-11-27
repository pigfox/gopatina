// Command gopatina-parity measures GoPatina on real Go programs: the
// corpora under testdata/parity (Go by Example and A Tour of Go, each with
// its licence and attribution). Generated programs (gopatina-difftest)
// overstate parity, since the generator writes what the subset accepts;
// these programs were written for people.
//
// usage: gopatina-parity [-corpus DIR] [-go PATH] [-gopatina PATH]
//
//	[-cargo PATH] [-work DIR] [-timeout D] [-build-timeout D] [-lint]
//	[-only S] [-json FILE] [-md FILE] [-baseline FILE]
//
// Each program (a NAME.go.txt file) is copied into a module of its own and
// taken through these steps, stopping at the first that fails:
//
//   - skipped, never counted as passing or failing: a program that imports
//     a package outside the standard library, uses the network, runs other
//     programs or waits for signals; that Go does not build; that does not
//     finish within -timeout with Go; or whose output differs between
//     three Go runs (it reads the clock, randomness or the environment),
//     or that the corpus's skip.txt lists, with why (its output varies, but
//     not always within three runs);
//   - refused: gopatina build -emit-rust reports diagnostics (patina mode
//     does not accept it), or crashed: it fails with none;
//   - not compiled: cargo build --release fails on the crate;
//   - mismatched: the binary's standard output, exit status, or (for a
//     nonzero status) first line of standard error differs from Go's;
//   - matched.
//
// With -lint each compiled crate is also run through cargo clippy -D
// warnings, reported separately. The report (JSON on standard output, or
// -json) gives the rates of accepted, compiled, matched and lint-clean
// programs among those tested (not skipped), each skip with its reason, and
// the failure clusters: each refusal, crash, Rust error or mismatch reduced
// to a fingerprint (the message with the program's own names, numbers and
// quoted text replaced), with the programs it covers, largest first. -md
// writes the same as Markdown. With -baseline, a run with fewer accepted,
// compiled or matched programs than the baseline report exits 3.
//
// The exit status is 0, 3 below the baseline, 1 on an error and 2 on bad
// usage. See docs/QUALITY.md.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var exit = os.Exit

func main() {
	exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// config holds the flags.
type config struct {
	corpus, goCmd, gopatina, cargo, work, only string
	jsonOut, mdOut, baseline                   string
	timeout, buildTimeout                      time.Duration
	lint                                       bool
	lang                                       string            // the go command's language version, for each go.mod
	listed                                     map[string]string // skip.txt: program to why it is skipped
}

// The stages a program ends at.
const (
	skipped    = "skipped"
	refused    = "refused"
	crashed    = "crashed"
	notBuilt   = "not compiled"
	mismatched = "mismatched"
	matched    = "matched"
)

// result is one program's outcome.
type result struct {
	Name         string   `json:"name"`
	Stage        string   `json:"stage"`
	Reason       string   `json:"reason,omitempty"`
	Fingerprints []string `json:"fingerprints,omitempty"`
	Linted       bool     `json:"linted,omitempty"`
	LintClean    bool     `json:"lint_clean,omitempty"`
}

// cluster is a fingerprint and the programs it covers.
type cluster struct {
	Fingerprint string   `json:"fingerprint"`
	Programs    []string `json:"programs"`
}

// report is the run's outcome.
type report struct {
	Programs  int            `json:"programs"`
	Skipped   int            `json:"skipped"`
	Tested    int            `json:"tested"`
	Accepted  int            `json:"accepted"`
	Compiled  int            `json:"compiled"`
	Matched   int            `json:"matched"`
	Linted    int            `json:"linted"`
	LintClean int            `json:"lint_clean"`
	Rates     map[string]any `json:"rates"`
	Skips     map[string]int `json:"skips"`
	Clusters  []cluster      `json:"clusters"`
	Results   []result       `json:"results"`
}

func run(args []string, stdout, stderr io.Writer) int {
	var c config
	fs := flag.NewFlagSet("gopatina-parity", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&c.corpus, "corpus", "testdata/parity", "directory of the corpora (NAME.go.txt files)")
	fs.StringVar(&c.goCmd, "go", "go", "the go command")
	fs.StringVar(&c.gopatina, "gopatina", "gopatina", "the gopatina command")
	fs.StringVar(&c.cargo, "cargo", "cargo", "the cargo command")
	fs.StringVar(&c.work, "work", "", "working directory (default: a temporary one, removed after)")
	fs.StringVar(&c.only, "only", "", "test only the programs whose name contains this")
	fs.StringVar(&c.jsonOut, "json", "", "write the JSON report here instead of standard output")
	fs.StringVar(&c.mdOut, "md", "", "also write a Markdown summary here")
	fs.StringVar(&c.baseline, "baseline", "", "a previous JSON report: exit 3 with fewer programs accepted, compiled or matched")
	fs.DurationVar(&c.timeout, "timeout", 10*time.Second, "limit for one run of a program")
	fs.DurationVar(&c.buildTimeout, "build-timeout", 5*time.Minute, "limit for one build of a program")
	fs.BoolVar(&c.lint, "lint", true, "also run cargo clippy -D warnings on each compiled crate")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		fmt.Fprintln(stderr, "usage: gopatina-parity [flags]")
		return 2
	}
	r, err := measure(c, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "gopatina-parity:", err)
		return 1
	}
	out, _ := json.MarshalIndent(r, "", "  ")
	if c.jsonOut == "" {
		fmt.Fprintf(stdout, "%s\n", out)
	} else if err := os.WriteFile(c.jsonOut, append(out, '\n'), 0o644); err != nil {
		fmt.Fprintln(stderr, "gopatina-parity:", err)
		return 1
	}
	if c.mdOut != "" {
		if err := os.WriteFile(c.mdOut, []byte(markdown(r)), 0o644); err != nil {
			fmt.Fprintln(stderr, "gopatina-parity:", err)
			return 1
		}
	}
	if c.baseline != "" {
		below, err := belowBaseline(r, c.baseline)
		if err != nil {
			fmt.Fprintln(stderr, "gopatina-parity:", err)
			return 1
		}
		if len(below) > 0 {
			fmt.Fprintln(stderr, "gopatina-parity: below the baseline:", strings.Join(below, ", "))
			return 3
		}
	}
	return 0
}

// measure takes every program of the corpus through the steps.
func measure(c config, progress io.Writer) (report, error) {
	names, err := programs(c.corpus, c.only)
	if err != nil {
		return report{}, err
	}
	if c.lang, err = goLang(c.goCmd); err != nil {
		return report{}, err
	}
	if c.listed, err = skipList(filepath.Join(c.corpus, "skip.txt")); err != nil {
		return report{}, err
	}
	if c.work == "" {
		dir, err := os.MkdirTemp("", "gopatina-parity-")
		if err != nil {
			return report{}, err
		}
		defer os.RemoveAll(dir)
		c.work = dir
	}
	var results []result
	for i, name := range names {
		res, err := program(c, name, filepath.Join(c.work, strconv.Itoa(i)))
		if err != nil {
			return report{}, fmt.Errorf("%s: %v", name, err)
		}
		fmt.Fprintf(progress, "%s: %s %s\n", name, res.Stage, res.Reason)
		results = append(results, res)
	}
	return summarize(results), nil
}

// programs lists the corpus's programs, by name (the path under the
// corpus, without .go.txt), sorted.
func programs(corpus, only string) ([]string, error) {
	var names []string
	err := filepath.WalkDir(corpus, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name, ok := strings.CutSuffix(path, ".go.txt"); ok && !d.IsDir() {
			rel, _ := filepath.Rel(corpus, name)
			if strings.Contains(rel, only) {
				names = append(names, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	sort.Strings(names)
	return names, err
}

// program takes one program through the steps, in its own directory dir.
func program(c config, name, dir string) (result, error) {
	res := result{Name: name}
	if why, ok := c.listed[name]; ok {
		res.Stage, res.Reason = skipped, why+" (skip.txt)"
		return res, nil
	}
	src, err := os.ReadFile(filepath.Join(c.corpus, filepath.FromSlash(name)+".go.txt"))
	if err != nil {
		return res, err
	}
	src = dropOmit(src)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return res, err
	}
	os.WriteFile(filepath.Join(dir, "main.go"), src, 0o644)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module parity\n\ngo "+c.lang+"\n"), 0o644)
	skip := func(why string) (result, error) {
		res.Stage, res.Reason = skipped, why
		return res, nil
	}
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", src, parser.SkipObjectResolution)
	if err != nil {
		return skip("does not parse")
	}
	if why := importSkip(file); why != "" {
		return skip(why)
	}
	goBin := filepath.Join(dir, "go.bin")
	if _, err := tool(c.buildTimeout, dir, c.goCmd, "build", "-o", goBin, "."); err != nil {
		return skip("does not build with Go")
	}
	want, timedOut := execute(goBin, dir, c.timeout)
	if timedOut {
		return skip(fmt.Sprintf("does not finish within %v with Go", c.timeout))
	}
	for range 2 { // three runs in all: two can agree by chance
		if again, _ := execute(goBin, dir, c.timeout); again != want {
			return skip("its output differs between Go runs (clock, randomness or environment)")
		}
	}
	names := declared(file)
	crate := filepath.Join(dir, "crate")
	if out, err := tool(c.buildTimeout, dir, c.gopatina, "build", "-emit-rust", "-o", crate, "."); err != nil {
		res.Stage = refused
		res.Fingerprints = fingerprints(diagnostics(out), names) // about the program: its names go
		if len(res.Fingerprints) == 0 {
			res.Stage = crashed
			res.Fingerprints = fingerprints([]string{"gopatina: " + firstLine(out)}, nil)
		}
		return res, nil
	}
	target := filepath.Join(c.work, "target") // shared: the crates build alike
	env := []string{"CARGO_TARGET_DIR=" + target}
	if out, err := toolEnv(c.buildTimeout, crate, env, c.cargo, "build", "--release", "--quiet"); err != nil {
		res.Stage = notBuilt
		res.Fingerprints = fingerprints([]string{"rustc: " + firstMatch(out, "error")}, nil)
		return res, nil
	}
	if c.lint {
		out, err := toolEnv(c.buildTimeout, crate, env, c.cargo, "clippy", "--release", "--quiet", "--", "-D", "warnings")
		res.Linted, res.LintClean = true, err == nil
		if err != nil {
			res.Fingerprints = fingerprints([]string{"clippy: " + firstMatch(out, "error")}, nil)
		}
	}
	bin := filepath.Join(target, crateTarget(crate), "release", crateName(crate))
	got, _ := execute(bin, dir, c.timeout)
	if why := differs(want, got); why != "" {
		res.Stage = mismatched
		res.Fingerprints = append(res.Fingerprints, "output: "+why)
		return res, nil
	}
	res.Stage = matched
	return res, nil
}

// skipList reads the corpus's skip.txt, lines "NAME: why" (# begins a
// comment): programs known to vary from run to run, such as by a map's
// iteration order, which Go randomises, so that three runs can agree by
// chance. A missing file lists none.
func skipList(file string) (map[string]string, error) {
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	listed := map[string]string{}
	for i, l := range strings.Split(string(data), "\n") {
		l, _, _ = strings.Cut(l, "#")
		if strings.TrimSpace(l) == "" {
			continue
		}
		name, why, ok := strings.Cut(l, ":")
		if !ok || strings.TrimSpace(why) == "" {
			return nil, fmt.Errorf("%s:%d: want NAME: why", file, i+1)
		}
		listed[strings.TrimSpace(name)] = strings.TrimSpace(why)
	}
	return listed, nil
}

// dropOmit removes the tour's build constraints naming OMIT (//go:build
// OMIT, or norun || OMIT, nobuild || OMIT), which keep its files out of
// the website's own build. A nobuild program then fails to build with Go,
// and is skipped as such; a norun one runs, as the tour's reader runs it.
func dropOmit(src []byte) []byte {
	var out []string
	for _, l := range strings.SplitAfter(string(src), "\n") {
		if t := strings.TrimSpace(l); strings.HasPrefix(t, "//go:build ") && strings.Contains(t, "OMIT") {
			continue
		}
		out = append(out, l)
	}
	return []byte(strings.Join(out, ""))
}

// goLang returns the language version of the go command, such as 1.27,
// which each program's go.mod names: the corpora use the language as it
// is now (new(42) needs Go 1.26).
func goLang(goCmd string) (string, error) {
	out, err := exec.Command(goCmd, "env", "GOVERSION").Output()
	m := regexp.MustCompile(`^go(\d+\.\d+)`).FindStringSubmatch(strings.TrimSpace(string(out)))
	if err != nil || m == nil {
		return "", fmt.Errorf("%s env GOVERSION: %q, %v", goCmd, out, err)
	}
	return m[1], nil
}

// importSkip returns why a program's imports keep it out, or "".
func importSkip(f *ast.File) string {
	for _, im := range f.Imports {
		path, _ := strconv.Unquote(im.Path.Value)
		first, _, _ := strings.Cut(path, "/")
		switch {
		case strings.Contains(first, "."):
			return "imports " + path + ", not in the standard library"
		case first == "net":
			return "uses the network (" + path + ")"
		case path == "os/exec":
			return "runs other programs (os/exec)"
		case path == "os/signal":
			return "waits for signals (os/signal)"
		}
	}
	return ""
}

// tool runs a command in dir, stopped after limit, and returns its
// combined output.
func tool(limit time.Duration, dir, name string, args ...string) ([]byte, error) {
	return toolEnv(limit, dir, nil, name, args...)
}

func toolEnv(limit time.Duration, dir string, env []string, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	return cmd.CombinedOutput()
}

// outcome is what a run printed and how it ended.
type outcome struct {
	stdout, stderrLine string
	code               int
}

// execute runs bin in dir, with no input and no arguments, stopped after
// limit.
func execute(bin, dir string, limit time.Duration) (outcome, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	cmd.Dir = dir
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	cmd.Run()
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	return outcome{o.String(), firstLine(e.Bytes()), code}, ctx.Err() != nil
}

// differs says how got differs from want, or "".
func differs(want, got outcome) string {
	switch {
	case want.code != got.code:
		return "exit status differs"
	case want.stdout != got.stdout:
		return "standard output differs"
	case want.code != 0 && want.stderrLine != got.stderrLine:
		return "first line of standard error differs"
	}
	return ""
}

func firstLine(b []byte) string {
	line, _, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
	return line
}

// firstMatch returns the first line of out that starts with prefix, or
// its first line.
func firstMatch(out []byte, prefix string) string {
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, prefix) {
			return l
		}
	}
	return firstLine(out)
}

// crateName returns the package name in crate's Cargo.toml.
func crateName(crate string) string {
	toml, _ := os.ReadFile(filepath.Join(crate, "Cargo.toml"))
	if m := regexp.MustCompile(`(?m)^name\s*=\s*"([^"]+)"`).FindSubmatch(toml); m != nil {
		return string(m[1])
	}
	return "main"
}

// crateTarget returns the target triple crate's .cargo/config.toml names
// (cargo then builds into target/TRIPLE/release), or "".
func crateTarget(crate string) string {
	conf, _ := os.ReadFile(filepath.Join(crate, ".cargo", "config.toml"))
	if m := regexp.MustCompile(`(?m)^target\s*=\s*"([^"]+)"`).FindSubmatch(conf); m != nil {
		return string(m[1])
	}
	return ""
}

// diagRE matches a diagnostic's first line, file:line:col: message.
var diagRE = regexp.MustCompile(`(?m)^(\S+):\d+:\d+: (.+)$`)

// diagnostics returns the messages of the diagnostics in out. One in a
// standard package's source, which patina mode compiles where it does not
// provide the package, stands for the program's own call that reaches it,
// the first function of its "reached from main through:" chain outside
// package main: what to provide is filepath.Join, not the internal
// package five calls below it. Without a chain it stands for its package.
// Either way the package's own constructs are not the program's.
func diagnostics(out []byte) []string {
	var msgs []string
	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		m := diagRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		file, msg := m[1], m[2]
		if _, rest, ok := strings.Cut(filepath.ToSlash(file), "/src/"); ok && filepath.IsAbs(file) {
			msg = "uses the standard package " + path.Dir(rest) + ", which patina mode does not support yet"
			if call := stdCall(lines[i+1:]); call != "" {
				msg = "calls " + call + ", whose standard-library code patina mode does not support yet"
			}
		}
		msgs = append(msgs, msg)
	}
	return msgs
}

// stdCall returns the program's call into the standard library from the
// "reached from main through:" line among a diagnostic's indented lines:
// the first function in the chain outside package main, or "" when there
// is none, or the chain starts in a package's initialization (aes.init,
// unicode.Properties), which no call of the program names.
func stdCall(follow []string) string {
	for _, line := range follow {
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break // the next diagnostic
		}
		_, chain, ok := strings.Cut(line, "reached from main through: ")
		if !ok {
			continue
		}
		fns := strings.Split(strings.TrimSpace(chain), " -> ")
		if !strings.HasPrefix(fns[0], "main.") {
			return ""
		}
		for _, fn := range fns {
			if !strings.HasPrefix(fn, "main.") {
				return fn
			}
		}
	}
	return ""
}

// declared returns the names the program itself declares: its functions,
// types, variables, constants, fields, parameters and labels.
func declared(f *ast.File) map[string]bool {
	names := map[string]bool{}
	add := func(ids ...*ast.Ident) {
		for _, id := range ids {
			if id != nil && id.Name != "_" && id.Name != "main" {
				names[id.Name] = true
			}
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			add(x.Name)
		case *ast.TypeSpec:
			add(x.Name)
		case *ast.ValueSpec:
			add(x.Names...)
		case *ast.Field:
			add(x.Names...)
		case *ast.LabeledStmt:
			add(x.Label)
		case *ast.AssignStmt:
			if x.Tok == token.DEFINE {
				for _, l := range x.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						add(id)
					}
				}
			}
		case *ast.RangeStmt:
			if x.Tok == token.DEFINE {
				for _, e := range []ast.Expr{x.Key, x.Value} {
					if id, ok := e.(*ast.Ident); ok {
						add(id)
					}
				}
			}
		}
		return true
	})
	return names
}

var (
	backtickRE = regexp.MustCompile("`[^`]*`")
	quoteRE    = regexp.MustCompile(`"[^"]*"`)
	numberRE   = regexp.MustCompile(`\b\d+\b`)
	wordRE     = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
)

// fingerprints reduces messages to their shape, once each, in order: the
// program's own names (names; nil for a message about GoPatina or Rust,
// whose words are not the program's) become _, numbers N, quoted text "S"
// or `_`.
func fingerprints(msgs []string, names map[string]bool) []string {
	var fps []string
	seen := map[string]bool{}
	for _, m := range msgs {
		m = backtickRE.ReplaceAllString(m, "`_`")
		m = quoteRE.ReplaceAllString(m, `"S"`)
		m = numberRE.ReplaceAllString(m, "N")
		if strings.HasPrefix(m, "calls ") || strings.HasPrefix(m, "uses the standard package ") {
			// A standard-library call or package (diagnostics): none of
			// its names is the program's, though the program may declare
			// the same (a variable named like the package os/signal's
			// last element).
			if !seen[m] {
				seen[m] = true
				fps = append(fps, m)
			}
			continue
		}
		m = wordRE.ReplaceAllStringFunc(m, func(w string) string {
			if names[w] {
				return "_"
			}
			return w
		})
		if !seen[m] {
			seen[m] = true
			fps = append(fps, m)
		}
	}
	return fps
}

// summarize counts the results and clusters their fingerprints.
func summarize(results []result) report {
	r := report{Programs: len(results), Skips: map[string]int{}, Results: results}
	byPrint := map[string][]string{}
	for _, res := range results {
		if res.Stage == skipped {
			r.Skipped++
			r.Skips[res.Reason]++
			continue
		}
		r.Tested++
		switch res.Stage {
		case notBuilt:
			r.Accepted++
		case mismatched:
			r.Accepted++
			r.Compiled++
		case matched:
			r.Accepted++
			r.Compiled++
			r.Matched++
		}
		if res.Linted {
			r.Linted++
			if res.LintClean {
				r.LintClean++
			}
		}
		for _, fp := range res.Fingerprints {
			byPrint[fp] = append(byPrint[fp], res.Name)
		}
	}
	for fp, ps := range byPrint {
		r.Clusters = append(r.Clusters, cluster{fp, ps})
	}
	sort.Slice(r.Clusters, func(i, j int) bool {
		a, b := r.Clusters[i], r.Clusters[j]
		if len(a.Programs) != len(b.Programs) {
			return len(a.Programs) > len(b.Programs)
		}
		return a.Fingerprint < b.Fingerprint
	})
	r.Rates = map[string]any{
		"accepted":   rate(r.Accepted, r.Tested),
		"compiled":   rate(r.Compiled, r.Tested),
		"matched":    rate(r.Matched, r.Tested),
		"lint_clean": rate(r.LintClean, r.Linted),
	}
	return r
}

// rate is n of total as a percentage with one decimal, or nil for none.
func rate(n, total int) any {
	if total == 0 {
		return nil
	}
	v, _ := strconv.ParseFloat(strconv.FormatFloat(100*float64(n)/float64(total), 'f', 1, 64), 64)
	return v
}

// markdown renders the report's summary.
func markdown(r report) string {
	var b strings.Builder
	pct := func(k string) string {
		if v, ok := r.Rates[k].(float64); ok {
			return strconv.FormatFloat(v, 'f', 1, 64) + "%"
		}
		return "n/a"
	}
	fmt.Fprintf(&b, "# GoPatina parity\n\n%d programs: %d tested, %d skipped.\n\n", r.Programs, r.Tested, r.Skipped)
	fmt.Fprintf(&b, "| of %d tested | programs | rate |\n|---|---|---|\n", r.Tested)
	fmt.Fprintf(&b, "| accepted by patina mode | %d | %s |\n", r.Accepted, pct("accepted"))
	fmt.Fprintf(&b, "| compiled to Rust | %d | %s |\n", r.Compiled, pct("compiled"))
	fmt.Fprintf(&b, "| output matches Go | %d | %s |\n", r.Matched, pct("matched"))
	fmt.Fprintf(&b, "| clippy clean (of %d linted) | %d | %s |\n", r.Linted, r.LintClean, pct("lint_clean"))
	if len(r.Clusters) > 0 {
		b.WriteString("\n## Failure clusters, largest first\n\n| programs | fingerprint | examples |\n|---|---|---|\n")
		for _, cl := range r.Clusters {
			ex := cl.Programs
			if len(ex) > 3 {
				ex = ex[:3]
			}
			fmt.Fprintf(&b, "| %d | %s | %s |\n", len(cl.Programs), strings.ReplaceAll(cl.Fingerprint, "|", `\|`), strings.Join(ex, ", "))
		}
	}
	if r.Skipped > 0 {
		b.WriteString("\n## Skipped (not counted)\n\n| programs | reason |\n|---|---|\n")
		var reasons []string
		for why := range r.Skips {
			reasons = append(reasons, why)
		}
		sort.Strings(reasons)
		for _, why := range reasons {
			fmt.Fprintf(&b, "| %d | %s |\n", r.Skips[why], why)
		}
	}
	return b.String()
}

// belowBaseline returns the rates of r below those of the report in file.
func belowBaseline(r report, file string) ([]string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var base report
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, fmt.Errorf("%s: %v", file, err)
	}
	// Counts, not rates: a program whose output only sometimes varies may
	// move between skipped and tested, which moves the rates' denominator;
	// a count falls only when a program stops getting through.
	var below []string
	for _, k := range []struct {
		name     string
		now, was int
	}{{"accepted", r.Accepted, base.Accepted}, {"compiled", r.Compiled, base.Compiled}, {"matched", r.Matched, base.Matched}} {
		if k.now < k.was {
			below = append(below, fmt.Sprintf("%s %d programs (baseline %d)", k.name, k.now, k.was))
		}
	}
	return below, nil
}
