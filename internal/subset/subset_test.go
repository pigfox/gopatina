package subset

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
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
	var x any = v
	var n, ok = x.(int)
	switch x.(type) {
	case int:
		fmt.Println(n, ok)
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
		{"library init", "package lib\nfunc init() {}\n", "an init function in a library"},
		{"library changed var", "package lib\nvar n int\nfunc Bump() { n++ }\n", "a package-level variable that a library changes"},
		{"library exported var", "package lib\nvar Limit = 3\nfunc F() int { return Limit }\n", "an exported package-level variable in a library, other than a sentinel error"},
		{"library var by call", "package lib\nfunc f() int { return 1 }\nvar n = f()\nfunc F() int { return n }\n", "a package-level variable of a library initialized by a call of the program"},
		{"library iface api", "package lib\ntype S interface{ M() }\nfunc F(s S) {}\n", "an interface type in the API of a library"},
		{"library heap ptr api", "package lib\ntype T struct{ n int }\nfunc New() *T { return &T{} }\n", "a pointer that escapes in the API of a library"},
		{"library heap recv", "package lib\ntype T struct{ n int }\nfunc k() *T { return &T{} }\nfunc (t *T) M() int { return t.n }\n", "a pointer that escapes as the receiver of an exported method of a library"},
		{"library api field", "package lib\ntype T struct{ A any }\n", "an interface type in an exported field of a library's exported struct"},
		{"library error param", "package lib\nfunc Describe(err error) string { return \"x\" }\n", "an error parameter in the API of a library"},
		{"library error field", "package lib\ntype T struct{ Err error }\n", "an error value in an exported field of a library's exported struct"},
		{"nomain", "package main\nfunc f() {}\n", "package main has no func main()"},
		{"unsafe", "package main\nimport \"unsafe\"\nvar _ = unsafe.Sizeof(0)\nfunc main() {}\n", "package unsafe is not supported"},
		{"otherpkg", "package main\nimport \"sort\"\nfunc main() { sort.Ints(nil) }\n", "package sort is not supported"},
		{"renamed", "package main\nimport f \"fmt\"\nfunc main() { f.Println() }\n", "a renamed or dot import is not supported"},
		{"blank import of an unsupported package", "package main\nimport _ \"sort\"\nfunc main() {}\n", "package sort is not supported"},
		{"changed slice global", "package main\nvar s []int\nfunc main() { s = append(s, 1) }\n", "a package-level variable of type []int that the program changes is not supported"},
		{"global address", "package main\nvar n int\nfunc main() { p := &n; *p = 1 }\n", "taking the address of a package-level variable is not supported"},
		{"global ptr method", "package main\ntype T struct{ X int }\nfunc (t *T) Inc() { t.X++ }\nvar g T\nfunc main() { g.Inc() }\n", "calling a pointer-receiver method on a package-level variable is not supported"},
		{"static slice copy", "package main\nvar s = []int{1}\nfunc main() { t := s; _ = t }\n", "using the package-level variable s this way (a copy of a slice or a map shares its elements) is not supported"},
		{"loop global call", "package main\nvar n int\nfunc f() int { n++; return n }\nfunc main() { for i := 0; i < n+f(); i++ { } }\n", "a condition that reads a changed package-level variable and calls a function of the program is not supported"},
		{"changed error global", "package main\nimport \"errors\"\nvar e = errors.New(\"x\")\nfunc main() { e = errors.New(\"y\"); _ = e }\n", "a package-level variable of type error that the program changes is not supported"},
		{"writer global", "package main\nimport (\"io\"; \"os\")\nvar w io.Writer = os.Stdout\nfunc main() { _ = w }\n", "a package-level variable of type io.Writer is not supported"},
		{"multi-value global", "package main\nfunc f() (int, int) { return 1, 2 }\nvar a, b = f()\nfunc main() { _, _ = a, b }\n", "a package-level variable spec with a multi-value initializer is not supported"},
		{"generic type", "package main\ntype B[T any] struct{ v T }\nfunc main() {}\n", "a generic type (Go 1.18) is not supported"},
		{"generic method", "package main\ntype B[T any] struct{ v T }\nfunc (b B[T]) M() {}\nfunc main() {}\n", "a method on a generic type (Go 1.18)"},
		{"generic instance", "package main\ntype B[T any] struct{ v T }\nfunc f(b B[int]) {}\nfunc main() {}\n", "a generic type (Go 1.18)"},
		{"closure variadic", "package main\nimport \"fmt\"\nvar g int\nfunc h() int { return 1 }\nfunc main() { f := func(xs ...int) {}; f(); fmt.Println() }\n", "a function literal with a ... parameter is not supported"},
		{"closure string parameter", "package main\nimport \"fmt\"\nvar g int\nfunc h() int { return 1 }\nfunc main() { f := func(s string) {}; f(\"x\"); fmt.Println() }\n", "a function literal with a parameter of type string (step 1 of RFC 0018 takes numbers and bools) is not supported"},
		{"closure two results", "package main\nimport \"fmt\"\nvar g int\nfunc h() int { return 1 }\nfunc main() { f := func() (int, int) { return 1, 2 }; a, b := f(); fmt.Println(a, b) }\n", "a function literal with more than one result is not supported"},
		{"closure named result", "package main\nimport \"fmt\"\nvar g int\nfunc h() int { return 1 }\nfunc main() { f := func() (n int) { return }; fmt.Println(f()) }\n", "a function literal with a named result is not supported"},
		{"closure slice result", "package main\nimport \"fmt\"\nvar g int\nfunc h() int { return 1 }\nfunc main() { f := func() []int { return nil }; fmt.Println(len(f())) }\n", "a function literal with a result of type []int (step 1 of RFC 0018 takes numbers, bools and strings) is not supported"},
		{"closure string capture", "package main\nimport \"fmt\"\nvar g int\nfunc h() int { return 1 }\nfunc main() { s := \"x\"; f := func() int { return len(s) }; fmt.Println(f()) }\n", "a function literal that uses s, of type string, from its enclosing function (step 1 of RFC 0018 takes numbers and bools) is not supported"},
		{"closure in closure", "package main\nimport \"fmt\"\nvar g int\nfunc h() int { return 1 }\nfunc main() { f := func() { g := func() {}; g() }; f(); fmt.Println() }\n", "a function literal inside a function literal is not supported"},
		{"closure defer", "package main\nimport \"fmt\"\nvar g int\nfunc h() int { return 1 }\nfunc main() { f := func() { defer fmt.Println() }; f() }\n", "defer inside a function literal is not supported"},
		{"closure address", "package main\nimport \"fmt\"\nvar g int\nfunc h() int { return 1 }\nfunc main() { x := 1; f := func() { p := &x; fmt.Println(*p) }; f() }\n", "taking the address of a variable a function literal uses from its enclosing function is not supported"},
		{"closure global", "package main\nimport \"fmt\"\nvar g int\nfunc h() int { return 1 }\nfunc main() { f := func() int { return g }; fmt.Println(f()) }\n", "a package-level variable inside a function literal is not supported"},
		{"closure program call", "package main\nimport \"fmt\"\nvar g int\nfunc h() int { return 1 }\nfunc main() { f := func() int { return h() }; fmt.Println(f()) }\n", "a call of a function of the program inside a function literal is not supported"},
		{"closure changes, then used", "package main\nimport \"fmt\"\nvar g int\nfunc h() int { return 1 }\nfunc main() { x := 0; f := func() { x++ }; f(); fmt.Println(x); f() }\n", "x is used while the closure f, which changes it, is still to be called (it borrows x until its last call) is not supported"},
		{"closure reads, then changed", "package main\nimport \"fmt\"\nvar g int\nfunc h() int { return 1 }\nfunc main() { x := 0; f := func() int { return x }; fmt.Println(f()); x = 2; fmt.Println(f()) }\n", "x is changed while the closure f, which reads it, is still to be called (it borrows x until its last call) is not supported"},
		{"closure called in a loop", "package main\nimport \"fmt\"\nvar g int\nfunc h() int { return 1 }\nfunc main() { x := 0; f := func() { x++ }; for i := 0; i < 3; i++ { f(); fmt.Println(x) } }\n", "x is used while the closure f, which changes it, is still to be called"},
		{"closure reads, then its address taken", "package main\nimport \"fmt\"\nfunc main() { x := 1; f := func() int { return x }; fmt.Println(f()); p := &x; fmt.Println(*p, f()) }\n", "x is changed while the closure f, which reads it"},
		{"closure not bound", "package main\nimport \"fmt\"\nvar g int\nfunc h() int { return 1 }\nfunc main() { var f = func() {}; f(); fmt.Println() }\n", "a function literal (closure) other than f := func(...) {...}, an argument or a returned value (RFC 0018) is not supported"},
		{"func param variadic", "package main\nimport (\"fmt\"; \"math\")\nfunc apply(f func(int) int, x int) int { return f(x) }\nfunc h() int { return 1 }\nfunc g(f func(...int)) {}\nfunc main() { fmt.Println(math.Pi) }\n", "a parameter of function type with a ... parameter is not supported"},
		{"func param results", "package main\nimport (\"fmt\"; \"math\")\nfunc apply(f func(int) int, x int) int { return f(x) }\nfunc h() int { return 1 }\nfunc g(f func() []int) {}\nfunc main() { fmt.Println(math.Pi) }\n", "a parameter of function type with results ([]int) (step 2 of RFC 0018 takes one number, bool or string) is not supported"},
		{"func param stored", "package main\nimport (\"fmt\"; \"math\")\nfunc apply(f func(int) int, x int) int { return f(x) }\nfunc h() int { return 1 }\nfunc g(f func()) { k := f; k() }\nfunc main() { fmt.Println(math.Pi) }\n", "using the function parameter f other than by calling it (step 2 of RFC 0018) is not supported"},
		{"func param f(f(x)) FnMut", "package main\nimport (\"fmt\"; \"math\")\nfunc apply(f func(int) int, x int) int { return f(x) }\nfunc h() int { return 1 }\nfunc t(f func(int) int) int { return f(f(1)) }\nfunc main() { x := 0; add := func(i int) int { x += i; return x }; fmt.Println(t(add), math.Pi) }\n", "calling f inside the arguments of a call of f, when it is passed a closure that changes what it captures is not supported"},
		{"func arg nil", "package main\nimport (\"fmt\"; \"math\")\nfunc apply(f func(int) int, x int) int { return f(x) }\nfunc h() int { return 1 }\nfunc main() { fmt.Println(apply(nil, 1), math.Pi) }\n", "a function argument other than a function of the program, a closure, a function literal or a math function (RFC 0018) is not supported"},
		{"func arg std", "package main\nimport (\"fmt\"; \"math\")\nfunc apply(f func(int) int, x int) int { return f(x) }\nfunc h() int { return 1 }\nfunc g(f func(float64, float64) float64) {}\nfunc main() { g(math.Max); fmt.Println() }\n", "a function argument other than a function of the program, a closure, a function literal or a math function"},
		{"func arg literal nested", "package main\nimport (\"fmt\"; \"math\")\nfunc apply(f func(int) int, x int) int { return f(x) }\nfunc h() int { return 1 }\nfunc main() { fmt.Println(apply(func(x int) int { return x }, 1), math.Pi) }\n", "a function literal passed in a call that is not a statement of its own, an assignment or a return (step 2 of RFC 0018) is not supported"},
		{"func arg literal changes", "package main\nimport (\"fmt\"; \"math\")\nfunc apply(f func(int) int, x int) int { return f(x) }\nfunc h() int { return 1 }\nfunc main() { x := 0; r := apply(func(i int) int { x += i; return x }, x); fmt.Println(r, math.Pi) }\n", "x is used in the statement that passes a function literal which changes it is not supported"},
		{"func arg literal read", "package main\nimport (\"fmt\"; \"math\")\nfunc apply(f func(int) int, x int) int { return f(x) }\nfunc h() int { return 1 }\nfunc main() { x := 1; x = apply(func(i int) int { return i + x }, 2); fmt.Println(x, math.Pi) }\n", "x is changed in the statement that passes a function literal which reads it is not supported"},
		{"func arg literal body", "package main\nimport (\"fmt\"; \"math\")\nfunc apply(f func(int) int, x int) int { return f(x) }\nfunc h() int { return 1 }\nfunc main() { r := apply(func(x int) int { return h() }, 1); fmt.Println(r, math.Pi) }\n", "a call of a function of the program inside a function literal is not supported"},
		{"func param passed on", "package main\nimport (\"fmt\"; \"math\")\nfunc apply(f func(int) int, x int) int { return f(x) }\nfunc g(f func(int) int) int { return apply(f, 1) }\nfunc main() { fmt.Println(math.Pi) }\n", "using the function parameter f other than by calling it (step 2 of RFC 0018) is not supported"},
		{"func arg literal address", "package main\nimport (\"fmt\"; \"math\")\nfunc apply(f func(int) int, x int) int { return f(x) }\nfunc main() { x := 1; r := apply(func(i int) int { return i + x }, *(&x)); fmt.Println(r, math.Pi) }\n", "x is changed in the statement that passes a function literal which reads it"},
		{"func result string", "package main\nimport (\"fmt\"; \"math\")\nfunc h() int { return 1 }\nfunc g() func(string) { return func(s string) {} }\nfunc main() { fmt.Println(math.Pi) }\n", "a result of function type: a parameter of function type taking string (step 2 of RFC 0018 takes numbers and bools) is not supported"},
		{"func result nil", "package main\nimport (\"fmt\"; \"math\")\nfunc h() int { return 1 }\nfunc g() func() { return nil }\nfunc main() { fmt.Println(math.Pi) }\n", "returning a function other than a function literal (step 3 of RFC 0018) is not supported"},
		{"func result body", "package main\nimport (\"fmt\"; \"math\")\nfunc h() int { return 1 }\nfunc g() func() int { return func() int { return h() } }\nfunc main() { fmt.Println(math.Pi) }\n", "a call of a function of the program inside a function literal is not supported"},
		{"func value copied", "package main\nimport (\"fmt\"; \"math\")\nfunc h() int { return 1 }\nfunc g() func() { return func() {} }\nfunc main() { f := g(); k := f; k(); fmt.Println(math.Pi) }\n", "using the function value f other than by calling it or passing it to a parameter of function type (step 3 of RFC 0018) is not supported"},
		{"recursive capture", "package main\nimport (\"fmt\"; \"math\")\nfunc h() int { return 1 }\nfunc main() { x := 1; var f func(int) int; f = func(n int) int { return n + x }; fmt.Println(f(1), math.Pi) }\n", "a recursive function literal that uses x from its enclosing function (it is a nested fn, which cannot capture; step 4 of RFC 0018) is not supported"},
		{"recursive called first", "package main\nimport (\"fmt\"; \"math\")\nfunc h() int { return 1 }\nfunc main() { var f func(); f(); f = func() {}; fmt.Println(math.Pi) }\n", "calling f before it is assigned (a nil function in Go) is not supported"},
		{"recursive reassigned", "package main\nimport (\"fmt\"; \"math\")\nfunc h() int { return 1 }\nfunc main() { var f func(); f = func() {}; f = func() {}; f(); fmt.Println(math.Pi) }\n", "using the recursive function f other than by calling it or passing it to a parameter of function type (step 4 of RFC 0018) is not supported"},
		{"recursive body", "package main\nimport (\"fmt\"; \"math\")\nfunc h() int { return 1 }\nfunc main() { var f func() int; f = func() int { return h() }; fmt.Println(f(), math.Pi) }\n", "a call of a function of the program inside a function literal is not supported"},
		{"defer closure args", "package main\nimport (\"fmt\"; \"math\"; \"strings\")\nfunc h() int { return 1 }\nfunc main() { defer func(n int) { fmt.Println(n) }(1); fmt.Println(math.Pi, strings.ToUpper(\"\")) }\n", "deferring a function literal with arguments (step 5 of RFC 0018 takes func() {...}()) is not supported"},
		{"defer closure params", "package main\nimport (\"fmt\"; \"math\"; \"strings\")\nfunc h() int { return 1 }\nfunc main() { defer func(n ...int) {}(); fmt.Println(math.Pi, strings.ToUpper(\"\")) }\n", "deferring a function literal with parameters or results (step 5 of RFC 0018 takes func() {...}()) is not supported"},
		{"defer closure in loop", "package main\nimport (\"fmt\"; \"math\"; \"strings\")\nfunc h() int { return 1 }\nfunc main() { for i := 0; i < 2; i++ { defer func() { fmt.Println(i) }() }; fmt.Println(math.Pi, strings.ToUpper(\"\")) }\n", "deferring a function literal inside a loop (step 5 of RFC 0018) is not supported"},
		{"defer closure body", "package main\nimport (\"fmt\"; \"math\"; \"strings\")\nfunc h() int { return 1 }\nfunc main() { defer func() { fmt.Println(h()) }(); fmt.Println(math.Pi, strings.ToUpper(\"\")) }\n", "a call of a function of the program inside a function literal is not supported"},
		{"defer closure writes", "package main\nimport (\"fmt\"; \"math\"; \"strings\")\nfunc h() int { return 1 }\nfunc main() { x := 1; defer func() { x++ }(); fmt.Println(x, math.Pi, strings.ToUpper(\"\")) }\n", "a deferred function literal that changes x, of its enclosing function (step 5 of RFC 0018) is not supported"},
		{"defer closure changed after", "package main\nimport (\"fmt\"; \"math\"; \"strings\")\nfunc h() int { return 1 }\nfunc main() { x := 1; defer func() { fmt.Println(x) }(); x = 2; fmt.Println(x, math.Pi, strings.ToUpper(\"\")) }\n", "changing x after deferring a function literal that uses it (step 5 of RFC 0018) is not supported"},
		{"defer closure addr after", "package main\nimport (\"fmt\"; \"math\"; \"strings\")\nfunc h() int { return 1 }\nfunc main() { x := 1; defer func() { fmt.Println(x) }(); p := &x; fmt.Println(*p, math.Pi, strings.ToUpper(\"\")) }\n", "changing x after deferring a function literal that uses it (step 5 of RFC 0018) is not supported"},
		{"defer closure index", "package main\nimport (\"fmt\"; \"math\"; \"strings\")\nfunc h() int { return 1 }\nfunc main() { defer func() { s := \"ab\"; n := 1; fmt.Println(s[n]) }(); fmt.Println(math.Pi, strings.ToUpper(\"\")) }\n", "an index or slice expression in a deferred function literal (it may panic while a panic unwinds) is not supported"},
		{"defer closure divide", "package main\nimport (\"fmt\"; \"math\"; \"strings\")\nfunc h() int { return 1 }\nfunc main() { d := 0; defer func() { fmt.Println(10 / d) }(); fmt.Println(math.Pi, strings.ToUpper(\"\")) }\n", "a division, remainder or shift by a variable in a deferred function literal (it may panic while a panic unwinds) is not supported"},
		{"defer closure shift", "package main\nimport (\"fmt\"; \"math\"; \"strings\")\nfunc h() int { return 1 }\nfunc main() { d := 2; defer func() { fmt.Println(1 << d) }(); fmt.Println(math.Pi, strings.ToUpper(\"\")) }\n", "a division, remainder or shift by a variable in a deferred function literal (it may panic while a panic unwinds) is not supported"},
		{"defer closure rem assign", "package main\nimport (\"fmt\"; \"math\"; \"strings\")\nfunc h() int { return 1 }\nfunc main() { d := 2; defer func() { n := 7; n %= d; fmt.Println(n) }(); fmt.Println(math.Pi, strings.ToUpper(\"\")) }\n", "a division, remainder or shift by a variable in a deferred function literal (it may panic while a panic unwinds) is not supported"},
		{"defer closure stdlib call", "package main\nimport (\"fmt\"; \"math\"; \"strings\")\nfunc h() int { return 1 }\nfunc main() { defer func() { fmt.Println(strings.Repeat(\"a\", 2)) }(); fmt.Println(math.Pi) }\n", "a call other than fmt.Print, Println, Printf or a math function in a deferred function literal (it may panic while a panic unwinds) is not supported"},
		{"defer closure panic", "package main\nimport (\"fmt\"; \"math\"; \"strings\")\nfunc h() int { return 1 }\nfunc main() { defer func() { panic(\"again\") }(); fmt.Println(math.Pi, strings.ToUpper(\"\")) }\n", "a call other than fmt.Print, Println, Printf or a math function in a deferred function literal (it may panic while a panic unwinds) is not supported"},
		{"defer closure fmt other", "package main\nimport (\"fmt\"; \"math\"; \"strings\")\nfunc h() int { return 1 }\nfunc main() { defer func() { fmt.Println(fmt.Errorf(\"x\")) }(); fmt.Println(math.Pi, strings.ToUpper(\"\")) }\n", "a call other than fmt.Print, Println, Printf or a math function in a deferred function literal (it may panic while a panic unwinds) is not supported"},
		{"defer closure string conversion", "package main\nimport (\"fmt\"; \"math\"; \"strings\")\nfunc h() int { return 1 }\nfunc main() { defer func() { fmt.Println(string(rune(65))) }(); fmt.Println(math.Pi, strings.ToUpper(\"\")) }\n", "a call other than fmt.Print, Println, Printf or a math function in a deferred function literal (it may panic while a panic unwinds) is not supported"},
		{"defer closure method of call", "package main\nimport (\"fmt\"; \"time\")\nfunc main() { defer func() { fmt.Println(time.Now().Unix()) }(); fmt.Println(1) }\n", "a call other than fmt.Print, Println, Printf or a math function in a deferred function literal (it may panic while a panic unwinds) is not supported"},
		{"defer closure method of local", "package main\nimport (\"fmt\"; \"time\")\nfunc main() { defer func() { d := time.Duration(5); fmt.Println(d.Hours()) }(); fmt.Println(1) }\n", "a call other than fmt.Print, Println, Printf or a math function in a deferred function literal (it may panic while a panic unwinds) is not supported"},
		{"map float key", "package main\nimport \"fmt\"\nfunc main() { m := map[float64]int{}; fmt.Println(len(m)) }\n", "a map with keys of type float64 (step 1 of RFC 0019 takes strings, integers and bools) is not supported"},
		{"map slice value", "package main\nimport \"fmt\"\nfunc main() { m := map[string][]int{}; fmt.Println(len(m)) }\n", "a map with values of type []int (RFC 0019 takes numbers, bools, strings and structs of numbers and bools) is not supported"},
		{"map make float key", "package main\nimport \"fmt\"\nfunc main() { m := make(map[float64]int); fmt.Println(len(m)) }\n", "a map with keys of type float64 (step 1 of RFC 0019 takes strings, integers and bools) is not supported"},
		{"map nil", "package main\nimport \"fmt\"\nfunc main() { var m map[string]int; fmt.Println(len(m)) }\n", "a map variable m not made by make or a map literal where it is declared (a nil map; step 1 of RFC 0019) is not supported"},
		{"map copied", "package main\nimport \"fmt\"\nfunc main() { m := map[string]int{}; n := m; fmt.Println(len(n)) }\n", "using the map m other than by indexing it, passing it to delete or len, or printing it (it would be copied; step 1 of RFC 0019) is not supported"},
		{"map param", "package main\nimport \"fmt\"\nfunc f(m map[string]int) int { return len(m) }\nfunc main() { fmt.Println(f(nil)) }\n", "a map type other than a variable's (RFC 0019) is not supported"},
		{"map size variable", "package main\nimport \"fmt\"\nfunc main() { n := 2; m := make(map[string]int, n); fmt.Println(len(m)) }\n", "make of a map with a size that is not a constant (Go panics on a negative one) is not supported"},
		{"map string append", "package main\nimport \"fmt\"\nfunc main() { m := map[string]string{}; m[\"a\"] += \"b\"; fmt.Println(m) }\n", "+= on an element of a map other than +=, -=, *=, ++ or -- of a number (step 1 of RFC 0019) is not supported"},
		{"map divide", "package main\nimport \"fmt\"\nfunc main() { m := map[string]int{}; m[\"a\"] /= 2; fmt.Println(m) }\n", "/= on an element of a map other than +=, -=, *=, ++ or -- of a number (step 1 of RFC 0019) is not supported"},
		{"map in closure", "package main\nimport \"fmt\"\nfunc main() { f := func() int { m := map[string]int{}; return len(m) }; fmt.Println(f()) }\n", "a map type other than a variable's (RFC 0019) is not supported"},
		{"map from call", "package main\nimport (\"fmt\"; \"maps\")\nfunc main() { m := map[string]int{}; n := maps.Clone(m); fmt.Println(len(n)) }\n", "a map variable n not made by make or a map literal where it is declared (a nil map; step 1 of RFC 0019) is not supported"},
		{"map other calls", "package main\nimport \"fmt\"\ntype T struct{}\nfunc (T) N(int) {}\nfunc main() { m := map[string]int{}; var t T; (t).N(1); t.N(2); defer func() {}(); m[\"a\"] /= 2; fmt.Println(m) }\n", "/= on an element of a map other than +=, -=, *=, ++ or -- of a number (step 1 of RFC 0019) is not supported"},
		{"array lit", "package main\nfunc main() { _ = [2]int{1, 2} }\n", "an array type is not supported"},
		{"global map nil", "package main\nimport \"fmt\"\nvar m map[string]int\nfunc main() { fmt.Println(len(m)) }\n", "a package-level map variable m not made by a map literal where it is declared (a nil map; step 2 of RFC 0019) is not supported"},
		{"global map make", "package main\nimport \"fmt\"\nvar m = make(map[string]int)\nfunc main() { fmt.Println(len(m)) }\n", "a package-level map variable m not made by a map literal where it is declared (step 2 of RFC 0019) is not supported"},
		{"global map set", "package main\nimport \"fmt\"\nvar m = map[string]int{}\nfunc main() { m[\"a\"] = 1; fmt.Println(m) }\n", "a package-level variable of type map[string]int that the program changes is not supported"},
		{"global map delete", "package main\nimport \"fmt\"\nvar m = map[string]int{\"a\": 1}\nfunc main() { delete(m, \"a\"); fmt.Println(m) }\n", "a package-level variable of type map[string]int that the program changes is not supported"},
		{"global map float key", "package main\nimport \"fmt\"\nvar m = map[float64]int{}\nfunc main() { fmt.Println(len(m)) }\n", "a map with keys of type float64 (step 1 of RFC 0019 takes strings, integers and bools) is not supported"},
		{"global map copied", "package main\nimport \"fmt\"\nvar m = map[string]int{}\nfunc main() { n := m; fmt.Println(len(n)) }\n", "using the package-level variable m this way (a copy of a slice or a map shares its elements) is not supported"},
		{"map struct of strings", "package main\nimport \"fmt\"\ntype T struct{ s string }\nfunc main() { m := map[string]T{}; fmt.Println(len(m)) }\n", "a map with values of type main.T (RFC 0019 takes numbers, bools, strings and structs of numbers and bools) is not supported"},
		{"map anonymous struct", "package main\nimport \"fmt\"\nfunc main() { m := map[string]struct{ x int }{}; fmt.Println(len(m)) }\n", "a map with values of type struct{x int} (RFC 0019 takes numbers, bools, strings and structs of numbers and bools) is not supported"},
		{"map range set", "package main\nimport \"fmt\"\nfunc main() { m := map[string]int{\"a\": 1}; for k := range m { m[k] = 2 }; fmt.Println(m) }\n", "changing the map m while ranging over it (decision 5 of RFC 0019) is not supported"},
		{"map range delete", "package main\nimport \"fmt\"\nfunc main() { m := map[string]int{\"a\": 1}; for k := range m { delete(m, k) }; fmt.Println(m) }\n", "changing the map m while ranging over it (decision 5 of RFC 0019) is not supported"},
		{"map range inc", "package main\nimport \"fmt\"\nfunc main() { m := map[string]int{\"a\": 1}; for k, v := range m { x := v; x++; m[k]++; fmt.Println(x) }; fmt.Println(m) }\n", "changing the map m while ranging over it (decision 5 of RFC 0019) is not supported"},
		{"func arg method value", "package main\nimport \"fmt\"\ntype T struct{}\nfunc (T) M(x float64) float64 { return x }\nfunc g(f func(float64) float64) float64 { return f(1) }\nfunc main() { var t T; fmt.Println(g(t.M)) }\n", "a function argument other than a function of the program, a closure, a function literal or a math function"},
		{"recover outside defer", "package main\nimport \"fmt\"\nfunc main() { r := recover(); fmt.Println(r != nil) }\n", "recover other than directly in a deferred function literal (it returns nil there; RFC 0020) is not supported"},
		{"recover value copied", "package main\nimport \"fmt\"\nfunc main() { defer func() { r := recover(); s := r; fmt.Println(s != nil) }(); fmt.Println(1) }\n", "using recover's value other than by comparing it with nil or printing it (RFC 0020) is not supported"},
		{"recover value returned", "package main\nimport \"fmt\"\nfunc main() { defer func() { _ = recover() }(); fmt.Println(1) }\n", "using recover's value other than by comparing it with nil or printing it (RFC 0020) is not supported"},
		{"recover error result", "package main\nimport \"fmt\"\nfunc f() error { defer func() { if r := recover(); r != nil { fmt.Println(r) } }(); return nil }\nfunc main() { fmt.Println(f()) }\n", "a function that recovers with a result of type error (RFC 0020 takes numbers, bools and structs of them, and unnamed strings) is not supported"},
		{"recover named string", "package main\nimport \"fmt\"\nfunc f() (s string) { defer func() { if r := recover(); r != nil { fmt.Println(r) } }(); s = \"x\"; return s }\nfunc main() { fmt.Println(f()) }\n", "a function that recovers with a result of type string (RFC 0020 takes numbers, bools and structs of them, and unnamed strings) is not supported"},
		{"recover named panic", "package main\nimport \"fmt\"\ntype T string\nfunc f() { defer func() { if r := recover(); r != nil { fmt.Println(r) } }(); panic(T(\"x\")) }\nfunc main() { f() }\n", "panic with a value of type main.T in a program that recovers (recover's value would print differently; RFC 0020) is not supported"},
		{"map copied errorf", "package main\nimport \"fmt\"\nfunc main() { m := map[string]int{}; n := m; fmt.Println(fmt.Errorf(\"x\"), len(n)) }\n", "using the map m other than by indexing it"},
		{"closure", "package main\nfunc main() { f := func() {}; _ = f }\n", "using the closure f other than by calling it or passing it to a parameter of function type (RFC 0018, steps 1 and 2) is not supported"},
		{"slice expr", "package main\nfunc main() { s := []int{1}; _ = s[0:1] }\n", "a slice expression (s[i:j]) is not supported"},
		{"print int pointer", "package main\nimport \"fmt\"\nfunc k() *int { x := 1; return &x }\nfunc main() { fmt.Println(k()) }\n", "printing a pointer of type *int, which has no String or Error method"},
		{"as bad target", "package main\nimport (\"errors\"; \"fmt\")\ntype E struct{ n int }\nfunc (e E) Error() string { return fmt.Sprint(10 / e.n) }\nfunc main() { var t E; err := errors.New(\"x\"); fmt.Println(errors.As(err, &t)) }\n", "converting to an interface a value whose Error method may panic"},
		{"print pointer", "package main\nimport \"fmt\"\ntype T struct{ n int }\nfunc k() *T { return &T{} }\nfunc main() { fmt.Println(k()) }\n", "printing a pointer of type *main.T, which has no String or Error method (fmt prints &{...} for a pointer to a struct, and an address for any other or inside another value) is not supported"},
		{"print iface of pointer", "package main\nimport \"fmt\"\ntype T struct{ n int }\nfunc main() { var a any = &T{}; fmt.Println(a) }\n", "printing an interface value in a program that converts *main.T to one"},
		{"pointer print may panic", "package main\nimport \"fmt\"\ntype T struct{ n []int }\nfunc (t *T) String() string { return fmt.Sprint(t.n[0]) }\nfunc k() *T { return &T{} }\nfunc main() { fmt.Println(k()) }\n", "printing a value whose String method may panic"},
		{"assert pointer", "package main\nfunc main() { var x any; _ = x.(*int) }\n", "converting a pointer to a type other than a named type of the program to an interface"},
		{"promoted", "package main\ntype E struct{ X int }\ntype S struct{ E }\nfunc main() { _ = S{}.X }\n", "a promoted field is not supported"},
		{"call literal", "package main\nfunc main() { func() {}() }\n", "calling a function value is not supported"},
		{"call func var", "package main\nfunc main() { var f func(); f() }\n", "calling a function value is not supported"},
		{"call generic", "package main\nfunc g[T any](x T) {}\nfunc main() { g[int](1) }\n", "an instantiated generic function (Go 1.18) is not supported"},
		{"call generic2", "package main\nfunc g[T, U any](x T, y U) {}\nfunc main() { g[int, int](1, 2) }\n", "an instantiated generic function (Go 1.18) is not supported"},
		{"alias pointer", "package main\ntype A = *int\nfunc main() {}\n", "a type alias of a pointer type is not supported"},
		{"alias error", "package main\ntype E = error\nfunc main() {}\n", "a type alias of error or io.Writer is not supported"},
		{"alias map", "package main\ntype M = map[int]int\nfunc main() {}\n", "type M: an alias of a map type other than a variable's (RFC 0019) is not supported"},
		{"local alias", "package main\nfunc main() { type L = int; var x L; _ = x }\n", "a type declared inside a function is not supported"},
		{"local type", "package main\nfunc main() { type L struct{}; _ = L{} }\n", "a type declared inside a function is not supported"},
		{"named map", "package main\ntype C map[string]int\nfunc main() {}\n", "type C: a map type other than a variable's (RFC 0019) is not supported"},
		{"named foreign struct", "package main\nimport \"strings\"\ntype B strings.Builder\nfunc main() {}\n", "type B: a struct type from another package is not supported"},
		{"panic panicky String", "package main\nimport \"fmt\"\ntype K int\nfunc (k K) String() string { return fmt.Sprint(10 / int(k)) }\nfunc main() { panic(K(1)) }\n", "printing a value whose String method may panic is not supported"},
		{"panic error whose Error may panic", "package main\nimport \"fmt\"\ntype T struct{ n []int }\nfunc (t *T) Error() string { return fmt.Sprint(t.n[0]) }\nfunc main() { var err error = &T{}; panic(err) }\n", "panic with an error value: printing an interface value"},
		{"print panicky String", "package main\nimport \"fmt\"\ntype K int\nfunc (k K) String() string { return fmt.Sprint(10 / int(k)) }\nfunc main() { fmt.Println(K(1)) }\n", "printing a value whose String method may panic is not supported"},
		{"print panicky Error", "package main\nimport \"fmt\"\ntype K []int\nfunc (k K) Error() string { return fmt.Sprint(k[0]) }\nfunc main() { fmt.Printf(\"%v\\n\", K{1}) }\n", "printing a value whose Error method may panic is not supported"},
		{"print String calls", "package main\nimport \"fmt\"\ntype K int\nfunc g() string { return \"\" }\nfunc (k K) String() string { return g() }\nfunc main() { fmt.Print(K(1)) }\n", "printing a value whose String method may panic is not supported"},
		{"print String nested", "package main\nimport \"fmt\"\ntype K int\nfunc (k K) String() string { return fmt.Sprint(k + 1) }\nfunc main() { fmt.Print(K(1)) }\n", "printing a value whose String method may panic is not supported"},
		{"print String loop", "package main\nimport \"fmt\"\ntype K int\nfunc (k K) String() string { for { } }\nfunc main() { fmt.Print(K(1)) }\n", "printing a value whose String method may panic is not supported"},
		{"print panicky String slice", "package main\nimport \"fmt\"\ntype K int\nfunc (k K) String() string { return fmt.Sprint(10 / int(k)) }\nfunc main() { fmt.Println([]K{1}) }\n", "printing a value whose String method may panic is not supported"},
		{"print String hex plus", "package main\nimport \"fmt\"\ntype K int\nfunc (k K) String() string { return \"k\" }\nfunc main() { fmt.Printf(\"%+x\", K(1)) }\n", "the format %+x with a value of type main.K is not supported"},
		{"hex space", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"% x\", \"ab\") }\n", "the format % x with a value of type string is not supported"},
		// FuzzCheck, 2026-11-22: a String method without a body made the
		// checker walk a nil body.
		{"print bodiless String", "package main\nimport \"fmt\"\ntype K int\nfunc (k K) String() string\nfunc main() { fmt.Println(K(1)) }\n", "printing a value whose String method may panic is not supported"},
		{"s of int", "package main\nimport \"fmt\"\nfunc main() { x := 1; fmt.Printf(\"%s\", x) }\n", "the format %s with a value of type int is not supported"},
		{"sharp d", "package main\nimport \"fmt\"\nfunc main() { x := 1; fmt.Printf(\"%#d\", x) }\n", "the format %#d with a value of type int is not supported"},
		{"defer user func", "package main\nfunc f() {}\nfunc main() { defer f() }\n", "defer of a call other than fmt.Print, Println and Printf is not supported"},
		{"defer method", "package main\nimport \"fmt\"\nfunc main() { defer fmt.Sprint(1) }\n", "defer of a call other than fmt.Print, Println and Printf is not supported"},
		{"defer slice", "package main\nimport \"fmt\"\nfunc main() { s := []int{1}; defer fmt.Println(s) }\n", "deferring the print of a value of type []int, whose text is made when the call runs, is not supported"},
		{"defer stringer", "package main\nimport \"fmt\"\ntype K int\nfunc (k K) String() string { return \"k\" }\nfunc main() { defer fmt.Println(K(1)) }\n", "deferring the print of a value of type main.K"},
		{"defer printf arg", "package main\nimport \"fmt\"\nfunc main() { var e error; defer fmt.Printf(\"%v\", e) }\n", "deferring the print of a value of type error"},
		{"defer bad verb", "package main\nimport \"fmt\"\nfunc main() { defer fmt.Printf(\"%e\", 1.0) }\n", "the verb %e is not supported"},
		{"g precision", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%.3g\", 2.5) }\n", "the format %.3g with a value of type float64 is not supported"},
		{"g plus", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%+g\", 2.5) }\n", "the format %+g with a value of type float64 is not supported"},
		{"g zero", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%08g\", 2.5) }\n", "the format %08g with a value of type float64 is not supported"},
		{"g int", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%g\", 1) }\n", "the format %g with a value of type int is not supported"},
		{"print String plus", "package main\nimport \"fmt\"\ntype K int\nfunc (k K) String() string { return \"k\" }\nfunc main() { fmt.Printf(\"%+v\", K(1)) }\n", "the format %+v with a value of type main.K is not supported"},
		{"embedded", "package main\ntype E struct{}\ntype S struct{ E }\nfunc main() {}\n", "an embedded field is not supported"},
		{"writer field", "package main\nimport \"io\"\ntype S struct{ w io.Writer }\nfunc main() {}\n", "a struct field of type io.Writer is not supported"},
		{"map field", "package main\ntype S struct{ m map[string]int }\nfunc main() {}\n", "a map type other than a variable's (RFC 0019) is not supported"},
		{"generic func", "package main\nfunc g[T any](x T) {}\nfunc main() {}\n", "a generic function (Go 1.18) is not supported"},
		{"no body method", "package main\ntype T int\nfunc (t *T) f()\nfunc main() {}\n", "a function declared without a body (assembly): main.T.f is not supported"},
		{"no body", "package main\nfunc f()\nfunc main() {}\n", "a function declared without a body (assembly): main.f is not supported"},
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
		{"complex", "package main\nfunc main() { var x complex128; _ = x }\n", "type complex128 is not supported"},
		{"complex64", "package main\nfunc f(x complex64) {}\nfunc main() {}\n", "type complex64 is not supported"},
		{"chan", "package main\nfunc main() { var c chan int; _ = c }\n", "a channel type (goroutines and channels are planned, RFC 0002) is not supported"},
		{"type set", "package main\ntype C interface{ ~int | ~string }\nfunc main() {}\n", "an interface with a type set (a generic constraint) is not supported"},
		{"iface slice param", "package main\ntype I interface{ M([]int) }\nfunc main() {}\n", "an interface method with a slice parameter is not supported"},
		{"iface struct slice param", "package main\ntype P struct{ N int; S []int }\ntype I interface{ M(P) }\nfunc main() {}\n", "an interface method with a slice parameter is not supported"},
		{"iface error result", "package main\ntype I interface{ M() error }\nfunc main() {}\n", "an interface method with an error result is not supported"},
		{"iface pointer param", "package main\ntype I interface{ M(*int) }\nfunc main() {}\n", "an interface method with a pointer parameter is not supported"},
		{"iface pointer result", "package main\ntype I interface{ M() *int }\nfunc main() {}\n", "an interface method with a pointer result is not supported"},
		{"iface variadic", "package main\ntype I interface{ M(...int) }\nfunc main() {}\n", "a variadic interface method is not supported"},
		{"iface map param", "package main\ntype I interface{ M(map[int]int) }\nfunc main() {}\n", "a map type other than a variable's (RFC 0019) is not supported"},
		{"iface map result", "package main\ntype I interface{ M() map[int]int }\nfunc main() {}\n", "a map type other than a variable's (RFC 0019) is not supported"},
		{"pointer to iface", "package main\nfunc main() { x := 1; var a any = &x; _ = a }\n", "converting a pointer to a type other than a named type of the program to an interface"},
		{"duration to iface", "package main\nimport \"time\"\nfunc main() { var a any = time.Second; _ = a }\n", "converting a value of type time.Duration to an interface"},
		{"panicky string", "package main\ntype T []int\nfunc (t T) String() string { return string(rune(t[0])) }\nfunc main() { var a any = T{1}; _ = a }\n", "converting to an interface a value whose String method may panic"},
		{"struct method field", "package main\ntype N int\nfunc (n N) String() string { return \"n\" }\ntype S struct{ X N }\nfunc main() { var a any = S{}; _ = a }\n", "converting to an interface a struct with a field fmt prints through a method"},
		{"map to iface", "package main\nfunc main() { var a any = map[int]int{}; _ = a }\n", "a map literal or make of a map other than the value of a local map variable where it is declared (step 1 of RFC 0019) is not supported"},
		{"iface print", "package main\nimport \"fmt\"\nfunc main() { var a any = 1; fmt.Print(a) }\n", "an interface value printed with fmt.Print (use fmt.Println or Printf) is not supported"},
		{"iface verb", "package main\nimport \"fmt\"\nfunc main() { var a any = 1; fmt.Printf(\"%d\", a) }\n", "the format %d with an interface value is not supported"},
		{"T of int", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%T\", 1) }\n", "the format %T with a value of type int is not supported"},
		{"iface slice global", "package main\ntype I interface{ M() }\nvar g []I\nfunc main() { _ = g }\n", "a package-level variable holding an interface value in a struct or slice is not supported"},
		{"iface field global", "package main\ntype S struct{ N int; X any }\nvar g S\nfunc main() { _ = g.N }\n", "a package-level variable holding an interface value in a struct or slice is not supported"},
		{"pointer slice global", "package main\ntype T struct{ L string }\nvar g = []*T{{L: \"a\"}}\nfunc main() { _ = g[0].L }\n", "a package-level variable holding pointers in a struct or slice is not supported"},
		{"pointer field global", "package main\ntype N struct{ Next *N }\ntype S struct{ Items [2][]*N; V int }\nvar g S\nfunc main() { _ = g.V }\n", "a package-level variable holding pointers in a struct or slice is not supported"},
		{"repeated field type global", "package main\ntype M struct{ V int }\ntype N struct{ V int }\ntype S struct{ A, B M; P []*N }\nvar g S\nfunc main() { _ = g.A.V }\n", "a package-level variable holding pointers in a struct or slice is not supported"},
		{"iface writer param", "package main\nimport \"io\"\ntype I interface{ M(io.Writer) }\nfunc main() {}\n", "an interface method with an io.Writer parameter is not supported"},
		{"writer case", "package main\nimport \"io\"\nfunc main() { var x any; switch x.(type) { case io.Writer: } }\n", "io.Writer as a type switch case or assertion type is not supported"},
		{"writer slice", "package main\nimport \"io\"\nfunc f(w []io.Writer) {}\nfunc main() {}\n", "type []io.Writer is not supported"},
		{"error precision", "package main\nimport (\"errors\"; \"fmt\")\nfunc main() { err := errors.New(\"x\"); fmt.Printf(\"%.2v\", err) }\n", "the format %.2v with an error value is not supported"},
		{"string error print", "package main\nimport \"fmt\"\ntype E string\nfunc (e E) Error() string { return string(e) }\nfunc main() { var err error = E(\"x\"); fmt.Print(err) }\n", "an interface value printed with fmt.Print"},
		{"w outside errorf", "package main\nimport (\"errors\"; \"fmt\")\nfunc main() { err := errors.New(\"x\"); fmt.Printf(\"%w\", err) }\n", "%w outside fmt.Errorf is not supported"},
		{"two w", "package main\nimport (\"errors\"; \"fmt\")\nfunc main() { err := errors.New(\"x\"); _ = fmt.Errorf(\"%w %w\", err, err) }\n", "more than one %w in a format"},
		{"w not error", "package main\nimport (\"errors\"; \"fmt\")\nfunc main() { _ = errors.New(\"x\"); _ = fmt.Errorf(\"%w\", 1) }\n", "%w of a value that is not of type error"},
		{"w width", "package main\nimport (\"errors\"; \"fmt\")\nfunc main() { err := errors.New(\"x\"); _ = fmt.Errorf(\"%5w\", err) }\n", "the format %5w is not supported"},
		{"w expr", "package main\nimport (\"errors\"; \"fmt\")\nfunc main() { _ = fmt.Errorf(\"%w\", errors.New(\"x\")) }\n", "%w of an expression other than a variable"},
		{"as target", "package main\nimport (\"errors\"; \"fmt\")\ntype E struct{ X int }\nfunc (e E) Error() string { return \"e\" }\nvar t E\nfunc main() { err := errors.New(\"x\"); _ = errors.As(err, &t); fmt.Println() }\n", "an errors.As target other than &v for a local variable v"},
		{"as int", "package main\nimport (\"errors\"; \"fmt\")\nfunc main() { var n int; err := errors.New(\"x\"); _ = errors.As(err, &n); fmt.Println() }\n", "an errors.As target whose type is neither an interface nor an error type"},
		{"as pointer unwrap", "package main\nimport (\"errors\"; \"fmt\")\ntype E struct{ err error }\nfunc (e *E) Error() string { return \"e\" }\nfunc (e *E) Unwrap() error { return e.err }\nfunc main() { var err error = &E{}; var e *E; fmt.Println(errors.As(err, &e)) }\n", "errors.As to a pointer in a program with an error type that has an Unwrap method"},
		{"as method", "package main\nimport (\"errors\"; \"fmt\")\ntype E struct{}\nfunc (e E) Error() string { return \"e\" }\nfunc (e E) As(t any) bool { return false }\nfunc main() { var err error = E{}; var e E; _ = errors.As(err, &e); fmt.Println() }\n", "errors.As in a program with an error type that has an As method"},
		{"unwrap slice", "package main\nimport \"fmt\"\ntype E struct{}\nfunc (e E) Error() string { return \"e\" }\nfunc (e E) Unwrap() []error { return nil }\nfunc main() { var err error = E{}; fmt.Println(err) }\n", "an error type with Unwrap() []error"},
		{"iface switch tag", "package main\nfunc main() { var a any = 1; switch a { case 1: } }\n", "a switch on an interface value (use a type switch) is not supported"},
		{"func type", "package main\nfunc f(g func(string)) {}\nfunc main() {}\n", "a parameter of function type taking string (step 2 of RFC 0018 takes numbers and bools) is not supported"},
		{"func type variable", "package main\nvar g func()\nfunc main() {}\n", "a function type is not supported"},
		{"array", "package main\nfunc main() { var a [3]int; _ = a }\n", "an array type is not supported"},
		{"anon struct var", "package main\nfunc main() { var a struct{}; _ = a }\n", "an anonymous struct type is not supported"},
		{"writer local", "package main\nimport (\"io\"; \"os\")\nfunc main() { var w io.Writer = os.Stdout; _ = w }\n", "a local variable of type io.Writer is not supported"},
		{"go", "package main\nfunc f() {}\nfunc main() { go f() }\n", "the go statement (goroutines are planned, RFC 0002) is not supported"},
		{"defer", "package main\nfunc f() {}\nfunc main() { defer f() }\n", "defer of a call other than fmt.Print, Println and Printf is not supported"},
		{"select", "package main\nfunc main() { select {} }\n", "select (planned, RFC 0002) is not supported"},
		{"send", "package main\nfunc f(c chan int) { c <- 1 }\nfunc main() {}\n", "a channel send (planned, RFC 0002) is not supported"},
		{"label", "package main\nfunc main() {\nL:\n\tfor {\n\t\tbreak L\n\t}\n}\n", "a labeled statement is not supported"},
		{"goto", "package main\nfunc main() {\n\tgoto end\nend:\n}\n", "goto is not supported"},
		{"fallthrough", "package main\nfunc main() {\n\tswitch 1 {\n\tcase 1:\n\t\tfallthrough\n\tdefault:\n\t}\n}\n", "fallthrough is not supported"},
		{"type switch init", "package main\nfunc main() { var x any; switch y := 1; x.(type) { default: _ = y } }\n", "a type switch with an init statement is not supported"},
		{"type switch case", "package main\nfunc main() { var x any; switch x.(type) { case *int: } }\n", "converting a pointer to a type other than a named type of the program to an interface"},
		{"assert iface", "package main\ntype I interface{ M([]int) }\nfunc main() { var x any; _ = x.(I) }\n", "an interface method with a slice parameter is not supported"},
		{"expr stmt", "package main\nfunc main() { c := make(chan int); <-c }\n", "a channel receive as a statement is not supported"},
		{"range map", "package main\nfunc main() { for range map[int]int{} {} }\n", "range over a map other than a map variable (step 3 of RFC 0019) is not supported"},
		{"range func", "package main\nfunc seq(yield func(int) bool) {}\nfunc main() { for range seq {} }\n", "range over a function (Go 1.23) is not supported"},
		{"range float", "package main\nfunc main() { var c chan int; for range c {} }\n", "range over a channel (chan int) is not supported"},
		{"range assign", "package main\nfunc main() { i := 0; for i = range 3 {}; _ = i }\n", "a range loop that assigns to existing variables (use :=) is not supported"},
		{"string byte assign", "package main\nfunc main() { var b [1]byte; s := \"x\"; _ = b; _ = s[0] }\n", "an array type is not supported"},
		{"assign target", "package main\nfunc f() []int { return nil }\nfunc main() { f()[0] = 1 }\n", "this assignment target is not supported"},
		{"invalid utf8 const", "package main\nconst s = \"\\xff\"\nfunc main() {}\n", "a string constant that is not valid UTF-8 is not supported"},
		{"invalid utf8 expr", "package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"\\xff\") }\n", "a string constant that is not valid UTF-8 is not supported"},
		{"complex", "package main\nimport \"fmt\"\nfunc main() { fmt.Println(1i) }\n", "a complex number is not supported"},
		{"func value", "package main\nfunc f() {}\nfunc main() { g := f; _ = g }\n", "using function f as a value is not supported"},
		{"std value", "package main\nimport \"strings\"\nfunc main() { f := strings.Repeat; _ = f }\n", "using strings.Repeat as a value is not supported"},
		{"std var", "package main\nimport \"os\"\nfunc main() { _ = os.Args }\n", "os.Args is not supported"},
		{"std type", "package main\nimport \"io\"\nfunc main() { var r io.Reader; _ = r }\n", "io.Reader is not supported"},
		{"std func", "package main\nimport \"strings\"\nfunc main() { _ = strings.Title(\"x\") }\n", "strings.Title is not supported"},
		{"method value", "package main\ntype T struct{}\nfunc (T) M() {}\nfunc main() { f := T{}.M; _ = f }\n", "a method value is not supported"},
		{"method expr", "package main\ntype T struct{}\nfunc (T) M() {}\nfunc main() { f := T.M; _ = f }\n", "a method expression is not supported"},
		{"error method", "package main\nimport \"errors\"\nfunc f() error { return errors.New(\"x\") }\nfunc main() { if err := f(); err != nil { _ = err.Error } }\n", "a method value is not supported"},
		{"writer method", "package main\nimport (\"io\"; \"os\")\nfunc f(w io.Writer) { w.Write(nil) }\nfunc main() { f(os.Stdout) }\n", "calling methods on an io.Writer"},
		{"anon struct lit", "package main\nfunc main() { _ = struct{ x int }{1} }\n", "an anonymous struct type is not supported"},
		{"keyed slice", "package main\nfunc main() { _ = []int{2: 1} }\n", "an indexed slice literal element is not supported"},
		{"map lit", "package main\nfunc main() { _ = map[int]int{} }\n", "a map literal or make of a map other than the value of a local map variable where it is declared (step 1 of RFC 0019) is not supported"},
		{"addr of call", "package main\ntype T struct{ X int }\nfunc f() T { return T{} }\nfunc g(p *T) {}\nfunc main() { g(&[]T{{}}[0]) }\n", "taking the address of this expression is not supported"},
		{"addr of pointer", "package main\nfunc main() { x := 1; p := &x; q := &p; _ = q }\n", "a pointer to a pointer variable that escapes is not supported"},
		{"interior escapes", "package main\ntype T struct{ X int }\nfunc f(t *T) *int { return &t.X }\nfunc main() { _ = f(&T{}) }\n", "a pointer into a struct field or a slice element that escapes (RFC 0013) is not supported"},
		{"range addr escapes", "package main\ntype T struct{ X int }\nvar keep *T\nfunc main() { for _, t := range []T{{}} { keep = &t } }\n", "the address of a range variable that escapes is not supported"},
		{"arrow", "package main\nfunc main() { c := make(chan int); _ = <-c }\n", "the <- operator is not supported"},
		{"slice compare", "package main\nfunc main() { var s []int; _ = s == nil }\n", "comparing a slice with nil is not supported"},
		{"call value", "package main\ntype T struct{ f func() }\nfunc main() { T{}.f() }\n", "a function type is not supported"},
		{"ellipsis", "package main\nfunc f(xs ...int) {}\nfunc main() { f([]int{}...) }\n", "a call with ... arguments is not supported"},
		{"instantiate", "package main\nimport \"slices\"\nfunc main() { _ = slices.Clone[[]int] }\n", "an instantiated generic function (Go 1.18) is not supported"},
		{"conversion string", "package main\nfunc main() { b := []byte(\"x\"); _ = string(b) }\n", "converting a []byte to a string (patina strings must be valid UTF-8) is not supported"},
		{"conversion rune", "package main\nfunc main() { n := 65; _ = string(rune(n)) }\n", "converting an integer to a string is not supported"},
		{"conversion other", "package main\nfunc main() { _ = []rune(\"x\") }\n", "converting string to []rune is not supported"},
		{"make map", "package main\nfunc main() { _ = make(map[int]int) }\n", "a map literal or make of a map other than the value of a local map variable where it is declared (step 1 of RFC 0019) is not supported"},
		{"panic value", "package main\nfunc main() { panic(1) }\n", "panic with a value that is not a string or an error is not supported"},
		{"min one arg", "package main\nfunc main() { x := 1; _ = min(x) }\n", "the min built-in with fewer than two arguments is not supported"},
		{"max float", "package main\nfunc main() { x := 1.0; _ = max(x, 2.0) }\n", "the max built-in with a non-integer argument is not supported"},
		{"clear", "package main\nfunc main() { s := []int{1}; clear(s) }\n", "the clear built-in (Go 1.21) is not supported"},
		{"cap", "package main\nfunc main() { _ = cap([]int{}) }\n", "the cap built-in is not supported"},
		{"format not const", "package main\nimport \"fmt\"\nfunc main() { f := \"%d\"; fmt.Printf(f, 1) }\n", "a format string that is not a constant is not supported"},
		{"format bad", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%[1]d\", 1) }\n", "'[' in a directive is not supported"},
		{"format missing", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%d %d\", 1) }\n", "format string: %d has no operand"},
		{"format extra", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%d\", 1, 2) }\n", "format string has 1 verbs but 2 operands are given"},
		{"verb", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%d\", \"x\") }\n", "the format %d with a value of type string is not supported"},
		{"print struct, an error field", "package main\nimport \"fmt\"\ntype T struct{ e error }\nfunc main() { fmt.Println(T{}) }\n", "printing a struct of type main.T, whose field e is of type error is not supported"},
		{"print struct, a field with String", "package main\nimport \"fmt\"\ntype N int\nfunc (n N) String() string { return \"n\" }\ntype T struct{ n N }\nfunc main() { fmt.Println(T{}) }\n", "printing a struct of type main.T, whose field n fmt prints through its String method is not supported"},
		{"print struct, a nested struct's field", "package main\nimport \"fmt\"\ntype U struct{ e error }\ntype T struct{ u U }\nfunc main() { fmt.Println(T{}) }\n", "printing a struct of type main.U, whose field e is of type error is not supported"},
		{"print struct with %+v", "package main\nimport \"fmt\"\ntype T struct{ a int }\nfunc main() { fmt.Printf(\"%+v\\n\", T{}) }\n", "the format %+v with a value of type main.T is not supported"},
		{"print struct with a width", "package main\nimport \"fmt\"\ntype T struct{ a int }\nfunc main() { fmt.Printf(\"%8v\\n\", T{}) }\n", "the format %8v with a value of type main.T is not supported"},
		{"%q of an int", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%q\\n\", 65) }\n", "the format %q with a value of type int is not supported"},
		{"%+q", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%+q\\n\", \"x\") }\n", "the format %+q with a value of type string is not supported"},
		{"%#q", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%#q\\n\", \"x\") }\n", "the format %#q with a value of type string is not supported"},
		{"%.2q", "package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%.2q\\n\", \"xyz\") }\n", "the format %.2q with a value of type string is not supported"},
		{"%+q of a String method", "package main\nimport \"fmt\"\ntype N int\nfunc (n N) String() string { return \"n\" }\nfunc main() { fmt.Printf(\"%+q\\n\", N(1)) }\n", "the format %+q with a value of type main.N is not supported"},
		{"bytes sprintf", "package main\nimport \"fmt\"\nfunc main() { _ = fmt.Sprintf(\"%s\", []byte(\"x\")) }\n", "the format %s with a value of type []byte is not supported"},
		{"time func", "package main\nimport \"time\"\nfunc main() { _ = time.Now() }\n", "time.Now is not supported"},
		{"time type", "package main\nimport \"time\"\nfunc f(t time.Month) {}\nfunc main() {}\n", "type time.Month is not supported"},
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

// checkModule loads a module of several packages (RFC 0009) from files
// and returns the subset diagnostics, rendered.
func checkModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	files["go.mod"] = "module example.com/m\n\ngo 1.25\n"
	for name, src := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := load.Dir(dir, dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return (&diag.Error{Dir: dir, Diags: Check(pkg)}).Error()
}

func TestPackages(t *testing.T) {
	got := checkModule(t, map[string]string{
		"main.go": "package main\n\nimport \"example.com/m/a\"\n\nfunc main() {\n\tf := a.F\n\t_ = f\n\t_ = a.V\n\ta.F()\n\t_ = a.T(a.C)\n}\n",
		"a/a.go":  "package a\n\nconst C = 1\n\ntype T int\n\nvar V = 2\n\nfunc F() {}\n",
	})
	for _, want := range []string{
		"main.go:6:7: using example.com/m/a.F as a value is not supported",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("diagnostics lack %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "is not supported") != 2 {
		t.Errorf("unexpected diagnostics:\n%s", got)
	}
}

func TestGlobals(t *testing.T) {
	got := checkModule(t, map[string]string{
		"main.go": "package main\n\nimport \"example.com/m/a\"\n\nfunc main() {\n\ta.V++\n\ta.S.Name = \"x\"\n}\n",
		"a/a.go":  "package a\n\nvar V int\n\ntype T struct{ Name string }\n\nvar S T\n",
	})
	if !strings.Contains(got, "a package-level variable of type example.com/m/a.T that the program changes is not supported") || strings.Count(got, "is not supported") != 1 {
		t.Errorf("diagnostics:\n%s", got)
	}
	src := `package main

import "fmt"

var table = []int{1, 2}

type Cfg struct{ Items []int }

var cfg = Cfg{Items: []int{3}}

var n int

var _ = f()

func f() int { n++; return n }

type C struct{ x int }

func (c C) get() int { return c.x + n }

func main() {
	for _, x := range table {
		fmt.Println(x, cfg.Items[0])
	}
	fmt.Println(n + f(), f() > 0 && f() > n, n|f(), C{x: f()}.get(), len([]byte("x")))
}
`
	if got := check(t, src); got != "" {
		t.Fatalf("unexpected diagnostics:\n%s", got)
	}
	pkg, err := load.Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	var mainFn *ast.FuncDecl
	for _, d := range pkg.Files[0].Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == "main" {
			mainFn = fd
		}
	}
	stmt := mainFn.Body.List[1]
	var order []string
	for _, c := range HoistableCalls(pkg, stmt) {
		order = append(order, fmt.Sprint(pkg.Fset.Position(c.Pos()).Column))
	}
	// f, f (the left operand of &&, not the right), f, the inner f, then
	// C{...}.get() (a call is at its start); not fmt.Println, len or the
	// conversion.
	if got := strings.Join(order, " "); got != "18 23 45 55 50" {
		t.Errorf("hoistable calls at columns %s", got)
	}
	kinds := Globals(pkg)
	if !ReadsChangedGlobal(pkg, stmt, kinds) || ReadsChangedGlobal(pkg, mainFn.Body.List[0], kinds) {
		t.Error("ReadsChangedGlobal")
	}
}

// TestProvidedMethodMissing covers a method of a provided type that patina
// mode does not provide yet.
func TestProvidedMethodMissing(t *testing.T) {
	ProvidedTypes["time.Month"] = true
	t.Cleanup(func() { delete(ProvidedTypes, "time.Month") })
	got := check(t, "package main\nimport (\"fmt\"; \"time\")\nfunc main() { fmt.Println(time.January.String()) }\n")
	if !strings.Contains(got, "time.Month.String is not supported") {
		t.Errorf("got %s", got)
	}
	if fn, name := ProvidedMethod(types.Universe.Lookup("error").Type().Underlying().(*types.Interface).Method(0)); fn != "" || name != "" {
		t.Errorf("error.Error: %q %q", fn, name)
	}
	plain := types.NewFunc(0, nil, "f", types.NewSignatureType(nil, nil, nil, nil, nil, false))
	if fn, name := ProvidedMethod(plain); fn != "" || name != "" {
		t.Errorf("a function: %q %q", fn, name)
	}
}

// TestLibraryAccepted checks library programs RFC 0014 accepts: constants
// and a blank variable, a method of an unexported type, a slice of structs
// and a pointer that only borrows in the API.
func TestLibraryAccepted(t *testing.T) {
	src := "package lib\n\nconst Max = 3\n\nvar _ = Max\n\ntype inner struct{ n int }\n\nfunc (i inner) Get() int { return i.n }\n\n" +
		"type T struct{ N int }\n\nfunc Sum(ts []T) int {\n\ts := 0\n\tfor _, t := range ts {\n\t\ts += t.N\n\t}\n\treturn s\n}\n\n" +
		"func Bump(t *T) { t.N++ }\n"
	if got := check(t, src); got != "" {
		t.Errorf("rejected:\n%s", got)
	}
}

func TestMapRangeOtherPackage(t *testing.T) {
	got := checkModule(t, map[string]string{
		"main.go": "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/m/a\"\n)\n\nfunc main() {\n\tfor k := range a.M {\n\t\tfmt.Println(k)\n\t}\n}\n",
		"a/a.go":  "package a\n\nvar M = map[string]int{\"x\": 1}\n",
	})
	want := "main.go:10:17: range over a package-level map of another package (step 3 of RFC 0019) is not supported"
	if !strings.Contains(got, want) || strings.Count(got, "is not supported") != 1 {
		t.Errorf("diagnostics lack %q, or have more:\n%s", want, got)
	}
}

func TestMapRangeAccepted(t *testing.T) {
	// Changing another map, a slice or a copy of a value while ranging
	// over a map is not changing the map (decision 5 of RFC 0019).
	pkg, err := load.Source("/m/app", "main.go", `package main

import "fmt"

type pt struct{ x int }

func main() {
	m := map[string]int{"a": 1}
	n := map[string]int{}
	p := pt{}
	s := []int{0}
	for k, v := range m {
		n[k] = v
		delete(n, "z")
		v++
		s[0]++
		p.x++
		fmt.Println(m, k, v)
	}
	fmt.Println(n, s, p.x)
}
`, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	if ds := Check(pkg); len(ds) > 0 {
		t.Errorf("rejected: %v", ds)
	}
}
