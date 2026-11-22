package subset

import (
	"go/types"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/diag"
	"github.com/pigfox/gopatina/internal/load"
)

// check loads src as main.go and returns the subset diagnostics, rendered.
func check(t *testing.T, src string) string {
	t.Helper()
	pkg, err := load.Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ds := Check(pkg)
	if len(ds) == 0 {
		return ""
	}
	return (&diag.Error{Dir: "/p", Diags: ds}).Error()
}

// TestNamedTypes checks the programs RFC 0007 accepts.
func TestNamedTypes(t *testing.T) {
	src := `package main

import "fmt"

type Op int

const (
	Add Op = iota
	Sub
)

func (o Op) String() string {
	switch o {
	case Add:
		return "+"
	}
	if o > Sub && len("x") == 1 {
		return fmt.Sprintf("Op(%d)", int(o)+0*2)
	}
	return "-"
}

type E int

func (e E) Error() string { return "e" }

type P int

func (p *P) String() string { return fmt.Sprint(10 / int(*p)) }

type Path []int

func (p *Path) Push(x int) { *p = append(*p, x) }

type Name string

type S struct{ X int }

type S2 S

type Count = int

type Names = []Name

func main() {
	var p Path
	p.Push(3)
	var n Count = len(p)
	var ns Names = []Name{"a"}
	s := S2(S{X: 1})
	q := P(2)
	fmt.Println(Add, Sub, E(1), q, p, n, ns, s.X, Name("x"), Path([]int{1}))
	fmt.Printf("%v %s %d %5v\n", Add, Add, Add, E(2))
}
`
	if got := check(t, src); got != "" {
		t.Errorf("unexpected diagnostics:\n%s", got)
	}
}

func TestAccepted(t *testing.T) {
	src := `package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

type T struct {
	A int
	B string
	C []byte
}

func (t *T) M(w io.Writer) { fmt.Fprintf(w, "%d %s %s\n", t.A, t.B, t.C) }

func f(x int) (int, error) {
	if x < 0 {
		return 0, errors.New("neg")
	}
	return x, nil
}

func main() {
	const k = 3
	var p *T = &T{A: k}
	q := &p.A
	*q = 4
	p.M(os.Stdout)
	s := []int{1, 2}
	s = append(s, 3)
	s = slices.Clone(s)
	b := bytes.Clone([]byte("x"))
	b = append(b, "yz"...)
	var none []int = nil
	v, err := f(len(s) + len(none) + len(b))
	if err != nil {
		os.Exit(1)
	}
	fmt.Println(v, strings.Repeat("a", v), s, float64(v)/2, int32(v), uint8(v), int64(v))
	fmt.Println(min(v, 2), max(v, 1, 9), min(int32(v), -1), max(uint8(v), 3))
	for i := range 3 {
		fmt.Print(i)
	}
	if v > 100 {
		panic("too big")
	}
	switch v {
	case 1:
		break
	}
}
`
	if got := check(t, src); got != "" {
		t.Errorf("unexpected diagnostics:\n%s", got)
	}
}

