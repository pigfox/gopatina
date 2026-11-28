package subset

import (
	"go/importer"
	"go/types"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/load"
)

// TestNaNObservers checks that every provided function exposing a float's
// bits, a float in and an unsigned integer of its width out
// (math.Float64bits, and Float32bits when it comes), is a NaN observer,
// and that fmt's bit-level float verbs stay refused: adding one means
// adding it to NaNObservers too.
func TestNaNObservers(t *testing.T) {
	imp := importer.Default()
	for name := range StdFuncs {
		path, fn, _ := strings.Cut(name, ".")
		pkg, err := imp.Import(path)
		if err != nil {
			t.Fatal(err)
		}
		f, ok := pkg.Scope().Lookup(fn).(*types.Func)
		if !ok {
			continue
		}
		sig := f.Signature()
		if sig.Params().Len() != 1 || sig.Results().Len() != 1 {
			continue
		}
		in, ok1 := sig.Params().At(0).Type().Underlying().(*types.Basic)
		out, ok2 := sig.Results().At(0).Type().Underlying().(*types.Basic)
		exposes := ok1 && ok2 && in.Info()&types.IsFloat != 0 && out.Info()&types.IsUnsigned != 0
		if exposes && !NaNObservers[name] {
			t.Errorf("%s exposes a float's bits but is not in NaNObservers", name)
		}
	}
	for _, src := range []string{`fmt.Printf("%x", 1.5)`, `fmt.Printf("%b", 1.5)`, `fmt.Printf("%X", 1.5)`} {
		if got := check(t, "package main\nimport \"fmt\"\nfunc main() { "+src+" }\n"); got == "" {
			t.Errorf("%s is accepted: a bit-level float verb is a NaN observer (NaNObservers)", src)
		}
	}
}

// nanExact checks src and returns the source text of each operation
// marked exact, and the diagnostics.
func nanExact(t *testing.T, src string) ([]string, string) {
	t.Helper()
	pkg, err := load.Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	errs := ""
	if ds := Check(pkg); len(ds) > 0 {
		for _, d := range ds {
			errs += d.Msg + "\n"
		}
	}
	var got []string
	for n := range pkg.NaNExact {
		start, end := pkg.Fset.Position(n.Pos()), pkg.Fset.Position(n.End())
		got = append(got, src[start.Offset:end.Offset])
	}
	return got, errs
}

