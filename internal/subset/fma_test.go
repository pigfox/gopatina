package subset

import (
	"errors"
	"go/ast"
	"go/types"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/diag"
	"github.com/pigfox/gopatina/internal/fma"
	"github.com/pigfox/gopatina/internal/load"
)

var (
	arm64 = fma.Target{GoVersion: "go1.27.1", GOOS: "linux", GOARCH: "arm64"}
	v3    = fma.Target{GoVersion: "go1.27.1", GOOS: "linux", GOARCH: "amd64", GOAMD64: "v3"}
)

// at returns the position of the n-th occurrence (from 0) of op on the
// line containing marker, as gc reports a fused site.
func at(t *testing.T, src, marker, op string, n int) fma.Pos {
	t.Helper()
	for i, line := range strings.Split(src, "\n") {
		if !strings.Contains(line, marker) {
			continue
		}
		col := -1
		for k, rest := 0, line; k <= n; k++ {
			j := strings.Index(rest, op)
			if j < 0 {
				t.Fatalf("%q not in %q", op, line)
			}
			col += j + 1
			rest = rest[j+1:]
		}
		return fma.Pos{File: "/p/main.go", Line: i + 1, Col: col + 1}
	}
	t.Fatalf("no line with %q", marker)
	return fma.Pos{}
}

// fused checks src with gc's report sites for target, and returns the
// package and the diagnostics.
func fused(t *testing.T, src string, target fma.Target, sites ...fma.Site) (*load.Package, string) {
	t.Helper()
	pkg, err := load.Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	pkg.Fused = &fma.Sites{Target: target, List: sites}
	ds := Check(pkg)
	if len(ds) == 0 {
		return pkg, ""
	}
	return pkg, (&diag.Error{Dir: "/p", Diags: ds}).Error()
}

const fmaSrc = `package main

import "fmt"

func f(a, b, c, d float64) float64 {
	x := a*b + c       // add
	y := a*b + c*d     // two
	w := c - a*b       // sub
	v := a*b - c       // msub
	u := a/4 + c       // div
	s := a*2 + c       // double
	r := (a * 0.5) + c // half
	n := 1 + 2
	return x + y + w + v + u + s + r + float64(n)
}

func main() { fmt.Println(f(1, 2, 3, 4)) }
`

// mulText renders a fusion's multiply as source.
func mulText(f fma.Fusion) string {
	x, _ := f.Mul.X.(*ast.Ident)
	y, ok := f.Mul.Y.(*ast.Ident)
	if !ok {
		return x.Name + "*k"
	}
	return x.Name + "*" + y.Name
}

func TestFusionForms(t *testing.T) {
	site := func(marker, op string, n int) fma.Site { return fma.Site{Pos: at(t, fmaSrc, marker, op, n)} }
	pkg, errs := fused(t, fmaSrc, arm64,
		site("// add", "+", 0), site("// two", "+", 0), site("// sub", "-", 0), site("// msub", "-", 0), site("// half", "+", 0))
	if errs != "" {
		t.Fatal(errs)
	}
	got := map[string]string{}
	for e, f := range pkg.Fusions {
		got[pkg.Fset.Position(e.OpPos).String()] = mulText(f) + " " + map[fma.Form]string{fma.Add: "add", fma.SubMul: "a-xy", fma.MulSub: "xy-a"}[f.Form]
	}
	want := map[string]string{
		"/p/main.go:6:11":  "a*b add",
		"/p/main.go:7:11":  "c*d add", // arm64 tries the left operand as the addend first
		"/p/main.go:8:9":   "a*b a-xy",
		"/p/main.go:9:11":  "a*b xy-a",
		"/p/main.go:12:17": "a*k add",
	}
	if len(got) != len(want) {
		t.Errorf("fusions %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %q, want %q (all %v)", k, got[k], v, got)
		}
	}
	// amd64 tries the left operand as the multiply first.
	pkg, errs = fused(t, fmaSrc, v3, site("// two", "+", 0))
	for _, f := range pkg.Fusions {
		if errs != "" || mulText(f) != "a*b" {
			t.Errorf("v3: %s %s", mulText(f), errs)
		}
	}
}

func TestFusionRefused(t *testing.T) {
	site := func(marker, op string, n int) fma.Site { return fma.Site{Pos: at(t, fmaSrc, marker, op, n)} }
	for name, c := range map[string]struct {
		target fma.Target
		site   fma.Site
		want   string
	}{
		"divide":    {arm64, site("// div", "+", 0), "a float + that gc fuses with a multiply for go1.27.1 linux/arm64, whose multiply is not one of its operands, is not supported"},
		"double":    {arm64, site("// double", "+", 0), "whose multiply is not one of its operands"},
		"inlined":   {arm64, fma.Site{Pos: at(t, fmaSrc, "// add", "+", 0), Inlined: []fma.Pos{{File: "/p/main.go", Line: 17, Col: 28}}}, "a float multiply-add gc fuses for go1.27.1 linux/arm64 where it inlines this function (at /p/main.go:17:28) is not supported"},
		"not an op": {arm64, fma.Site{Pos: fma.Pos{File: "/p/main.go", Line: 6, Col: 2}}, "an operation gc fuses into a float multiply-add here for go1.27.1 linux/arm64, which is not a float + or -, is not supported"},
		"v3 sub":    {v3, site("// sub", "-", 0), "a float - that gc fuses with a multiply for go1.27.1 linux/amd64 GOAMD64=v3"},
	} {
		if _, errs := fused(t, fmaSrc, c.target, c.site); !strings.Contains(errs, c.want) {
			t.Errorf("%s: %s", name, errs)
		}
	}
	// A site in code GoPatina does not write is not the program's.
	if _, errs := fused(t, fmaSrc, arm64, fma.Site{Pos: fma.Pos{File: "/usr/local/go/src/math/sin.go", Line: 1, Col: 1}}); errs != "" {
		t.Errorf("outside: %s", errs)
	}
	src32 := "package main\n\nimport \"fmt\"\n\nfunc f(a, b, c float32) float32 { return a*b + c } // f32\n\nfunc main() { fmt.Println(f(1, 2, 3)) }\n"
	if _, errs := fused(t, src32, arm64, fma.Site{Pos: at(t, src32, "// f32", "+", 0)}); !strings.Contains(errs, "a float32 multiply-add gc fuses for go1.27.1 linux/arm64 is not supported") {
		t.Errorf("float32: %s", errs)
	}
}

// TestFusionUnknown checks that, when gc's report is not available on a
// target that fuses, every float + and - is refused, and nothing else.
func TestFusionUnknown(t *testing.T) {
	pkg, err := load.Source("/p", "main.go", fmaSrc, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	pkg.FusedErr = errors.New("the report is missing")
	ds := Check(pkg)
	n := 0
	for _, d := range ds {
		if strings.Contains(d.Msg, "which gc may fuse with a multiply for this target, while the report is missing") {
			n++
		}
	}
	// 7 statements of one + or -, the return's seven +, and float64(n)'s:
	// every float + and -, not 1 + 2.
	if n != 14 || len(ds) != n {
		t.Errorf("%d of %d: %v", n, len(ds), ds)
	}
	// A target that never fuses: nothing to match.
	pkg, _ = load.Source("/p", "main.go", fmaSrc, "go1.25")
	if ds := Check(pkg); len(ds) != 0 || pkg.Fusions != nil {
		t.Errorf("no fusion: %v %v", ds, pkg.Fusions)
	}
	_ = types.Float64
}
