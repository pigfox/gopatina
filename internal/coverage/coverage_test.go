package coverage

import (
	"go/ast"
	"go/token"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/diag"
	"github.com/pigfox/gopatina/internal/load"
	"github.com/pigfox/gopatina/internal/subset"
)

func TestFeature(t *testing.T) {
	for msg, want := range map[string]string{
		"package lib: only package main is supported in patina mode yet (libraries are planned)": "package-main rule",
		"package main has no func main()": "package-main rule",
		"the go statement (goroutines are planned, RFC 0002) is not supported in patina mode yet":             "goroutines and channels",
		"make of chan int is not supported in patina mode yet":                                                "goroutines and channels",
		"a map type is not supported in patina mode yet":                                                      "maps",
		"an interface type is not supported in patina mode yet":                                               "interfaces",
		"type any is not supported in patina mode yet":                                                        "interfaces",
		"a function literal (closure) is not supported in patina mode yet":                                    "closures and function values",
		"range over a function (Go 1.23) is not supported in patina mode yet":                                 "closures and function values",
		"a generic function (Go 1.18) is not supported in patina mode yet":                                    "generics",
		"defer is not supported in patina mode yet":                                                           "defer",
		"a package-level variable is not supported in patina mode yet":                                        "package-level variables",
		"package sync is not supported in patina mode yet":                                                    "imports of other packages",
		"strconv.Itoa is not supported in patina mode yet":                                                    "standard library (calls and variables)",
		"unicode/utf8.RuneLen is not supported in patina mode yet":                                            "standard library (calls and variables)",
		"github.com/x/y.F is not supported in patina mode yet":                                                "calls into other modules",
		"type uint64 is not supported in patina mode yet":                                                     "other integer and float types",
		"type []float32 is not supported in patina mode yet":                                                  "other integer and float types",
		"type io.Reader is not supported in patina mode yet":                                                  "types from other packages",
		"a pointer to a pointer is not supported in patina mode yet":                                          "pointers",
		"a slice expression (s[i:j]) is not supported in patina mode yet":                                     "slicing",
		"an array type is not supported in patina mode yet":                                                   "arrays",
		"range over [256]byte is not supported in patina mode yet":                                            "arrays",
		"an embedded field is not supported in patina mode yet":                                               "embedded and promoted fields",
		"the <- operator is not supported in patina mode yet":                                                 "goroutines and channels",
		"vendor/golang.org/x/net/http/httpguts.HeaderValuesContainsToken is not supported in patina mode yet": "standard library (calls and variables)",
		"crypto/internal/entropy/v1.0.0.Seed is not supported in patina mode yet":                             "standard library (calls and variables)",
		"the format % 7d with a value of type int is not supported in patina mode yet":                        FmtVerbs,
		"the format % x with a value of type [4]byte is not supported in patina mode yet":                     FmtVerbs,
		"type Timestamp: a struct type from another package is not supported in patina mode yet":              "types from other packages",
		"type Code: type uint32 is not supported in patina mode yet":                                          "other integer and float types",
		"type sysConn: an alias of type syscall.Conn is not supported in patina mode yet":                     "types from other packages",
		"type Set: a map type is not supported in patina mode yet":                                            "maps",
		"type H: an alias of a function type is not supported in patina mode yet":                             "closures and function values",
		"a channel receive as a statement is not supported in patina mode yet":                                "goroutines and channels",
		"range over a map (google.golang.org/grpc/metadata.MD) is not supported in patina mode yet":           "maps",
		"range over an array ([4]byte) is not supported in patina mode yet":                                   "arrays",
		"range over a pointer to an array (*[4]byte) is not supported in patina mode yet":                     "arrays",
		"a type assertion in an assignment target is not supported in patina mode yet":                        "interfaces",
		"this assignment target is not supported in patina mode yet":                                          "statement forms",
		"a range loop that assigns to existing variables (use :=) is not supported in patina mode yet":        "statement forms",
		"type unsafe.Pointer is not supported in patina mode yet":                                             Excluded,
		"printing a value whose String method may panic is not supported in patina mode yet":                  FmtMayPanic,
		"printing a slice of a type with a String or Error method is not supported in patina mode yet":        FmtVerbs,
		"the format %#v with a value of type string is not supported in patina mode yet":                      FmtVerbs,
		"the format %#v with a value of type []interface {} is not supported in patina mode yet":              FmtVerbs,
		"the format %016x with a value of type uint64 is not supported in patina mode yet":                    FmtVerbs,
		"the format %s with a value of type []byte is not supported in patina mode yet":                       FmtVerbs,
		"the format %+v with a value of type *main.CommonTlsContext is not supported in patina mode yet":      FmtVerbs,
		"printing a value of type main.Point is not supported in patina mode yet":                             FmtVerbs,
		"printing a value of type interface {} is not supported in patina mode yet":                           "interfaces",
		"the format %v with a value of type any is not supported in patina mode yet":                          "interfaces",
		"the format %s with a value of type []any is not supported in patina mode yet":                        "interfaces",
		"the format %s with a value of type time.Duration is not supported in patina mode yet":                "types from other packages",
		"the format %s with a value of type *net/http.pattern is not supported in patina mode yet":            "types from other packages",
		"printing a value of type []*github.com/x/y.ValueType is not supported in patina mode yet":            "types from other packages",
		"the format %v with a value of type main.Op is not supported in patina mode yet":                      FmtVerbs,
		"type Kind: a named type that is not a struct is not supported in patina mode yet":                    "named non-struct types",
		"a type alias is not supported in patina mode yet":                                                    "type aliases",
		"a struct field of type error is not supported in patina mode yet":                                    "struct fields of type error",
		"an anonymous struct type is not supported in patina mode yet":                                        "anonymous structs and local types",
		"reflect.ValueOf is not supported in patina mode yet":                                                 Excluded,
		"package unsafe is not supported in patina mode yet":                                                  Excluded,
		"the error from f is not checked right after the call":                                                "error handling patterns",
		"a variadic function is not supported in patina mode yet":                                             "variadic functions",
		"fallthrough is not supported in patina mode yet":                                                     "labels, goto and fallthrough",
		"the copy built-in is not supported in patina mode yet":                                               "built-ins",
		"converting an integer to a string is not supported in patina mode yet":                               "strings and formatting",
		"a shift by a non-constant count is not supported in patina mode yet":                                 "shifts by a variable count",
		"func init is not supported in patina mode yet":                                                       "init and functions without a body",
		"this expression statement is not supported in patina mode yet":                                       "statement forms",
	} {
		if got := Feature(msg); got != want {
			t.Errorf("Feature(%q) = %q, want %q", msg, got, want)
		}
	}
}