func TestNaNTaint(t *testing.T) {
	head := "package main\n\nimport (\n\t\"fmt\"\n\t\"math\"\n)\n\n"
	for name, c := range map[string]struct {
		src     string
		exact   []string
		refused string
	}{
		"none": {"func main() { a := 1.5; fmt.Println(a*a+1, math.Pi) }", nil, ""},
		"local": {"func main() { a := math.NaN(); b := a + 1; c := b * 2; fmt.Println(math.Float64bits(c), a*a) }",
			[]string{"a + 1", "b * 2"}, ""},
		"param": {"func f(x float64) uint64 { return math.Float64bits(x) }\nfunc main() { a := math.NaN(); fmt.Println(f(a/3), f(2)) }",
			[]string{"a/3"}, ""},
		"return": {"func g(x float64) float64 { return x - 1 }\nfunc main() { fmt.Println(math.Float64bits(g(math.NaN()))) }",
			[]string{"x - 1"}, ""},
		"conversion and sqrt": {"func main() { a := math.NaN(); fmt.Println(math.Float64bits(float64(float32(math.Sqrt(a))))) }",
			[]string{"float64(float32(math.Sqrt(a)))", "float32(math.Sqrt(a))", "math.Sqrt(a)"}, ""},
		"int conversion": {"func main() { n := 3; fmt.Println(math.Float64bits(float64(n) * 2)) }", []string{"float64(n) * 2"}, ""},
		"op assign":      {"func main() { a := math.NaN(); a -= 1; fmt.Println(math.Float64bits(a)) }", []string{"a -= 1"}, ""},
		"two nans": {"func main() { a, b := math.NaN(), math.Inf(1); fmt.Println(math.Float64bits(a + b)) }",
			[]string{"a + b"}, "a float + of two values that may both be NaN"},
		"two nans assign": {"func main() { a, b := math.NaN(), math.Inf(1); a *= b; fmt.Println(math.Float64bits(a)) }",
			[]string{"a *= b"}, "a float *= of two values that may both be NaN"},
		"var":           {"func main() { var a = math.NaN(); fmt.Println(math.Float64bits(a / 2)) }", []string{"a / 2"}, ""},
		"twice":         {"func g(x float64) float64 { return x * 3 }\nfunc main() { a := math.NaN(); fmt.Println(math.Float64bits(g(a)), math.Float64bits(g(a))) }", []string{"x * 3"}, ""},
		"closure":       {"func g(x float64) float64 { h := func() float64 { return 1 }; _ = h; return x * 3 }\nfunc main() { fmt.Println(math.Float64bits(g(math.NaN()))) }", []string{"x * 3"}, "closure"},
		"closure param": {"func main() { a := 1.5; f := func(x float64) uint64 { return math.Float64bits(x * 2) }; fmt.Println(f(a - a)) }", []string{"a - a", "x * 2"}, ""},
		"two nans sub":  {"func main() { a, b := math.NaN(), math.Inf(1); fmt.Println(math.Float64bits(a - b)) }", []string{"a - b"}, ""},
		"int bits":      {"func main() { fmt.Println(math.Float64bits(math.Float64frombits(1 << 62))) }", nil, ""},
	} {
		got, errs := nanExact(t, head+c.src+"\n")
		if c.refused == "" && errs != "" || c.refused != "" && !strings.Contains(errs, c.refused) {
			t.Errorf("%s: %s", name, errs)
		}
		if strings.Join(sorted(got), "|") != strings.Join(sorted(c.exact), "|") {
			t.Errorf("%s: exact %q, want %q", name, got, c.exact)
		}
	}
}

// TestNaNTaintAll checks that a value the analysis cannot follow makes
// every float operation one an observer may see.
func TestNaNTaintAll(t *testing.T) {
	head := "package main\n\nimport (\n\t\"fmt\"\n\t\"math\"\n)\n\n"
	for name, src := range map[string]string{
		"element":    "func main() { xs := []float64{1}; y := 3.0; fmt.Println(math.Float64bits(xs[0]), y-1) }",
		"field":      "type T struct{ F float64 }\nfunc main() { t := T{1}; y := 3.0; fmt.Println(math.Float64bits(t.F), y-1) }",
		"global":     "var G = 1.5\nfunc main() { y := 3.0; fmt.Println(math.Float64bits(G), y-1) }",
		"range":      "func main() { y := 3.0; for _, x := range []float64{1} { fmt.Println(math.Float64bits(x), y-1) } }",
		"address":    "func main() { x := 1.5; p := &x; _ = p; y := 3.0; fmt.Println(math.Float64bits(x), y-1) }",
		"results":    "func f() (float64, error) { return 1, nil }\nfunc main() { x, _ := f(); y := 3.0; fmt.Println(math.Float64bits(x), y-1) }",
		"bare":       "func f() (r float64) { r = 1; return }\nfunc main() { y := 3.0; fmt.Println(math.Float64bits(f()), y-1) }",
		"literal":    "var F = func() uint64 { x := 1.5; return math.Float64bits(x) }\nfunc main() { y := 3.0; fmt.Println(F(), y-1) }",
		"func value": "func g() float64 { return 1 }\nfunc main() { f := g; y := 3.0; fmt.Println(math.Float64bits(f()), y-1) }",
	} {
		got, errs := nanExact(t, head+src+"\n")
		if !strings.Contains(strings.Join(got, "|"), "y-1") {
			t.Errorf("%s: exact %q (%s)", name, got, errs)
		}
	}
}

func sorted(xs []string) []string {
	out := append([]string(nil), xs...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
