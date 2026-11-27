package emit

import (
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/borrow"
	"github.com/pigfox/gopatina/internal/fma"
	"github.com/pigfox/gopatina/internal/load"
	"github.com/pigfox/gopatina/internal/subset"
)

// TestFused checks the Rust of the float operations gc fuses: its NaN
// rules' helper for the target, an operand negated for FMSUB and FNMSUB,
// and the header naming the gc it matches.
func TestFused(t *testing.T) {
	src := program(`func f(a, b, c float64) (float64, float64, float64) {
	x := a*b + c
	y := c - a*b
	z := a*b - c
	return x, y, z
}

func main() {
	x, y, z := f(1, 2, 3)
	fmt.Println(x, y, z)
}`)
	// Lines 6 to 8, the + or - at column 11, 9 and 11.
	sites := []fma.Site{
		{Pos: fma.Pos{File: "/r/main.go", Line: 6, Col: 11}},
		{Pos: fma.Pos{File: "/r/main.go", Line: 7, Col: 9}},
		{Pos: fma.Pos{File: "/r/main.go", Line: 8, Col: 11}},
	}
	for _, c := range []struct {
		target fma.Target
		want   []string
	}{
		{fma.Target{GoVersion: "go1.27.1", GOOS: "linux", GOARCH: "arm64"}, []string{
			"// Float multiply-adds are fused (mul_add) exactly where gc go1.27.1 linux/arm64\n",
			"let x = rt::fma_arm64(b, a, c);", "let y = rt::fma_arm64(-b, a, c);", "let z = rt::fma_arm64(b, a, -c);",
		}},
		{fma.Target{GoVersion: "go1.27.1", GOOS: "linux", GOARCH: "amd64", GOAMD64: "v3"}, []string{
			"exactly where gc go1.27.1 linux/amd64 GOAMD64=v3\n", "let x = rt::fma_amd64(a, b, c);",
		}},
	} {
		pkg, err := load.Source("/r", "main.go", src, "go1.25")
		if err != nil {
			t.Fatal(err)
		}
		pkg.Fused = &fma.Sites{Target: c.target, List: sites}
		if c.target.GOARCH == "amd64" {
			pkg.Fused.List = sites[:1] // amd64 fuses no subtraction
		}
		if ds := subset.Check(pkg); len(ds) > 0 {
			t.Fatalf("%v: %v", c.target, ds)
		}
		res, _ := borrow.Check(pkg)
		var main string
		for _, f := range Emit(pkg, res, Options{Target: goldenTarget, Version: "devel"}).Files {
			if f.Path == "src/main.rs" {
				main = string(f.Data)
			}
		}
		for _, w := range c.want {
			if !strings.Contains(main, w) {
				t.Errorf("%v: main.rs lacks %q:\n%s", c.target, w, main)
			}
		}
	}
}

// TestFusedLocal checks a product held in a local and fused later: its
// factors read into locals first, the local the rounded product, and the
// float32 helpers.
func TestFusedLocal(t *testing.T) {
	src := program("func f(fs []float64, c float32) (float32, float32) {\n\tq := float32(fs[1]) * -7.5\n\tfs[1] = 9\n\treturn q, q + c\n}\n\nfunc main() {\n\tq, r := f([]float64{1, 2}, 3)\n\tfmt.Println(q, r)\n}")
	site := []fma.Site{{Pos: fma.Pos{File: "/r/main.go", Line: 8, Col: 14}}}
	for target, want := range map[fma.Target][]string{
		{GoVersion: "go1.27.1", GOOS: "linux", GOARCH: "arm64"}:                {"let factor = fs[1] as f32;\n    let q = factor * -7.5;", "rt::fma_arm64_f32(-7.5, factor, c)"},
		{GoVersion: "go1.27.1", GOOS: "linux", GOARCH: "amd64", GOAMD64: "v3"}: {"rt::fma_amd64_f32(factor, -7.5, c)"},
	} {
		pkg, err := load.Source("/r", "main.go", src, "go1.25")
		if err != nil {
			t.Fatal(err)
		}
		pkg.Fused = &fma.Sites{Target: target, List: site}
		if ds := subset.Check(pkg); len(ds) > 0 {
			t.Fatalf("%v: %v", target, ds)
		}
		res, _ := borrow.Check(pkg)
		var main string
		for _, f := range Emit(pkg, res, Options{Target: goldenTarget, Version: "devel"}).Files {
			if f.Path == "src/main.rs" {
				main = string(f.Data)
			}
		}
		for _, w := range want {
			if !strings.Contains(main, w) {
				t.Errorf("%v: main.rs lacks %q:\n%s", target, w, main)
			}
		}
	}
}

// TestNaNExact checks the Rust of float operations whose NaN a bit-level
// read may see: the target's NaN rules (rt::*_nan), float32 too, and an
// assignment operator.
func TestNaNExact(t *testing.T) {
	src := "package main\n\nimport (\n\t\"fmt\"\n\t\"math\"\n)\n\nfunc main() {\n\ta := math.NaN()\n\ta -= 1\n\tb := float32(a) / 3\n\tfmt.Println(math.Float64bits(float64(b)))\n}\n"
	pkg, err := load.Source("/r", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	if ds := subset.Check(pkg); len(ds) > 0 {
		t.Fatal(ds)
	}
	res, _ := borrow.Check(pkg)
	var main string
	for _, f := range Emit(pkg, res, Options{Target: goldenTarget, Version: "devel"}).Files {
		if f.Path == "src/main.rs" {
			main = string(f.Data)
		}
	}
	for _, w := range []string{"a = rt::sub_nan(a, 1.0_f64);", "rt::div_nan_f32(rt::to_f32(a), 3.0_f32)", "rt::to_f64(b)"} {
		if !strings.Contains(main, w) {
			t.Errorf("main.rs lacks %q:\n%s", w, main)
		}
	}
}

// TestExpFMAChoice checks which rule math.Exp's FMA choice follows on
// amd64: GODEBUG may turn FMA off below GOAMD64=v3, not from v3 on.
func TestExpFMAChoice(t *testing.T) {
	src := "package main\n\nimport (\n\t\"fmt\"\n\t\"math\"\n)\n\nfunc main() { fmt.Println(math.Exp(1)) }\n"
	for level, want := range map[string]string{"v1": "GODEBUG", "v3": "cannot turn them off"} {
		pkg, err := load.Source("/r", "main.go", src, "go1.25")
		if err != nil {
			t.Fatal(err)
		}
		pkg.Target = fma.Target{GOOS: "linux", GOARCH: "amd64", GOAMD64: level}
		if ds := subset.Check(pkg); len(ds) > 0 {
			t.Fatal(ds)
		}
		res, _ := borrow.Check(pkg)
		var rt string
		for _, f := range Emit(pkg, res, Options{Target: goldenTarget, Version: "devel"}).Files {
			if f.Path == "src/rt.rs" {
				rt = string(f.Data)
			}
		}
		if !strings.Contains(rt, want) || !strings.Contains(rt, "fn fma_usable() -> bool") {
			t.Errorf("%s: rt.rs lacks %q", level, want)
		}
	}
}