func TestRejected(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"library", "package lib\n", "package lib: only package main is supported in patina mode yet"},
		{"nomain", "package main\nfunc f() {}\n", "package main has no func main()"},
		{"unsafe", "package main\nimport \"unsafe\"\nvar _ = unsafe.Sizeof(0)\nfunc main() {}\n", "package unsafe is not supported"},
		{"otherpkg", "package main\nimport \"sort\"\nfunc main() { sort.Ints(nil) }\n", "package sort is not supported"},
		{"renamed", "package main\nimport f \"fmt\"\nfunc main() { f.Println() }\n", "a renamed or dot import is not supported"},
		{"pkgvar", "package main\nvar x int\nfunc main() { _ = x }\n", "a package-level variable is not supported"},
		{"generic type", "package main\ntype B[T any] struct{ v T }\nfunc main() {}\n", "a generic type (Go 1.18) is not supported"},
		{"generic method", "package main\ntype B[T any] struct{ v T }\nfunc (b B[T]) M() {}\nfunc main() {}\n", "a method on a generic type (Go 1.18)"},
		{"generic instance", "package main\ntype B[T any] struct{ v T }\nfunc f(b B[int]) {}\nfunc main() {}\n", "a generic type (Go 1.18)"},
		{"closure", "package main\nfunc main() { f := func() {}; _ = f }\n", "a function literal (closure) is not supported"},
		{"slice expr", "package main\nfunc main() { s := []int{1}; _ = s[0:1] }\n", "a slice expression (s[i:j]) is not supported"},
		{"type assert", "package main\nfunc main() { var x any; _ = x.(int) }\n", "a type assertion is not supported"},
		{"promoted", "package main\ntype E struct{ X int }\ntype S struct{ E }\nfunc main() { _ = S{}.X }\n", "a promoted field is not supported"},
		{"call literal", "package main\nfunc main() { func() {}() }\n", "calling a function value is not supported"},
		{"call func var", "package main\nfunc main() { var f func(); f() }\n", "calling a function value is not supported"},
		{"call generic", "package main\nfunc g[T any](x T) {}\nfunc main() { g[int](1) }\n", "an instantiated generic function (Go 1.18) is not supported"},
		{"call generic2", "package main\nfunc g[T, U any](x T, y U) {}\nfunc main() { g[int, int](1, 2) }\n", "an instantiated generic function (Go 1.18) is not supported"},
		{"alias pointer", "package main\ntype A = *int\nfunc main() {}\n", "a type alias of a pointer type is not supported"},
		{"alias error", "package main\ntype E = error\nfunc main() {}\n", "a type alias of error or io.Writer is not supported"},
		{"alias map", "package main\ntype M = map[int]int\nfunc main() {}\n", "type M: an alias of a map type is not supported"},
		{"local alias", "package main\nfunc main() { type L = int; var x L; _ = x }\n", "a type declared inside a function is not supported"},
		{"local type", "package main\nfunc main() { type L struct{}; _ = L{} }\n", "a type declared inside a function is not supported"},
		{"named map", "package main\ntype C map[string]int\nfunc main() {}\n", "type C: a map type is not supported"},
		{"named foreign struct", "package main\nimport \"strings\"\ntype B strings.Builder\nfunc main() {}\n", "type B: a struct type from another package is not supported"},
		{"print panicky String", "package main\nimport \"fmt\"\ntype K int\nfunc (k K) String() string { return fmt.Sprint(10 / int(k)) }\nfunc main() { fmt.Println(K(1)) }\n", "printing a value whose String method may panic is not supported"},
		{"print panicky Error", "package main\nimport \"fmt\"\ntype K []int\nfunc (k K) Error() string { return fmt.Sprint(k[0]) }\nfunc main() { fmt.Printf(\"%v\\n\", K{1}) }\n", "printing a value whose Error method may panic is not supported"},
		{"print String calls", "package main\nimport \"fmt\"\ntype K int\nfunc g() string { return \"\" }\nfunc (k K) String() string { return g() }\nfunc main() { fmt.Print(K(1)) }\n", "printing a value whose String method may panic is not supported"},
		{"print String nested", "package main\nimport \"fmt\"\ntype K int\nfunc (k K) String() string { return fmt.Sprint(k + 1) }\nfunc main() { fmt.Print(K(1)) }\n", "printing a value whose String method may panic is not supported"},
		{"print String loop", "package main\nimport \"fmt\"\ntype K int\nfunc (k K) String() string { for { } }\nfunc main() { fmt.Print(K(1)) }\n", "printing a value whose String method may panic is not supported"},
		{"print panicky String slice", "package main\nimport \"fmt\"\ntype K int\nfunc (k K) String() string { return fmt.Sprint(10 / int(k)) }\nfunc main() { fmt.Println([]K{1}) }\n", "printing a value whose String method may panic is not supported"},
		{"print String hex plus", "package main\nimport \"fmt\"\ntype K int\nfunc (k K) String() string { return \"k\" }\nfunc main() { fmt.Printf(\"%+x\", K(1)) }\n", "the format %+x with a value of type main.K is not supported"},
		{"hex space", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"% x\", \"ab\") }\n", "the format % x with a value of type string is not supported"},
		{"s of int", "package main\nimport \"fmt\"\nfunc main() { x := 1; fmt.Printf(\"%s\", x) }\n", "the format %s with a value of type int is not supported"},
		{"sharp d", "package main\nimport \"fmt\"\nfunc main() { x := 1; fmt.Printf(\"%#d\", x) }\n", "the format %#d with a value of type int is not supported"},
		{"print String plus", "package main\nimport \"fmt\"\ntype K int\nfunc (k K) String() string { return \"k\" }\nfunc main() { fmt.Printf(\"%+v\", K(1)) }\n", "the format %+v with a value of type main.K is not supported"},
		{"embedded", "package main\ntype E struct{}\ntype S struct{ E }\nfunc main() {}\n", "an embedded field is not supported"},
		{"error field", "package main\ntype S struct{ e error }\nfunc main() {}\n", "a struct field of type error is not supported"},
		{"writer field", "package main\nimport \"io\"\ntype S struct{ w io.Writer }\nfunc main() {}\n", "a struct field of type io.Writer is not supported"},
		{"pointer field", "package main\ntype S struct{ p *int }\nfunc main() {}\n", "a struct field of pointer type is not supported"},
		{"map field", "package main\ntype S struct{ m map[string]int }\nfunc main() {}\n", "a map type is not supported"},
		{"generic func", "package main\nfunc g[T any](x T) {}\nfunc main() {}\n", "a generic function (Go 1.18) is not supported"},
		{"no body", "package main\nfunc f()\nfunc main() {}\n", "a function declared without a body is not supported"},
		{"init", "package main\nfunc init() {}\nfunc main() {}\n", "func init is not supported"},
		{"variadic", "package main\nfunc v(xs ...int) {}\nfunc main() {}\n", "a variadic function is not supported"},
		{"range array", "package main\nfunc main() { var a [3]int; for i := range a { _ = i } }\n", "range over an array ([3]int) is not supported"},
		{"range array pointer", "package main\nfunc f(a *[3]int) { for i := range a { _ = i } }\nfunc main() {}\n", "range over a pointer to an array (*[3]int) is not supported"},
		{"assert target", "package main\ntype T struct{ X int }\nfunc main() { var x any = &T{}; x.(*T).X = 1 }\n", "a type assertion in an assignment target is not supported"},
		{"unsafe pointer", "package main\nimport \"unsafe\"\nfunc f(p unsafe.Pointer) {}\nfunc main() {}\n", "type unsafe.Pointer is not supported"},
		// More arguments than parameters (google.golang.org/grpc/interop
		// made the checker index past them, 2026-11-22).
		{"variadic call", "package main\nfunc v(xs ...[]int) {}\nfunc main() { v(nil, nil, nil, nil) }\n", "a variadic function is not supported"},
		{"error not last", "package main\nfunc e() (error, int) { return nil, 0 }\nfunc main() {}\n", "an error result that is not the last result is not supported"},
		{"writer result", "package main\nimport (\"io\"; \"os\")\nfunc w() io.Writer { return os.Stdout }\nfunc main() {}\n", "an io.Writer result is not supported"},
		{"pointer result", "package main\ntype S struct{}\nfunc p() *S { return &S{} }\nfunc main() {}\n", "a pointer result (phase 1 supports pointers only as borrows) is not supported"},
		{"error param", "package main\nfunc e(err error) {}\nfunc main() {}\n", "a parameter of type error"},
		{"pointer pointer param", "package main\nfunc e(p **int) {}\nfunc main() {}\n", "a pointer to a pointer is not supported"},
		{"uint", "package main\nfunc main() { var x uint; _ = x }\n", "type uint is not supported"},
		{"float32", "package main\nfunc f(x float32) {}\nfunc main() {}\n", "type float32 is not supported"},
		{"chan", "package main\nfunc main() { var c chan int; _ = c }\n", "a channel type (goroutines and channels are planned, RFC 0002) is not supported"},
		{"interface", "package main\nfunc f(x interface{ M() }) {}\nfunc main() {}\n", "an interface type is not supported"},
		{"func type", "package main\nfunc f(g func()) {}\nfunc main() {}\n", "a function type is not supported"},
		{"array", "package main\nfunc main() { var a [3]int; _ = a }\n", "an array type is not supported"},
		{"anon struct var", "package main\nfunc main() { var a struct{}; _ = a }\n", "an anonymous struct type is not supported"},
		{"slice of pointers", "package main\nfunc f(x []*int) {}\nfunc main() {}\n", "a slice of pointers is not supported"},
		{"slice of errors", "package main\nfunc f(x []error) {}\nfunc main() {}\n", "type []error is not supported"},
		{"pointer local slice", "package main\nfunc main() { s := []int{}; p := &s; _ = p; var q *[]int = p; _ = q }\n", "a pointer variable that does not start as &x"},
		{"writer local", "package main\nimport (\"io\"; \"os\")\nfunc main() { var w io.Writer = os.Stdout; _ = w }\n", "a local variable of type io.Writer is not supported"},
		{"go", "package main\nfunc f() {}\nfunc main() { go f() }\n", "the go statement (goroutines are planned, RFC 0002) is not supported"},
		{"defer", "package main\nfunc f() {}\nfunc main() { defer f() }\n", "defer is not supported"},
		{"select", "package main\nfunc main() { select {} }\n", "select (planned, RFC 0002) is not supported"},
		{"send", "package main\nfunc f(c chan int) { c <- 1 }\nfunc main() {}\n", "a channel send (planned, RFC 0002) is not supported"},
		{"label", "package main\nfunc main() {\nL:\n\tfor {\n\t\tbreak L\n\t}\n}\n", "a labeled statement is not supported"},
		{"goto", "package main\nfunc main() {\n\tgoto end\nend:\n}\n", "goto is not supported"},
		{"fallthrough", "package main\nfunc main() {\n\tswitch 1 {\n\tcase 1:\n\t\tfallthrough\n\tdefault:\n\t}\n}\n", "fallthrough is not supported"},
		{"type switch", "package main\nfunc f(x any) {}\nfunc main() { var x any; switch x.(type) {} }\n", "a type switch is not supported"},
		{"expr stmt", "package main\nfunc main() { c := make(chan int); <-c }\n", "a channel receive as a statement is not supported"},
		{"range map", "package main\nfunc main() { for range map[int]int{} {} }\n", "range over a map (map[int]int) is not supported"},
		{"range func", "package main\nfunc seq(yield func(int) bool) {}\nfunc main() { for range seq {} }\n", "range over a function (Go 1.23) is not supported"},
		{"range float", "package main\nfunc main() { var c chan int; for range c {} }\n", "range over a channel (chan int) is not supported"},
		{"range assign", "package main\nfunc main() { i := 0; for i = range 3 {}; _ = i }\n", "a range loop that assigns to existing variables (use :=) is not supported"},
		{"shift assign", "package main\nfunc main() { x, n := 1, 2; x <<= n; _ = x }\n", "a shift by a non-constant count is not supported"},
		{"shift", "package main\nfunc main() { x, n := 1, 2; _ = x << n }\n", "a shift by a non-constant count is not supported"},
		{"string byte assign", "package main\nfunc main() { var b [1]byte; s := \"x\"; _ = b; _ = s[0] }\n", "an array type is not supported"},
		{"assign target", "package main\nfunc f() []int { return nil }\nfunc main() { f()[0] = 1 }\n", "this assignment target is not supported"},
		{"invalid utf8 const", "package main\nconst s = \"\\xff\"\nfunc main() {}\n", "a string constant that is not valid UTF-8 is not supported"},
		{"invalid utf8 expr", "package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"\\xff\") }\n", "a string constant that is not valid UTF-8 is not supported"},
		{"complex", "package main\nimport \"fmt\"\nfunc main() { fmt.Println(1i) }\n", "a complex number is not supported"},
		{"func value", "package main\nfunc f() {}\nfunc main() { g := f; _ = g }\n", "using function f as a value is not supported"},
		{"std value", "package main\nimport \"strings\"\nfunc main() { f := strings.Repeat; _ = f }\n", "using strings.Repeat as a value is not supported"},
		{"std var", "package main\nimport \"os\"\nfunc main() { _ = os.Args }\n", "os.Args is not supported"},
		{"std type", "package main\nimport \"io\"\nfunc main() { var r io.Reader; _ = r }\n", "io.Reader is not supported"},
		{"std func", "package main\nimport \"strings\"\nfunc main() { _ = strings.ToUpper(\"x\") }\n", "strings.ToUpper is not supported"},
		{"method value", "package main\ntype T struct{}\nfunc (T) M() {}\nfunc main() { f := T{}.M; _ = f }\n", "a method value is not supported"},
		{"method expr", "package main\ntype T struct{}\nfunc (T) M() {}\nfunc main() { f := T.M; _ = f }\n", "a method expression is not supported"},
		{"error method", "package main\nimport \"errors\"\nfunc f() error { return errors.New(\"x\") }\nfunc main() { if err := f(); err != nil { _ = err.Error } }\n", "a method value is not supported"},
		{"writer method", "package main\nimport (\"io\"; \"os\")\nfunc f(w io.Writer) { w.Write(nil) }\nfunc main() { f(os.Stdout) }\n", "calling methods on an io.Writer"},
		{"anon struct lit", "package main\nfunc main() { _ = struct{ x int }{1} }\n", "an anonymous struct type is not supported"},
		{"keyed slice", "package main\nfunc main() { _ = []int{2: 1} }\n", "an indexed slice literal element is not supported"},
		{"map lit", "package main\nfunc main() { _ = map[int]int{} }\n", "a map type is not supported"},
		{"addr of call", "package main\ntype T struct{ X int }\nfunc f() T { return T{} }\nfunc g(p *T) {}\nfunc main() { g(&[]T{{}}[0]) }\n", "taking the address of this expression is not supported"},
		{"addr of pointer", "package main\nfunc main() { x := 1; p := &x; q := &p; _ = q }\n", "a pointer to a pointer is not supported"},
		{"arrow", "package main\nfunc main() { c := make(chan int); _ = <-c }\n", "the <- operator is not supported"},
		{"slice compare", "package main\nfunc main() { var s []int; _ = s == nil }\n", "comparing a slice with nil is not supported"},
		{"error compare", "package main\nimport \"errors\"\nfunc f() error { return errors.New(\"x\") }\nfunc main() { if err := f(); err != nil { _ = err == err } }\n", "comparing two error values"},
		{"call value", "package main\ntype T struct{ f func() }\nfunc main() { T{}.f() }\n", "a function type is not supported"},
		{"ellipsis", "package main\nfunc f(xs ...int) {}\nfunc main() { f([]int{}...) }\n", "a call with ... arguments is not supported"},
		{"instantiate", "package main\nimport \"slices\"\nfunc main() { _ = slices.Clone[[]int] }\n", "an instantiated generic function (Go 1.18) is not supported"},
		{"conversion string", "package main\nfunc main() { b := []byte(\"x\"); _ = string(b) }\n", "converting a []byte to a string (patina strings must be valid UTF-8) is not supported"},
		{"conversion rune", "package main\nfunc main() { n := 65; _ = string(rune(n)) }\n", "converting an integer to a string is not supported"},
		{"conversion other", "package main\nfunc main() { _ = []rune(\"x\") }\n", "converting string to []rune is not supported"},
		{"make map", "package main\nfunc main() { _ = make(map[int]int) }\n", "make of map[int]int is not supported"},
		{"panic value", "package main\nfunc main() { panic(1) }\n", "panic with a value that is not a string is not supported"},
		{"min one arg", "package main\nfunc main() { x := 1; _ = min(x) }\n", "the min built-in with fewer than two arguments is not supported"},
		{"max float", "package main\nfunc main() { x := 1.0; _ = max(x, 2.0) }\n", "the max built-in with a non-integer argument is not supported"},
		{"clear", "package main\nfunc main() { s := []int{1}; clear(s) }\n", "the clear built-in (Go 1.21) is not supported"},
		{"cap", "package main\nfunc main() { _ = cap([]int{}) }\n", "the cap built-in is not supported"},
		{"format not const", "package main\nimport \"fmt\"\nfunc main() { f := \"%d\"; fmt.Printf(f, 1) }\n", "a format string that is not a constant is not supported"},
		{"format bad", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%[1]d\", 1) }\n", "'[' in a directive is not supported"},
		{"format missing", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%d %d\", 1) }\n", "format string: %d has no operand"},
		{"format extra", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%d\", 1, 2) }\n", "format string has 1 verbs but 2 operands are given"},
		{"verb", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%d\", \"x\") }\n", "the format %d with a value of type string is not supported"},
		{"print struct", "package main\nimport \"fmt\"\ntype T struct{}\nfunc main() { fmt.Println(T{}) }\n", "printing a value of type main.T is not supported"},
		{"bytes sprintf", "package main\nimport \"fmt\"\nfunc main() { _ = fmt.Sprintf(\"%s\", []byte(\"x\")) }\n", "the format %s with a value of type []byte is not supported"},
		{"pointer assign", "package main\nfunc main() { x, y := 1, 2; p := &x; p = &y; _ = p }\n", "assigning to a pointer variable"},
		{"nil pointer arg", "package main\nfunc f(p *int) {}\nfunc main() { f(nil) }\n", "passing nil for a pointer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := check(t, tt.src)
			if !strings.Contains(got, tt.want) {
				t.Errorf("diagnostics do not contain %q:\n%s", tt.want, got)
			}
			if got != "" && !strings.Contains(got, "./main.go:") {
				t.Errorf("diagnostics do not point at the source:\n%s", got)
			}
		})
	}
}

// TestUnsupportedTypeOther covers a type the subset never meets as a value
// type (a tuple), which once reached it as a type alias.
func TestUnsupportedTypeOther(t *testing.T) {
	if got := unsupportedType(types.NewTuple(), false); got != "type ()" {
		t.Errorf("unsupportedType(tuple) = %q", got)
	}
}