func TestGenerated(t *testing.T) {
	if !Generated([]byte("// Code generated by stringer. DO NOT EDIT.\n\npackage x\n")) {
		t.Error("generated file not recognised")
	}
	if Generated([]byte("package x\n\n// Code generated by stringer. DO NOT EDIT.\n")) {
		t.Error("a marker after the package clause does not count")
	}
}

const src = `package main

import "fmt"

var counter int

type plain struct{ n int }

type withMap struct{ m map[string]int }

type gen[P any] struct{ v P }

type gen2[P, Q any] struct {
	p P
	q Q
}

func ok(a int) int { return a + 1 }

func usesMap() int {
	m := map[string]int{}
	return len(m)
}

func usesBadType(w withMap) int { return 1 }

func callsBad() int { return usesMap() }

func usesVar() int { return counter }

func (p plain) Get() int { return p.n }

func (p *withMap) Size() int { return 0 }

func callsMethod(p plain) int { return p.Get() }

func callsBadMethod(w *withMap) int { return w.Size() }

func (g gen[P]) Val() P { return g.v }

func (g gen2[P, Q]) First() P { return g.p }

func (p (plain)) Paren() int { return 0 }

func prints() { fmt.Println("hi") }
`

func measure(t *testing.T, source string) Package {
	t.Helper()
	pkg, err := load.Source("/m", "a.go", source, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	return Measure(pkg, "example.com/m", "example.com/m", subset.Check(pkg))
}

func TestMeasure(t *testing.T) {
	p := measure(t, src)
	got := map[string]Func{}
	for _, f := range p.Funcs {
		got[f.Name] = f
	}
	for name, want := range map[string]struct {
		ok       bool
		features string
	}{
		"ok":             {true, ""},
		"usesMap":        {false, "maps"},
		"usesBadType":    {false, "maps"},
		"callsBad":       {false, "maps"},
		"usesVar":        {false, "package-level variables"},
		"plain.Get":      {true, ""},
		"withMap.Size":   {false, "maps"},
		"callsMethod":    {true, ""},
		"callsBadMethod": {false, "maps"},
		"gen.Val":        {false, "generics"},
		"gen2.First":     {false, "generics"},
		"Paren":          {true, ""},
		"prints":         {true, ""},
	} {
		f, ok := got[name]
		if !ok {
			t.Errorf("no function %s in %v", name, p.Funcs)
			continue
		}
		if f.Accepted != want.ok || !strings.Contains(strings.Join(f.Features, ","), want.features) {
			t.Errorf("%s: accepted %t features %v, want %t %q", name, f.Accepted, f.Features, want.ok, want.features)
		}
	}
	if got["ok"].Lines != 1 || got["usesMap"].Lines != 4 || got["ok"].File != "a.go" {
		t.Errorf("lines or file: %+v %+v", got["ok"], got["usesMap"])
	}
	if p.Accepted || p.Files != 1 || p.FilesOK != 0 || p.Generated != 0 {
		t.Errorf("package %+v", p)
	}
}

func TestMeasureAcceptedAndGenerated(t *testing.T) {
	p := measure(t, "package lib\n\nfunc a() int { return 1 }\n")
	if !p.Accepted || p.FilesOK != 1 || len(p.Funcs) != 1 || !p.Funcs[0].Accepted {
		t.Errorf("clean package %+v", p)
	}
	pkg, err := load.Source("/m", "a.go", "// Code generated by x. DO NOT EDIT.\n\npackage lib\n\nfunc a() int { return 1 }\n", "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	g := Measure(pkg, "m", "m", nil)
	if g.Generated != 1 || g.Files != 0 || len(g.Funcs) != 0 {
		t.Errorf("generated package %+v", g)
	}
}

func TestMeasureFileLevelAndUnclassified(t *testing.T) {
	pkg, err := load.Source("/m", "a.go", "package lib\n\nimport \"sync\"\n\nvar _ sync.Mutex\n\nfunc a() int { return 1 }\n", "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	pos := pkg.Fset.Position(pkg.Files[0].Imports[0].Pos())
	diags := []diag.Diagnostic{
		{Pos: pos, Msg: "package sync is not supported in patina mode yet"},
		{Pos: pos, Msg: "something new"},
	}
	p := Measure(pkg, "m", "m", diags)
	if p.Accepted || p.FilesOK != 0 || p.Unclassified["something new"] != 1 || !p.Funcs[0].Accepted {
		t.Errorf("file-level diagnostics: %+v", p)
	}
	_ = ast.File{}
	_ = token.NoPos
}

func pkgs() []Package {
	f := func(ok bool, lines int, feats ...string) Func {
		return Func{Name: "f", Lines: lines, Accepted: ok, Features: feats}
	}
	return []Package{
		{Path: "a", Module: "std", Files: 2, FilesOK: 1, Accepted: false, Funcs: []Func{f(true, 3), f(false, 5, "maps"), f(false, 2, "maps", "defer")}},
		{Path: "b", Module: "std", Files: 1, FilesOK: 1, Accepted: true, Funcs: []Func{f(true, 4)}},
		{Path: "c", Module: "x.org/m", Files: 1, Accepted: false, Funcs: []Func{f(false, 7, "defer"), f(false, 1, "generics", "maps")}},
		{Path: "d", Module: "y.org/n", Files: 1, Accepted: false, Funcs: []Func{f(false, 1, "generics", "interfaces")}},
		{Path: "e", Module: "y.org/n", Files: 1, Accepted: false, Funcs: []Func{{Name: "u", Lines: 9, Excluded: true, Features: []string{Excluded}}}},
	}
}

func TestSummarize(t *testing.T) {
	s := Summarize(pkgs(), 200, 1)
	a := s.All
	if a.Excluded != 1 || a.Packages != 5 || a.Functions.Num != 2 || a.Functions.Den != 7 || a.Lines.Num != 7 || a.Lines.Den != 23 {
		t.Errorf("all: %+v", a)
	}
	if a.Files.Num != 2 || a.Files.Den != 6 || a.PackagesWhole.Num != 1 {
		t.Errorf("files or packages: %+v %+v", a.Files, a.PackagesWhole)
	}
	if !(a.Functions.Low <= a.Functions.Value && a.Functions.Value <= a.Functions.High) {
		t.Errorf("interval %+v", a.Functions)
	}
	if a.Reasons[0].Feature != "maps" || a.Reasons[0].In != 3 || a.Reasons[0].Only != 1 {
		t.Errorf("reasons %+v", a.Reasons)
	}
	// defer alone unlocks one, maps alone one; ties keep the most-needed
	// first, so maps, then defer, then generics (none alone), then
	// interfaces.
	var order []string
	for _, st := range a.Roadmap {
		order = append(order, st.Feature)
	}
	if strings.Join(order, ",") != "maps,defer,generics,interfaces" || a.Roadmap[len(a.Roadmap)-1].Rate != 1 {
		t.Errorf("roadmap %+v", a.Roadmap)
	}
	if s.Std.Packages != 2 || s.ThirdParty.Packages != 3 || s.Modules["std"] != 0.5 {
		t.Errorf("groups %+v %+v %v", s.Std, s.ThirdParty, s.Modules)
	}
	if !(a.LeaveOneOutLow <= a.Functions.Value && a.Functions.Value <= a.LeaveOneOutHigh) || a.LeaveOneOutLow == a.LeaveOneOutHigh {
		t.Errorf("leave one out %v %v", a.LeaveOneOutLow, a.LeaveOneOutHigh)
	}
	// One module only: leave-one-out is the value itself.
	if g := s.Std; g.LeaveOneOutLow != g.Functions.Value || g.LeaveOneOutHigh != g.Functions.Value {
		t.Errorf("std leave one out %+v", g)
	}
}

func TestSummarizeEdges(t *testing.T) {
	s := Summarize(nil, 10, 1)
	if s.All.Functions.Value != 0 || s.All.Functions.Low != 0 || len(s.All.Roadmap) != 0 {
		t.Errorf("empty %+v", s.All)
	}
	if r := Summarize(pkgs(), 0, 1).All.Functions; r.Low != 0 || r.High != 0 || r.Den != 7 {
		t.Errorf("no bootstrap draws: %+v", r)
	}
	odd := []Package{{Path: "z", Module: "std", Funcs: []Func{{Name: "f", Accepted: false}}}}
	if r := Summarize(odd, 10, 1).All.Roadmap; len(r) != 0 {
		t.Errorf("a rejected function without features: roadmap %+v", r)
	}
}
