package subset

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/diag"
	"github.com/pigfox/gopatina/internal/load"
)

func corpus(t *testing.T, kind string) []string {
	_, file, _, _ := runtime.Caller(0)
	dirs, err := filepath.Glob(filepath.Join(filepath.Dir(file), "..", "..", "testdata", kind, "*"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no programs: %v", err)
	}
	return dirs
}

// TestGoldenPrograms checks that every golden program is in the subset.
func TestGoldenPrograms(t *testing.T) {
	for _, dir := range corpus(t, "golden") {
		pkg, err := load.Dir(dir, dir)
		if err != nil {
			t.Fatal(err)
		}
		if ds := Check(pkg); len(ds) > 0 {
			t.Errorf("%s:\n%s", filepath.Base(dir), (&diag.Error{Dir: dir, Diags: ds}).Error())
		}
	}
}

// TestRejectPrograms runs the checker on the programs patina mode
// rejects; most are inside the subset and are rejected by the borrow
// checker.
func TestRejectPrograms(t *testing.T) {
	for _, dir := range corpus(t, "reject") {
		pkg, err := load.Dir(dir, dir)
		if err != nil {
			t.Fatal(err)
		}
		Check(pkg)
	}
}

func TestParseFormat(t *testing.T) {
	vs, err := ParseFormat("a%-+0 #8.3fb%%c%5.d%v")
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 5 || vs[0].Text != "a" || vs[2].Text != "b%c" || vs[3].Prec != 0 || vs[3].Width != 5 {
		t.Errorf("ParseFormat = %+v", vs)
	}
	f := vs[1]
	if !f.Minus || !f.Plus || !f.Zero || !f.Space || !f.Sharp || f.Width != 8 || f.Prec != 3 || f.Verb != 'f' {
		t.Errorf("flags = %+v", f)
	}
	if vs, _ := ParseFormat("%99999d"); vs[0].Width != 1000 {
		t.Errorf("a huge width = %d", vs[0].Width)
	}
	for format, want := range map[string]string{
		"%":     "ends in the middle",
		"%5":    "ends in the middle",
		"%*d":   "'*' in a directive",
		"%é":    "unknown verb",
		"%5%":   "flags on %%",
		"%e":    "the verb %e is not supported",
		"%[1]d": "'[' in a directive",
	} {
		if _, err := ParseFormat(format); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ParseFormat(%q) error = %v, want %q", format, err, want)
		}
	}
}

func TestErrorPatterns(t *testing.T) {
	head := "package main\nimport (\"errors\"; \"fmt\")\nfunc f() (int, error) { return 0, errors.New(\"x\") }\nfunc g() error { return nil }\nfunc use() { fmt.Println() }\n"
	accepted := []string{
		"func main() { if v, err := f(); err != nil { fmt.Println(err) } else if v > 0 { fmt.Println(v) } }",
		"func main() { if err := g(); err != nil { fmt.Printf(\"%v\", err) } else { fmt.Println() } }",
		"func main() { v, err := f(); if err != nil { fmt.Fprintln(nil, err.Error()); return }; fmt.Println(v) }",
		"func h() (int, error) { v, err := f(); if err != nil { return 0, err }; return v, nil }\nfunc main() { h() }",
		"func h() (string, bool, []int, error) { return \"\", false, nil, errors.New(\"e\") }\nfunc main() { h() }",
		"type T struct{ A int }\nfunc h() (T, error) { return T{}, fmt.Errorf(\"e %d\", 1) }\nfunc main() { _, _ = h() }",
		"func h() error { return g() }\nfunc main() { _ = h() }",
		"func h() (int, error) { return f() }\nfunc main() { for { v, err := h(); if err != nil { break }; _ = v } }",
		"func main() { switch { default: _, err := f(); if err != nil { return } } }",
		"func main() { for range 2 { if err := g(); err != nil { continue } } }",
		"func main() { { v, err := f(); if err != nil { return }; _ = v } }",
		"type T struct{}\nfunc main() { _ = T(T{}); ; }",
	}
	for _, src := range accepted {
		if got := check(t, head+src+"\n"); got != "" {
			t.Errorf("rejected:\n%s\n%s", src, got)
		}
	}
	rejected := map[string]string{
		"func main() { f() ; x := 1; _ = x; v, err := f(); fmt.Println(v, err) }":    "error err is not checked right after the call",
		"func main() { v, err := f(); if err != nil { fmt.Println(v) } }":            "v holds no value when err != nil",
		"func main() { if v, err := f(); err == nil { fmt.Println(v) } }":            "must check it",
		"func main() { fmt.Println(g()) }":                                           "the error from g is not checked",
		"func main() { err := errors.New(\"x\"); _ = err }":                          "errors.New is used outside a return statement",
		"func h() (int, error) { var e error; return 0, e }\nfunc main() { h() }":    "this error may be nil",
		"func h() (int, error) { return 1, errors.New(\"x\") }\nfunc main() { h() }": "must be the zero value",
		"func h() (n int, err error) { return }\nfunc main() { h() }":                "a bare return in a function that returns an error",
		"func main() { v, err := f(); if err != nil { } else { fmt.Println(v) } }":   "is not checked right after the call",
		"func main() { _, err := f(); if !(err == nil) { } }":                        "is not checked right after the call",
		"func main() { _, err := f(); if nil != err { } }":                           "is not checked right after the call",
		"func main() { var err error; _ = err }":                                     "error err is used here",
		"func main() { _, err := f(); for err != nil { break } }":                    "is not checked right after the call",
	}
	for src, want := range rejected {
		if got := check(t, head+src+"\n"); !strings.Contains(got, want) {
			t.Errorf("%s\ndiagnostics do not contain %q:\n%s", src, want, got)
		}
	}
}
