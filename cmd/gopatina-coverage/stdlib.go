package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pigfox/gopatina/internal/coverage"
)

// The standard library measured with TinyGo's yardstick (docs/COVERAGE.md,
// "The standard library against TinyGo"). TinyGo publishes, for each of
// the Go standard library's packages, whether a program can import it and
// whether the package passes its own tests. stdlib asks the same two
// questions of GoPatina's patina mode, for the packages of TinyGo's table:
//
//   - importable: a program that only imports the package
//     (package main; import _ "P"; func main() {}) builds with
//     gopatina build -patina and runs to exit status 0;
//   - passes its tests: the package's own _test.go files, built and run
//     through GoPatina in patina mode, pass. GoPatina has no patina test
//     mode yet, so no package passes; the report says so for each, rather
//     than running go test, which would build the tests with Go.

// noTestMode is why no package passes its tests yet.
const noTestMode = "GoPatina has no patina test mode yet: a package's own _test.go files cannot be built through GoPatina"

// stdPkg is one package of the report.
type stdPkg struct {
	Path       string `json:"path"`
	Importable bool   `json:"importable"`
	Why        string `json:"why,omitempty"` // the first line of the error, when not importable
	Tests      bool   `json:"passes_tests"`
	TestsWhy   string `json:"tests_why,omitempty"`
	// From a coverage report (measure), when one is given: the package's
	// functions and how many patina mode accepts, and whether it accepts
	// every file, declaration and function, the strictest reading of
	// "importable".
	Funcs         int  `json:"funcs,omitempty"`
	FuncsAccepted int  `json:"funcs_accepted,omitempty"`
	AllAccepted   bool `json:"all_accepted,omitempty"`
	// TinyGo's answers for the same package, from the table.
	TinyGoImportable bool `json:"tinygo_importable"`
	TinyGoTests      bool `json:"tinygo_passes_tests"`
}

// stdReport is the stdlib measurement.
type stdReport struct {
	Note     string   `json:"note"`
	Table    []string `json:"table"` // the table file's comment lines: where TinyGo's answers come from
	Packages []stdPkg `json:"packages"`
	Summary  struct {
		Packages         int `json:"packages"`
		Importable       int `json:"importable"`
		AllAccepted      int `json:"all_accepted"`
		Tests            int `json:"passes_tests"`
		TinyGoImportable int `json:"tinygo_importable"`
		TinyGoTests      int `json:"tinygo_passes_tests"`
	} `json:"summary"`
}

// buildRun builds the program in dir with gopatina in patina mode and runs
// it; it returns "" if both succeed, or the first line of what went wrong.
// Replaceable in tests.
var buildRun = func(gopatina, dir string) string {
	bin := filepath.Join(dir, "prog")
	build := exec.Command(gopatina, "build", "-patina", "-o", bin, ".")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		return firstLine(string(out), err)
	}
	if out, err := exec.Command(bin).CombinedOutput(); err != nil {
		return firstLine(string(out), err)
	}
	return ""
}

// firstLine is the first non-empty line of out, or err's text.
func firstLine(out string, err error) string {
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return err.Error()
}

// readTable reads TinyGo's table: "path importable passes-tests" with
// yes or no, # comments kept for the report.
func readTable(path string) (pkgs []stdPkg, comments []string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if c, ok := strings.CutPrefix(line, "#"); ok {
			comments = append(comments, strings.TrimSpace(c))
			continue
		}
		if line == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 3 || !yesNo(f[1]) || !yesNo(f[2]) {
			return nil, nil, fmt.Errorf("%s:%d: want \"path yes|no yes|no\", got %q", path, n, line)
		}
		pkgs = append(pkgs, stdPkg{Path: f[0], TinyGoImportable: f[1] == "yes", TinyGoTests: f[2] == "yes"})
	}
	return pkgs, comments, sc.Err()
}

func yesNo(s string) bool { return s == "yes" || s == "no" }

// stdlib measures every package of the table and writes the report.
func stdlib(table, gopatina, work, covReport string, stdout, stderr io.Writer) error {
	pkgs, comments, err := readTable(table)
	if err != nil {
		return err
	}
	cov := map[string]coverage.Package{}
	if covReport != "" {
		data, err := os.ReadFile(covReport)
		if err != nil {
			return err
		}
		var r report
		if err := json.Unmarshal(data, &r); err != nil {
			return fmt.Errorf("%s: %v", covReport, err)
		}
		for _, p := range r.Packages {
			cov[p.Path] = p
		}
	}
	if work == "" {
		if work, err = os.MkdirTemp("", "gopatina-stdlib-"); err != nil {
			return err
		}
		defer os.RemoveAll(work)
	}
	r := stdReport{
		Note:  "TinyGo's yardstick: importable = a program importing only the package builds with gopatina build -patina and runs; passes tests = its own tests pass through GoPatina",
		Table: comments,
	}
	for i := range pkgs {
		p := &pkgs[i]
		dir := filepath.Join(work, fmt.Sprintf("p%03d", i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module stdlibcheck\n\ngo 1.25\n"), 0o644)
		os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport _ \""+p.Path+"\"\n\nfunc main() {}\n"), 0o644)
		p.Why = buildRun(gopatina, dir)
		p.Importable = p.Why == ""
		p.TestsWhy = noTestMode
		if c, ok := cov[p.Path]; ok {
			p.Funcs = len(c.Funcs)
			for _, f := range c.Funcs {
				p.FuncsAccepted += b2i(f.Accepted)
			}
			p.AllAccepted = c.Accepted
		}
		r.Summary.AllAccepted += b2i(p.AllAccepted)
		fmt.Fprintf(stderr, "%s: importable %v\n", p.Path, p.Importable)
		r.Summary.Packages++
		r.Summary.Importable += b2i(p.Importable)
		r.Summary.TinyGoImportable += b2i(p.TinyGoImportable)
		r.Summary.TinyGoTests += b2i(p.TinyGoTests)
	}
	r.Packages = pkgs
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
