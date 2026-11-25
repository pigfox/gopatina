// Package progen generates random Go programs inside the phase 1 patina
// subset, for differential testing against the Go toolchain
// (cmd/gopatina-difftest, docs/QUALITY.md).
//
// A batch is one package main holding many independent test programs: each
// is a function pN with its own helper hN, and main calls them in order,
// printing a marker line "#N" before each. Comparing the output segment by
// segment counts each program separately while the batch is built only once
// per toolchain.
//
// Every program is valid Go that terminates without a panic: literals are
// held in variables, so no constant expression can overflow at compile time;
// divisors are made odd, so never zero; indexes are reduced modulo the
// slice's fixed length; loops have small constant bounds. Within that, the
// programs exercise wrapping arithmetic (including MinInt64 / -1), shifts,
// bitwise operators, min and max, slices and append, switch, string
// building and formatted printing. The same seed always gives the same
// source.
//
// Programs also contain the loop nests RFC 0006 specializes: counted loops
// over a slice made with a length held in a variable, sieve-like and cube
// nests (for b := a*a; b < m; b += a), whose bodies read the loop
// variables, also as the left operand of << and <; now and then the cube nest runs at exactly its threshold
// (m = 2097152, the largest the fast copy takes).
//
// Each program also declares a named type of its own (RFC 0007): an
// integer type of a random width with a String method, a method doing
// wrapping arithmetic in that width (division, remainder, and shifts by
// constants up to past the width), and a named slice of it with a
// pointer-receiver method, sometimes through an alias; the program converts
// its values to the type, calls the methods and prints them. That part
// comes from a random stream of its own, so the rest of each program is the
// same as before it was added.
//
// Each program also converts float64 values made from its integers to int,
// int32 and uint8 and prints them: products and quotients at scales up to
// far past the integer ranges, NaN and the infinities made at run time.
// Go leaves an out-of-range conversion to the compiler; the tests hold
// GoPatina to gc's. This part, too, has a random stream of its own.
//
// Each program also prints with fmt's widths and flags: a width on a slice
// of ints and on a slice printed through String (Go pads each element),
// %x and %X with the 0, # and - flags on an integer and on a value printed
// through String, and %x of its string and of the string's bytes. This
// part has a random stream of its own too.
//
// Each program also builds a string from its values and words where Go's
// Unicode case mapping and folding differ from Rust's, and calls the
// provided strings API on it (RFC 0011): ToLower, ToUpper, EqualFold,
// Split, Cut, TrimPrefix, TrimSuffix, ReplaceAll, IndexByte. This part has
// a random stream of its own.
//
// Each program also makes time.Duration values from its integers in every
// unit and calls every method of it patina mode provides (RFC 0011),
// printing them directly, with widths and in a slice, from a random stream
// of its own.
//
// Each program also declares an interface with two implementing types, a
// struct and a named integer, converts its values to it and to any, calls
// through them, switches on and asserts their dynamic types, compares them
// and prints them with %v and %T (RFC 0012), from a random stream of its
// own.
//
// Each program also has an error type and a function returning an error
// of that type, one made by errors.New, or nil, and keeps its errors as
// values: in a slice, compared, switched on, asserted and printed, and
// wrapped with %w and walked with errors.Unwrap, Is and As (RFC 0012),
// from a random stream of its own.
//
// Each program also shifts values of a random integer type by counts of a
// random integer type known only at run time, from 0 to far past the width
// (never negative: that panics, and the golden programs test it), with
// << >> <<= and >>=, from a random stream of its own.
//
// Programs are also of two packages (RFC 0009): every batch comes with a
// package lib (Lib), at LibPath in module Module, where each program has a
// struct type with value and pointer methods, a constant, a function of
// random wrapping arithmetic and a named integer type with a String
// method, and a function returning an error, sometimes its package-level
// sentinel error, which the program uses across the package boundary. lib comes from a random stream of its own.
//
// Programs also have package-level variables (RFC 0010): in lib, a
// counter changed by a function, a read-only table read through one, a
// changed string and a struct changed field by field from main; in main, a
// changed counter. A function returns the counter beside a call that
// changes it. Statements mix their reads with calls that change them,
// so the tests hold GoPatina to gc's order (calls first). Declarations and
// uses each come from a random stream of their own.
//
// After the n programs, each batch ends with one more, its panic case,
// numbered first+n: a program of the same kind that then, in most batches,
// panics at run time, by an index out of range (positive or negative, of a
// slice or a string), an integer division or remainder by zero, a make
// length or capacity out of range, an explicit panic, or the cube nest one
// past its threshold, where a*a*a wraps to a negative index. A panic ends the
// binary, so only the last program can test one; comparing its output, the
// first line of standard error and the exit status tests Go's panic
// behaviour once per batch. The panic case depends on the seed and on
// first+n.
package progen

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
)

// Marker is the prefix of the line main prints before each program.
const Marker = "#"

// Module is the module of a batch, and LibPath the import path of its
// second package.
const (
	Module  = "difftest"
	LibPath = Module + "/lib"
)

// globalDeclStream and globalUseStream are the random streams of a
// program's package-level variables and of its uses of them.
func globalDeclStream(seed, k uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed^0x6a09e667f3bcc908, k))
}

func globalUseStream(seed, k uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed^0xbb67ae8584caa73b, k))
}

// libStream is the random stream of a program's declarations in lib.
func libStream(seed, k uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed^0x1f83d9abfb41bd6b, k))
}

// Lib returns the source of package lib for the batch Batch(seed, first, n)
// makes: the declarations of programs first to first+n, the panic case
// included.
func Lib(seed uint64, first, n int) string {
	var b strings.Builder
	b.WriteString("// Code generated by internal/progen. DO NOT EDIT.\n\n// Package lib holds the generated programs' declarations of a second\n// package.\npackage lib\n\nimport \"fmt\"\n")
	for k := first; k <= first+n; k++ {
		key := uint64(k)
		if k == first+n {
			key = 1<<63 | key
		}
		g := &gen{lr: libStream(seed, key), gr: globalDeclStream(seed, key), k: k}
		g.libDecls(&b)
		g.globalDecls(&b)
	}
	return b.String()
}

// libDecls writes program k's declarations in lib.
func (g *gen) libDecls(b *strings.Builder) {
	k := g.k
	ints := []string{"int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64"}
	fmt.Fprintf(b, "\n// P%d is program %d's struct type.\ntype P%d struct{ A, B int }\n", k, k, k)
	fmt.Fprintf(b, "\nfunc (p P%d) Sum() int { return p.A + p.B*K%d }\n", k, k)
	fmt.Fprintf(b, "\nfunc (p *P%d) Grow(x int) {\n\tp.A ^= x\n\tp.B += x\n}\n", k)
	fmt.Fprintf(b, "\nconst K%d = %d\n", k, g.lr.IntN(2001)-1000)
	// A list of package lib that main builds and owns (RFC 0017 with RFC
	// 0009): a Box of another package's type, not a shared cell.
	fmt.Fprintf(b, "\n// Lst%d is program %d's list, newest first.\ntype Lst%d struct {\n\tV    int\n\tT    Tally%d\n\tNext *Lst%d\n}\n", k, k, k, k, k)
	fmt.Fprintf(b, "\nfunc Cons%d(v int, next *Lst%d) *Lst%d { return &Lst%d{V: v, Next: next} }\n", k, k, k, k)
	fmt.Fprintf(b, "\nfunc (l *Lst%d) Sum() int {\n\tt := 0\n\tfor p := l; p != nil; p = p.Next {\n\t\tt += p.V\n\t}\n\treturn t\n}\n", k)
	fmt.Fprintf(b, "\n// Tally%d counts; no pointer to one escapes, so &l.T is lent as &mut.\ntype Tally%d int\n", k, k)
	fmt.Fprintf(b, "\nfunc AddTo%d(x *Tally%d, d int) { *x += Tally%d(d) }\n", k, k, k)
	fmt.Fprintf(b, "\nfunc (l *Lst%d) Bump(d int) {\n\tl.V += d\n\tif l.Next != nil {\n\t\tl.Next.Bump(d)\n\t}\n}\n", k)
	ops := []string{"+", "-", "*", "&", "|", "^"}
	expr := "x"
	for range 1 + g.lr.IntN(4) {
		y := []string{"x", "y", fmt.Sprint(g.lr.IntN(64))}[g.lr.IntN(3)]
		expr = fmt.Sprintf("(%s %s %s)", expr, ops[g.lr.IntN(len(ops))], y)
	}
	fmt.Fprintf(b, "\nfunc F%d(x, y int) int { return %s / (y | 1) }\n", k, expr)
	fmt.Fprintf(b, "\ntype N%d %s\n", k, ints[g.lr.IntN(len(ints))])
	fmt.Fprintf(b, "\nfunc (n N%d) String() string { return fmt.Sprintf(\"n%%d\", int(n)) }\n", k)
	// G returns an error to main, from another package of the program:
	// the sentinel ES for multiples of 3.
	fmt.Fprintf(b, "\nvar ES%d = fmt.Errorf(\"sentinel %d\")\n", k, k)
	// An error handed straight back from an if's init statement (Rust's ?).
	fmt.Fprintf(b, "\nfunc Pos%d(x int) error {\n\tif x < 0 {\n\t\treturn fmt.Errorf(\"negative %%d\", x)\n\t}\n\treturn nil\n}\n", k)
	fmt.Fprintf(b, "\nfunc Dbl%d(x int) (int, error) {\n\tif err := Pos%d(x); err != nil {\n\t\treturn 0, err\n\t}\n\tif _, err := G%d(x); err != nil {\n\t\treturn 0, err\n\t}\n\treturn x * 2, nil\n}\n", k, k, k)
	fmt.Fprintf(b, "\nfunc G%d(x int) (int, error) {\n\tif x%%3 == 0 {\n\t\treturn 0, ES%d\n\t}\n\tif x%%2 != 0 {\n\t\treturn 0, fmt.Errorf(\"odd %%d\", x)\n\t}\n\treturn x / 2, nil\n}\n", k, k)
}

// Batch returns the source of a package main with n programs, numbered
// first, first+1, ..., generated from seed.
func Batch(seed uint64, first, n int) string {
	var b strings.Builder
	b.WriteString("// Code generated by internal/progen. DO NOT EDIT.\n\npackage main\n\nimport (\n\t\"errors\"\n\t\"fmt\"\n\t\"strings\"\n\t\"time\"\n\t_ \"unicode/utf8\"\n\n\t\"" + LibPath + "\"\n)\n")
	for k := first; k < first+n; k++ {
		g := &gen{r: rand.New(rand.NewPCG(seed, uint64(k))), nr: namedStream(seed, uint64(k)), fr: floatStream(seed, uint64(k)), pr: printStream(seed, uint64(k)), sr: shiftStream(seed, uint64(k)), xr: stringsStream(seed, uint64(k)), dr: durationStream(seed, uint64(k)), ir: ifaceStream(seed, uint64(k)), er: errorStream(seed, uint64(k)), qr: ptrStream(seed, uint64(k)), gr: globalUseStream(seed, uint64(k)), k: k}
		g.program(&b)
	}
	last := first + n
	g := &gen{r: rand.New(rand.NewPCG(seed, 1<<63|uint64(last))), nr: namedStream(seed, 1<<63|uint64(last)), fr: floatStream(seed, 1<<63|uint64(last)), pr: printStream(seed, 1<<63|uint64(last)), sr: shiftStream(seed, 1<<63|uint64(last)), xr: stringsStream(seed, 1<<63|uint64(last)), dr: durationStream(seed, 1<<63|uint64(last)), ir: ifaceStream(seed, 1<<63|uint64(last)), er: errorStream(seed, 1<<63|uint64(last)), qr: ptrStream(seed, 1<<63|uint64(last)), gr: globalUseStream(seed, 1<<63|uint64(last)), k: last, finale: true}
	g.program(&b)
	b.WriteString("\nfunc main() {\n")
	for k := first; k <= last; k++ {
		fmt.Fprintf(&b, "\tfmt.Println(%q)\n\tp%d()\n", fmt.Sprintf("%s%d", Marker, k), k)
	}
	b.WriteString("}\n")
	return b.String()
}

// namedStream is the random stream of a program's named types.
func namedStream(seed, k uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed^0x9e3779b97f4a7c15, k))
}

// floatStream is the random stream of a program's float conversions.
func floatStream(seed, k uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed^0x3c6ef372fe94f82b, k))
}

// shiftStream is the random stream of a program's variable shifts.
func shiftStream(seed, k uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed^0x510e527fade682d1, k))
}

// stringsStream is the random stream of a program's strings calls.
func stringsStream(seed, k uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed^0x9b05688c2b3e6c1f, k))
}

// durationStream is the random stream of a program's time.Duration values.
func durationStream(seed, k uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed^0x5be0cd19137e2179, k))
}

// ifaceStream is the random stream of a program's interface values.
func ifaceStream(seed, k uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed^0x243f6a8885a308d3, k))
}

// errorStream is the random stream of a program's error values.
func errorStream(seed, k uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed^0x13198a2e03707344, k))
}

// ptrStream is the random stream of a program's pointers that escape.
func ptrStream(seed, k uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed^0xa4093822299f31d0, k))
}

// printStream is the random stream of a program's fmt widths and flags.
func printStream(seed, k uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed^0x78dde6e5fd29f05b, k))
}

type gen struct {
	gr     *rand.Rand // the package-level variables' stream (declarations or uses)
	lr     *rand.Rand // the lib declarations' stream
	fr     *rand.Rand // the float conversions' stream
	pr     *rand.Rand // the fmt widths' and flags' stream
	sr     *rand.Rand // the variable shifts' stream
	xr     *rand.Rand // the strings calls' stream
	dr     *rand.Rand // the time.Duration values' stream
	ir     *rand.Rand // the interface values' stream
	er     *rand.Rand // the error values' stream
	qr     *rand.Rand // the pointers' stream (RFC 0013)
	r      *rand.Rand
	nr     *rand.Rand // the named types' stream
	under  string     // the named integer type's underlying type
	alias  bool       // whether the program uses an alias of it
	finale bool       // the batch's panic case
	k      int
	vars   []string // int variables that may be assigned
	reads  []string // loop variables in scope, read but never assigned
	loops  int      // loop variables made so far
	depth  int      // statement nesting
	sl     int      // the fixed length of s
	out    *strings.Builder
}

// interesting are literals near the edges where Go and Rust differ.
var interesting = []int64{
	0, 1, -1, 2, -2, 7, 63, 64, 255, 256, 1 << 31, -(1 << 31), 1<<32 + 1,
	math.MaxInt64, math.MinInt64, math.MaxInt64 - 1, math.MinInt64 + 1,
}

func (g *gen) literal() string {
	if g.r.IntN(2) == 0 {
		return fmt.Sprint(interesting[g.r.IntN(len(interesting))])
	}
	return fmt.Sprint(g.r.Int64N(2001) - 1000)
}

func (g *gen) line(format string, args ...any) {
	g.out.WriteString(strings.Repeat("\t", g.depth+1))
	fmt.Fprintf(g.out, format, args...)
	g.out.WriteString("\n")
}

func (g *gen) program(b *strings.Builder) {
	g.out = b
	// The helper takes two ints and returns one.
	fmt.Fprintf(b, "\nvar M%d uint8 = %d\n", g.k, g.gr.IntN(256))
	fmt.Fprintf(b, "\nfunc h%d(a, b int) int {\n", g.k)
	g.vars = []string{"a", "b"}
	fmt.Fprintf(b, "\treturn %s\n}\n", g.expr(2, false))
	g.namedDecls(b)
	// K takes a string by value and keeps it, for an operand that reads a
	// string another operand hands to it (fmt.Println(s, K(s))).
	fmt.Fprintf(b, "\nfunc K%d(s string) string {\n\ts += \".\"\n\treturn s\n}\n", g.k)
	g.ifaceDecls(b)
	g.errorDecls(b)
	g.ptrDecls(b)

	fmt.Fprintf(b, "\nfunc p%d() {\n", g.k)
	nv := 3 + g.r.IntN(3)
	g.vars = nil
	for i := range nv {
		g.line("v%d := %s", i, g.literal())
		g.vars = append(g.vars, fmt.Sprintf("v%d", i))
	}
	g.sl = 1 + g.r.IntN(8)
	g.line("s := make([]int, %d)", g.sl)
	g.line("for i := range s {")
	g.depth++
	g.line("s[i] = %s * (i + 1)", g.vars[0])
	g.depth--
	g.line("}")
	g.line("var t []int")
	g.line(`str := ""`)
	g.line("v0 = h%d(v0, %s)", g.k, g.vars[1])
	for range 4 + g.r.IntN(8) {
		g.stmt()
	}
	if g.finale {
		g.panicking()
	}
	g.line("sum := 0")
	g.line("for _, x := range s {")
	g.depth++
	g.line("sum += x")
	g.depth--
	g.line("}")
	g.line("fmt.Println(%s, sum, len(t), len(str))", strings.Join(g.vars, ", "))
	g.line(`fmt.Println(str)`)
	g.namedUses()
	g.floatUses()
	g.printUses()
	g.shiftUses()
	g.stringsUses()
	g.durationUses()
	g.ifaceUses()
	g.errorUses()
	g.ptrUses()
	g.libUses()
	g.globalUses()
	b.WriteString("}\n")
}

// namedDecls writes the program's named types and their methods.
func (g *gen) namedDecls(b *strings.Builder) {
	k := g.k
	unders := []string{"int", "int64", "int32", "uint8", "int8", "int16", "uint", "uint16", "uint32", "uint64", "uintptr"}
	g.under = unders[g.nr.IntN(len(unders))]
	g.alias = g.nr.IntN(2) == 0
	fmt.Fprintf(b, "\ntype N%d %s\n", k, g.under)
	// String cannot panic, so fmt may call it (the subset requires that).
	fmt.Fprintf(b, "\nfunc (x N%d) String() string {\n\tif x&1 == 0 {\n\t\treturn fmt.Sprintf(\"e%%d\", int(x))\n\t}\n\treturn \"o\" + fmt.Sprint(int(x))\n}\n", k)
	fmt.Fprintf(b, "\nfunc (x N%d) Mix(y N%d) N%d {\n\treturn %s\n}\n", k, k, k, g.nexpr(3))
	fmt.Fprintf(b, "\ntype L%d []N%d\n", k, k)
	fmt.Fprintf(b, "\nfunc (l *L%d) Add(x N%d) { *l = append(*l, x) }\n", k, k)
	fmt.Fprintf(b, "\nfunc (l L%d) Total() N%d {\n\tt := N%d(0)\n\tfor _, x := range l {\n\t\tt += x\n\t}\n\treturn t\n}\n", k, k, k)
	if g.alias {
		fmt.Fprintf(b, "\ntype A%d = N%d\n", k, k)
	}
}

// width is the bit width of the named integer type.
func (g *gen) width() int {
	return map[string]int{"int": 64, "int64": 64, "int32": 32, "uint8": 8, "int8": 8, "int16": 16,
		"uint": 64, "uint16": 16, "uint32": 32, "uint64": 64, "uintptr": 64}[g.under]
}

// nexpr is an expression of the named type over x and y, of at most the
// given depth.
func (g *gen) nexpr(depth int) string {
	leaf := []string{"x", "y"}[g.nr.IntN(2)]
	if depth == 0 {
		return leaf
	}
	x := func() string { return g.nexpr(depth - 1) }
	switch g.nr.IntN(6) {
	case 0:
		op := []string{"+", "-", "*", "&", "|", "^", "&^"}[g.nr.IntN(7)]
		return fmt.Sprintf("(%s %s %s)", x(), op, x())
	case 1:
		op := []string{"/", "%"}[g.nr.IntN(2)]
		return fmt.Sprintf("(%s %s (%s | 1))", x(), op, x())
	case 2:
		op := []string{"<<", ">>"}[g.nr.IntN(2)]
		return fmt.Sprintf("(%s %s %d)", x(), op, g.nr.IntN(g.width()+4))
	case 3:
		return fmt.Sprintf("(-%s)", x())
	case 4:
		return fmt.Sprintf("(%s + %d)", x(), g.nr.IntN(9))
	}
	return leaf
}

// globalDecls writes program k's package-level variables in lib.
func (g *gen) globalDecls(b *strings.Builder) {
	k := g.k
	var tab []string
	for range 1 + g.gr.IntN(6) {
		tab = append(tab, fmt.Sprint(g.gr.Int64N(2001)-1000))
	}
	fmt.Fprintf(b, "\nvar C%d = %d\n", k, g.gr.IntN(100))
	fmt.Fprintf(b, "\nvar T%d = []int{%s}\n", k, strings.Join(tab, ", "))
	fmt.Fprintf(b, "\nvar S%d = %q\n", k, []string{"", "s", "go"}[g.gr.IntN(3)])
	fmt.Fprintf(b, "\nvar Q%d P%d\n", k, k)
	// Variables nothing changes (statics): read as call arguments, an
	// operand, a field's base and a method's receiver.
	fmt.Fprintf(b, "\nvar D%d = %d\n", k, g.gr.IntN(1000)-500)
	fmt.Fprintf(b, "\nvar O%d = P%d{A: %d, B: %d}\n", k, k, g.gr.IntN(100), g.gr.IntN(100))
	// A variable nothing uses, whose initializer prints: Go initializes it
	// all the same, before main.
	fmt.Fprintf(b, "\nfunc say%d(s string) int {\n\tfmt.Println(\"init\", s)\n\treturn len(s)\n}\n", k)
	fmt.Fprintf(b, "\nvar Said%d = say%d(\"lib%d\")\n", k, k, k)
	fmt.Fprintf(b, "\nfunc Bump%d(x int) int {\n\tC%d += x\n\treturn C%d\n}\n", k, k, k)
	fmt.Fprintf(b, "\nfunc Look%d(i int) int { return T%d[(i%%len(T%d)+len(T%d))%%len(T%d)] }\n", k, k, k, k, k)
	// A return that reads C and calls what changes it: gc calls first.
	fmt.Fprintf(b, "\nfunc Next%d() int { return C%d + Bump%d(1) }\n", k, k, k)
	// A return of two values, one reading C, the other changing it: gc
	// makes the call first here too.
	fmt.Fprintf(b, "\nfunc Pair%d() (int, int) { return C%d, Bump%d(1) }\n", k, k, k)
}

// globalUses reads and changes program k's package-level variables.
func (g *gen) globalUses() {
	k := g.k
	v := func() string { return fmt.Sprintf("v%d", g.gr.IntN(3)) }
	g.line("fmt.Println(lib.C%d, lib.Bump%d(%s), lib.C%d+lib.Bump%d(%s), lib.Look%d(%s), len(lib.T%d), lib.T%d)", k, k, v(), k, k, v(), k, v(), k, k)
	g.line("lib.C%d += lib.Bump%d(%d)", k, k, g.gr.IntN(9))
	g.line("lib.S%d += fmt.Sprint(%s)", k, v())
	g.line("lib.Q%d.A = lib.Bump%d(%s)", k, k, v())
	g.line("lib.Q%d.B++", k)
	g.line("M%d -= uint8(%s)", k, v())
	g.line("fmt.Println(lib.C%d, lib.S%d, len(lib.S%d), lib.Q%d.A, lib.Q%d.B, M%d, lib.Q%d.Sum(), lib.Next%d())", k, k, k, k, k, k, k, k)
	g.line("fmt.Println(lib.Look%d(lib.D%d), -lib.D%d, lib.D%d*%s, lib.O%d.A, lib.O%d.Sum())", k, k, k, k, v(), k, k)
	g.line("gpa, gpb := lib.Pair%d()", k)
	g.line("fmt.Println(gpa, gpb)")
}

// libUses uses program k's declarations in lib.
func (g *gen) libUses() {
	k := g.k
	// A struct declared with no value and its fields set at once: the
	// initializer gives them (clippy's field_reassign_with_default), and
	// where a value reads the variable, the zeros are spelled out.
	g.line("var lz lib.P%d", k)
	g.line("lz.A = v2")
	g.line("lz.B = v0 + 1")
	g.line("var lw lib.P%d", k)
	g.line("lw.A = lw.B + v1")
	g.line("fmt.Println(lz.Sum(), lw.Sum())")
	g.line("lp := lib.P%d{A: v0, B: v1}", k)
	g.line("lp.Grow(v2)")
	g.line("fmt.Println(lp.Sum(), lp.A, lib.F%d(v0, v1), lib.K%d, lib.N%d(v2), []lib.N%d{lib.N%d(v1)})", k, k, k, k, k)
	g.line("lg, lgerr := lib.G%d(v0)", k)
	g.line("if lgerr != nil {")
	g.depth++
	g.line("fmt.Println(\"lib error:\", lgerr)")
	g.depth--
	g.line("}")
	g.line("ldv, lderr := lib.Dbl%d(v0)", k)
	g.line("if lderr != nil {")
	g.depth++
	g.line("fmt.Println(\"dbl error:\", lderr)")
	g.depth--
	g.line("}")
	g.line("fmt.Println(ldv)")
	g.line("_, lgerr2 := lib.G%d(v1)", k)
	g.line("fmt.Println(lg, lgerr2, lgerr2 == lib.ES%d, errors.Is(lgerr2, lib.ES%d))", k, k)
	g.line("var ll *lib.Lst%d", k)
	g.line("for i := 0; i < 3; i++ {")
	g.depth++
	g.line("ll = lib.Cons%d(v0+i, ll)", k)
	g.depth--
	g.line("}")
	g.line("ll.Bump(v1)")
	// A field's address lent to a call, as &mut ll.v, its other argument
	// read through the same list first (Go evaluates it before the call).
	g.line("lib.AddTo%d(&ll.T, ll.Next.V)", k)
	g.line("fmt.Println(ll.T, ll.Next.T)")
	g.line("fmt.Println(ll.Sum(), ll.V, ll.Next.Next.V, ll.Next.Next.Next == nil)")
	// A walk to a stop pointer, compared by address (std::ptr::eq), and a
	// new pointer moved into a Vec's element.
	g.line("lstop := ll.Next.Next")
	g.line("lsteps := 0")
	g.line("for p := ll; p != lstop; p = p.Next {")
	g.depth++
	g.line("lsteps += p.V")
	g.depth--
	g.line("}")
	g.line("lv := make([]*lib.Lst%d, 2)", k)
	g.line("lv[0] = lib.Cons%d(v2, nil)", k)
	g.line("fmt.Println(lsteps, lv[0].V, lv[1] == nil, lv[0] == lstop)")
}

// shiftUses shifts a value of a random integer type by run-time counts.
func (g *gen) shiftUses() {
	ints := []string{"int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64"}
	v := func() string { return fmt.Sprintf("v%d", g.sr.IntN(3)) }
	g.line("sa := %s(%s)", ints[g.sr.IntN(len(ints))], v())
	g.line("sc := %s(%s & 127)", []string{"int", "uint8", "uint64", "int32", "uint"}[g.sr.IntN(5)], v())
	g.line("fmt.Println(sa<<sc, sa>>sc, sa<<(sc&7), sa>>(sc%%70), -sa>>sc)")
	g.line("sa <<= sc & %d", 1+g.sr.IntN(70))
	g.line("sa >>= sc / 3")
	g.line("fmt.Println(sa)")
}

// words are strings where Go's case mapping, folding and splitting differ
// from Rust's standard library, or are easy to get wrong.
var words = []string{
	"", "a", "Go", "ΟΔΟΣ", "İstanbul", "straße", "ẞ", "\u212a", "ſ", "ǅ", "ﬀ", "σς",
	"a,b,,c", "--", "héllo", "ÅngStröm", "x=y=z", " pad ",
}

// stringsUses builds a string from the program's values and calls the
// provided strings API on it (RFC 0011).
func (g *gen) stringsUses() {
	w := func() string { return fmt.Sprintf("%q", words[g.xr.IntN(len(words))]) }
	g.line("cw := %s + fmt.Sprint(v%d) + %s + str", w(), g.xr.IntN(3), w())
	g.line("fmt.Println(strings.ToLower(cw), strings.ToUpper(cw), strings.EqualFold(cw, strings.ToUpper(cw)), strings.EqualFold(cw, %s))", w())
	g.line("cp := strings.Split(cw, %s)", w())
	g.line("cb, ca, cok := strings.Cut(cw, %s)", w())
	g.line("fmt.Println(len(cp), cp, cb, ca, cok, strings.TrimPrefix(cw, %s), strings.TrimSuffix(cw, %s))", w(), w())
	g.line("fmt.Println(strings.ReplaceAll(cw, %s, %s), strings.IndexByte(cw, %q))", w(), w(), "a-=,1"[g.xr.IntN(5)])
	g.line("fmt.Println(cw, K%d(cw), cw+K%d(cw))", g.k, g.k)
}

// durationUses makes time.Duration values from the program's integers, in
// every unit (far past the int64 range, so they wrap), and calls every
// method patina mode provides (RFC 0011) and prints them.
func (g *gen) durationUses() {
	units := []string{"time.Nanosecond", "time.Microsecond", "time.Millisecond", "time.Second", "time.Minute", "time.Hour", "1499", "-7"}
	unit := func() string { return units[g.dr.IntN(len(units))] }
	g.line("dd := time.Duration(v%d) * %s", g.dr.IntN(3), unit())
	g.line("de := time.Duration(v%d%%%d) * %s", g.dr.IntN(3), []int{2, 1000, 1000003}[g.dr.IntN(3)], unit())
	g.line("fmt.Println(dd, de, dd+de, dd.Round(de), dd.Truncate(de), de.Round(%s), dd.Abs(), de.Abs())", unit())
	g.line("fmt.Println(dd.Nanoseconds(), dd.Microseconds(), de.Milliseconds(), dd.Seconds(), de.Minutes(), dd.Hours())")
	g.line("fmt.Printf(\"%%v|%%%dv|%%-%dv|%%d|%%s\\n\", de, dd, de, dd, []time.Duration{dd, de, dd / 3})", 1+g.dr.IntN(20), 1+g.dr.IntN(20))
}

// ifaceDecls writes the program's interface I and the two types that
// implement it: a struct R and a named integer U (RFC 0012).
func (g *gen) ifaceDecls(b *strings.Builder) {
	k := g.k
	op := func() string { return []string{"+", "-", "*", "^", "|", "&"}[g.ir.IntN(6)] }
	fmt.Fprintf(b, "\ntype I%d interface {\n\tGet(x int) int\n\tName() string\n}\n", k)
	fmt.Fprintf(b, "\ntype R%d struct {\n\tA int\n\tB string\n}\n", k)
	fmt.Fprintf(b, "\nfunc (r R%d) Get(x int) int { return (r.A %s x) %s %d }\n", k, op(), op(), g.ir.IntN(100))
	fmt.Fprintf(b, "\nfunc (r R%d) Name() string { return r.B + fmt.Sprint(r.A) }\n", k)
	fmt.Fprintf(b, "\ntype U%d int\n", k)
	fmt.Fprintf(b, "\nfunc (u U%d) Get(x int) int { return int(u) %s x }\n", k, op())
	fmt.Fprintf(b, "\nfunc (u U%d) Name() string { return fmt.Sprintf(\"u%%d\", int(u)) }\n", k)
}

// ifaceUses converts the program's values to I and to any, calls through
// them, switches on and asserts their dynamic types, compares them and
// prints them with %v and %T (RFC 0012).
func (g *gen) ifaceUses() {
	k := g.k
	v := func() string { return fmt.Sprintf("v%d", g.ir.IntN(3)) }
	word := func() string { return fmt.Sprintf("%q", words[g.ir.IntN(len(words))]) }
	r := func() string { return fmt.Sprintf("R%d{A: %s, B: %s}", k, v(), word()) }
	u := func() string { return fmt.Sprintf("U%d(%s)", k, v()) }
	g.line("ifs := []I%d{%s, %s}", k, r(), u())
	g.line("ifs = append(ifs, %s)", []string{r(), u()}[g.ir.IntN(2)])
	anys := []string{v(), word(), r(), u(), "nil", "float64(" + v() + ") / 3", fmt.Sprintf("N%d(%s)", k, v())}
	g.line("var ify any = %s", anys[g.ir.IntN(len(anys))])
	g.line("for _, ifv := range ifs {")
	g.depth++
	g.line("fmt.Println(ifv.Get(%s), ifv.Name(), ifv)", v())
	g.depth--
	g.line("}")
	g.line("switch ifw := ify.(type) {")
	g.line("case nil:")
	g.depth++
	g.line("fmt.Println(\"nil\", ifw == nil)")
	g.depth--
	g.line("case int:")
	g.depth++
	g.line("fmt.Println(\"int\", ifw)")
	g.depth--
	g.line("case I%d:", k)
	g.depth++
	g.line("fmt.Println(\"i\", ifw.Get(%s), ifw.Name())", v())
	g.depth--
	g.line("case string, float64:")
	g.depth++
	g.line("fmt.Println(\"sf\", ifw)")
	g.depth--
	g.line("default:")
	g.depth++
	g.line("fmt.Printf(\"%%v %%T\\n\", ifw, ifw)")
	g.depth--
	g.line("}")
	g.line("ifr, ifok := ify.(R%d)", k)
	g.line("ifi, ifok2 := ify.(I%d)", k)
	g.line("fmt.Println(ifr.A, ifr.B, ifok, ifok2, ifi == nil, ifs[0] == ifs[1], ify == any(%s), ify != nil, ifs[2] == I%d(%s))", v(), k, []string{r(), u()}[g.ir.IntN(2)])
	g.line("fmt.Printf(\"%%v|%%T|%%v|%%T\\n\", ify, ify, ifs[len(ifs)-1], ifs[0])")
}

// errorDecls writes the program's error type E and a function F that
// returns a value and an error: E, one made by errors.New, or nil
// (RFC 0012).
func (g *gen) errorDecls(b *strings.Builder) {
	k := g.k
	m := 2 + g.er.IntN(4)
	fmt.Fprintf(b, "\ntype E%d struct{ C int }\n", k)
	fmt.Fprintf(b, "\nfunc (e E%d) Error() string { return fmt.Sprint(\"e\", e.C) }\n", k)
	fmt.Fprintf(b, "\nfunc F%d(x int) (int, error) {\n\tswitch x %% %d {\n\tcase 0:\n\t\treturn 0, E%d{C: x}\n\tcase 1, -1:\n\t\treturn 0, errors.New(fmt.Sprint(\"n\", x))\n\t}\n\treturn x ^ %d, nil\n}\n", k, m, k, g.er.IntN(1000))
}

// errorUses keeps the program's errors as values: in a slice, compared,
// switched on, asserted and printed (RFC 0012).
func (g *gen) errorUses() {
	k := g.k
	v := func() string { return fmt.Sprintf("v%d", g.er.IntN(3)) }
	g.line("ev, eerr := F%d(%s)", k, v())
	g.line("fmt.Println(ev, eerr, eerr == nil)")
	g.line("_, eerr2 := F%d(%s)", k, v())
	g.line("eerrs := []error{eerr, eerr2, nil, eerr}")
	g.line("for _, ee := range eerrs {")
	g.depth++
	g.line("switch ex := ee.(type) {")
	g.line("case nil:")
	g.depth++
	g.line("fmt.Println(\"none\")")
	g.depth--
	g.line("case E%d:", k)
	g.depth++
	g.line("fmt.Println(\"e\", ex.C, ex)")
	g.depth--
	g.line("default:")
	g.depth++
	g.line("fmt.Println(\"other\", ex, ex.Error())")
	g.depth--
	g.line("}")
	g.depth--
	g.line("}")
	g.line("ec, eok := eerr2.(E%d)", k)
	g.line("fmt.Println(ec.C, eok, eerrs[0] == eerrs[3], eerrs[0] == eerrs[1], eerrs[2] == nil)")
	g.line("fmt.Printf(\"%%v|%%s|%%T|%%%dv|\\n\", eerr, eerr2, eerrs[2], eerr)", 1+g.er.IntN(15))
	// The errors API: %w, Unwrap, Is and As through a chain.
	g.line("ew := fmt.Errorf(\"w%d: %%w\", eerr)", g.er.IntN(100))
	g.line("eww := fmt.Errorf(\"ww: %%w\", ew)")
	g.line("var eas E%d", k)
	g.line("fmt.Println(ew, eww, errors.Unwrap(eww) == ew, errors.Is(eww, eerr), errors.Is(eww, eerr2), errors.As(eww, &eas), eas.C)")
}

// printUses prints the program's values with fmt's widths and flags.
func (g *gen) printUses() {
	width := func() string {
		w := ""
		if g.pr.IntN(3) == 0 {
			w = "-"
		}
		return w + fmt.Sprint(1+g.pr.IntN(8)) // %0v would be the 0 flag
	}
	hex := func() string {
		flags := []string{"", "#", "0", "#0", "-#", "-0", "-"}[g.pr.IntN(7)]
		w := ""
		if g.pr.IntN(3) > 0 {
			w = fmt.Sprint(1 + g.pr.IntN(20))
		}
		return "%" + flags + w + []string{"x", "X"}[g.pr.IntN(2)]
	}
	v := fmt.Sprintf("v%d", g.pr.IntN(3))
	format := "|%" + width() + "v|%" + width() + "v|" + hex() + "|" + hex() + "|" + hex() + "|" + hex() + "|\n"
	g.line("fmt.Printf(%q, s, l, %s, n, str, []byte(str))", format, v)
}

// floatUses writes float64 values made from the program's integers, some
// far outside every integer range, NaN and the infinities, and prints their
// conversions to int, int32 and uint8.
func (g *gen) floatUses() {
	v := func() string { return fmt.Sprintf("v%d", g.fr.IntN(3)) }
	scale := []string{"1", "0.37", "1e3", "3.3e9", "1e10", "9.3e18", "1e19", "1e300", "-1e300"}
	g.line("fz := float64(%s - %s)", v(), v())
	g.line("fz = fz - fz")
	g.line("fs := []float64{float64(%s) * %s, float64(%s) / float64(%s|1) * %s, fz / fz, 1 / fz, -1 / fz}",
		v(), scale[g.fr.IntN(len(scale))], v(), v(), scale[g.fr.IntN(len(scale))])
	g.line("for _, f := range fs {")
	g.depth++
	g.line("fmt.Println(f, int(f), int32(f), uint8(f), int(f*%s))", scale[g.fr.IntN(len(scale))])
	g.depth--
	g.line("}")
	// float32 (RFC 0008), drawn after the float64 part, which is unchanged.
	scale32 := []string{"1", "0.37", "1e3", "3e9", "1e-3", "-7.5"}
	g.line("f32 := float32(fs[%d]) * %s", g.fr.IntN(2), scale32[g.fr.IntN(len(scale32))])
	g.line("fmt.Println(f32, f32*f32, f32/3, -f32, f32+0.1, float64(f32), []float32{f32, 0.1})")
	g.line("fmt.Println(int(f32), int8(f32), int16(f32), uint16(f32), uint32(f32), uint64(f32), uint(fs[0]), uint32(fs[1]), uint64(fs[%d]))", 2+g.fr.IntN(3))
}

// namedUses converts the program's values to its named type, calls the
// methods and prints the results, with String and without.
func (g *gen) namedUses() {
	k := g.k
	v := func() string { return fmt.Sprintf("v%d", g.nr.IntN(3)) }
	op := []string{"+", "-", "*", "^", "&", "|"}[g.nr.IntN(6)]
	g.line("n := N%d(%s %s %s)", k, v(), op, v())
	g.line("m := n.Mix(N%d(%s))", k, v())
	g.line("var l L%d", k)
	g.line("l.Add(n)")
	g.line("l.Add(m)")
	g.line("pl := &l")
	g.line("pl.Add(m.Mix(n))")
	if g.alias {
		g.line("var a A%d = m", k)
		g.line("m = a ^ n")
	}
	g.line("fmt.Println(n, m, int(m), len(l), l.Total(), l[0] < l[1], l[2] == m)")
	g.line(`fmt.Printf("%%v|%%d|%%5s|%%-4v|\n", m, m, l[2], n)`)
}

func (g *gen) stmt() {
	v := g.vars[g.r.IntN(len(g.vars))]
	nested := g.depth < 2
	switch c := g.r.IntN(11); {
	case c == 8 && g.depth == 0:
		g.nest()
	case c == 0 && nested:
		g.line("if %s {", g.cond())
		g.block()
		if g.r.IntN(2) == 0 {
			g.line("} else {")
			g.block()
		}
		g.line("}")
	case c == 1 && nested:
		i := fmt.Sprintf("i%d", g.loops)
		g.loops++
		g.line("for %s := 0; %s < %d; %s++ {", i, i, 1+g.r.IntN(4), i)
		g.reads = append(g.reads, i)
		g.block()
		g.reads = g.reads[:len(g.reads)-1]
		g.line("}")
	case c == 2 && nested:
		g.line("switch %s %% 3 {", v)
		g.line("case 0:")
		g.block()
		g.line("case 1, -1:")
		g.block()
		g.line("default:")
		g.block()
		g.line("}")
	case c == 3:
		g.line("s[%s] = %s", g.index(1), g.expr(2, true))
	case c == 4:
		g.line("t = append(t, %s)", g.expr(2, true))
	case c == 5:
		verb := []string{"%d", "%x", "%v", "%5d", "%-4d|", "%+d", "%08d", "%X"}[g.r.IntN(8)]
		g.line("str += fmt.Sprintf(%q, %s)", verb+",", g.expr(1, true))
	case c == 6:
		g.line("fmt.Println(%s, %s)", g.expr(2, true), g.cond())
	case c == 7:
		op := []string{"+=", "-=", "*=", "|=", "&=", "^=", "<<=", ">>="}[g.r.IntN(8)]
		rhs := g.expr(2, true)
		if op == "<<=" || op == ">>=" {
			rhs = fmt.Sprint(g.r.IntN(64))
		}
		g.line("%s %s %s", v, op, rhs)
	default:
		g.line("%s = %s", v, g.expr(3, true))
	}
}

func (g *gen) block() {
	g.depth++
	for range 1 + g.r.IntN(3) {
		g.stmt()
	}
	g.depth--
}

// index is an expression in [0, len(s)) of at most the given depth.
func (g *gen) index(depth int) string {
	return fmt.Sprintf("(%s%%%d+%d)%%%d", g.expr(depth, true), g.sl, g.sl, g.sl)
}

func (g *gen) cond() string {
	switch g.r.IntN(4) {
	case 0:
		return fmt.Sprintf("!(%s)", g.cmp())
	case 1:
		return fmt.Sprintf("%s && %s", g.cmp(), g.cmp())
	case 2:
		return fmt.Sprintf("%s || %s", g.cmp(), g.cmp())
	default:
		return g.cmp()
	}
}

func (g *gen) cmp() string {
	op := []string{"<", "<=", ">", ">=", "==", "!="}[g.r.IntN(6)]
	return fmt.Sprintf("%s %s %s", g.expr(1, true), op, g.expr(1, true))
}

// expr is an int expression of at most the given depth; local says whether
// the slice s, the slice t and the helper are in scope.
func (g *gen) expr(depth int, local bool) string {
	all := g.vars
	if local {
		all = append(append([]string(nil), g.vars...), g.reads...)
	}
	leaf := all[g.r.IntN(len(all))]
	if depth == 0 {
		return leaf
	}
	x := func() string { return g.expr(depth-1, local) }
	switch c := g.r.IntN(12); {
	case c == 0:
		return leaf
	case c == 1:
		op := []string{"+", "-", "*", "&", "|", "^", "&^"}[g.r.IntN(7)]
		return fmt.Sprintf("(%s %s %s)", x(), op, x())
	case c == 2:
		op := []string{"/", "%"}[g.r.IntN(2)]
		return fmt.Sprintf("(%s %s (%s | 1))", x(), op, x())
	case c == 3:
		op := []string{"<<", ">>"}[g.r.IntN(2)]
		return fmt.Sprintf("(%s %s %d)", x(), op, g.r.IntN(64))
	case c == 4:
		return fmt.Sprintf("(-%s)", x())
	case c == 5:
		return fmt.Sprintf("min(%s, %s)", x(), x())
	case c == 6:
		return fmt.Sprintf("max(%s, %s)", x(), x())
	case c == 7 && local:
		return fmt.Sprintf("h%d(%s, %s)", g.k, x(), x())
	case c == 8 && local:
		return fmt.Sprintf("s[%s]", g.index(depth-1))
	case c == 9 && local:
		return "len(t)"
	case c == 10:
		return fmt.Sprintf("(%s ^ %s)", leaf, x())
	default:
		return fmt.Sprintf("(%s + %s)", x(), leaf)
	}
}

// panicking writes the panic case's failing statement, chosen at random;
// one choice in twenty-seven panics nowhere. Values are computed at run
// time (z is 0 but not a constant), so the Go compiler cannot reject them.
func (g *gen) panicking() {
	g.line("z := %s - %s", g.vars[0], g.vars[0])
	k := g.r.IntN(5)
	switch g.r.IntN(9) {
	case 0:
		g.line("fmt.Println(s[len(s)+z+%d])", k)
	case 1:
		g.line("fmt.Println(s[z-%d])", k+1)
	case 2:
		op := []string{"/", "%"}[g.r.IntN(2)]
		g.line("fmt.Println(%s %s z)", g.vars[1], op)
	case 3:
		g.line("u := make([]int, z-%d)", k+1)
		g.line("fmt.Println(len(u))")
	case 4:
		g.line("u := make([]int, len(s)+%d, len(s)+z)", k+1)
		g.line("fmt.Println(len(u))")
	case 5:
		g.line(`panic("generated panic " + fmt.Sprint(%s+z))`, g.vars[1])
	case 6:
		g.line("fmt.Println(str[len(str)+z+%d])", k)
	case 7:
		g.cube("2097153 + z", "bool")
	default:
		switch g.qr.IntN(3) {
		case 0:
			// A nil pointer dereferenced (RFC 0013).
			g.line("var ptrZ *Nd%d", g.k)
			g.line("fmt.Println(z, ptrZ.V)")
		case 1:
			// A nil owned pointer (RFC 0016) read before a call that
			// prints, which gc makes first.
			g.line("var trZ *Tr%d", g.k)
			g.line("fmt.Println(z, trZ.V+trSay%d(\"before the panic\"))", g.k)
		default:
			g.line("fmt.Println(z)")
		}
	}
}

// nest writes a loop nest of the kind RFC 0006 specializes, over a fresh
// slice whose length is held in a variable that never changes, so every
// index stays in range.
func (g *gen) nest() {
	k := g.loops
	g.loops++
	m, w, a, b := fmt.Sprintf("m%d", k), fmt.Sprintf("w%d", k), fmt.Sprintf("a%d", k), fmt.Sprintf("b%d", k)
	if g.r.IntN(64) == 0 {
		g.cube("2097152", "int")
		return
	}
	g.line("%s := %d", m, 1+g.r.IntN(40))
	g.line("%s := make([]int, %s)", w, m)
	switch g.r.IntN(3) {
	case 0:
		g.line("for %s := 0; %s < %s; %s++ {", a, a, m, a)
		g.reads = append(g.reads, a)
		g.depth++
		g.line("%s[%s] += %s", w, a, g.expr(1, true))
		g.castOps(w, a, a)
		g.depth--
		g.block()
		g.reads = g.reads[:len(g.reads)-1]
		g.line("}")
	default:
		start := a + " * " + a
		if g.r.IntN(2) == 0 {
			start += " * " + a
		}
		g.line("for %s := 1; %s < %s; %s++ {", a, a, m, a)
		g.reads = append(g.reads, a)
		g.depth++
		g.line("for %s := %s; %s < %s; %s += %s {", b, start, b, m, b, a)
		g.reads = append(g.reads, b)
		g.depth++
		g.line("%s[%s] += %s", w, b, g.expr(1, true))
		g.castOps(w, b, a)
		g.depth--
		g.reads = g.reads[:len(g.reads)-1]
		g.line("}")
		g.depth--
		g.reads = g.reads[:len(g.reads)-1]
		g.line("}")
	}
	g.line("fmt.Println(%s[0], %s[%s-1], len(%s))", w, w, m, w)
}

// cube writes the cube nest over make([]T, m) for m = size: a*a*a fits in
// an int for a <= 2097151, so it is specialized for m <= 2097152 and kept
// general above.
func (g *gen) cube(size, elem string) {
	k := g.loops
	g.loops++
	m, w, a, b := fmt.Sprintf("m%d", k), fmt.Sprintf("w%d", k), fmt.Sprintf("a%d", k), fmt.Sprintf("b%d", k)
	g.line("%s := %s", m, size)
	g.line("%s := make([]%s, %s)", w, elem, m)
	g.line("for %s := 1; %s < %s; %s++ {", a, a, m, a)
	g.line("\tfor %s := %s * %s * %s; %s < %s; %s += %s {", b, a, a, a, b, m, b, a)
	if elem == "bool" {
		g.line("\t\t%s[%s] = true", w, b)
	} else {
		g.line("\t\t%s[%s]++", w, b)
	}
	g.line("\t}")
	g.line("}")
	g.line("fmt.Println(len(%s))", w)
}

// castOps writes a shift and a comparison whose left operand is a product
// of loop variables. In a specialized nest those are usize values used as
// Go ints, written x as i64 * y as i64, which Rust misreads before << or <
// unless the emitter adds parentheses.
func (g *gen) castOps(w, x, y string) {
	g.line("%s[%s] ^= %s * %s << %d", w, x, x, y, g.r.IntN(40))
	g.line("if %s*%s < %s {", x, y, g.vars[g.r.IntN(len(g.vars))])
	g.line("\t%s[%s]++", w, x)
	g.line("}")
}

// ptrDecls writes the program's types with pointers that escape (RFC
// 0013): a list node built by a constructor, with a counter held by value
// (Cn), and a counter whose address escapes from a local (Ct). They are the
// program's own types: a pointer type escaping in one program would make
// every program's pointers of that type heap pointers.
func (g *gen) ptrDecls(b *strings.Builder) {
	k := g.k
	op := func() string { return []string{"+", "-", "*", "^", "|", "&"}[g.qr.IntN(6)] }
	fmt.Fprintf(b, "\ntype Ct%d struct{ n int }\n", k)
	fmt.Fprintf(b, "\nfunc (c *Ct%d) Bump(x int) int {\n\tc.n = c.n %s x\n\treturn c.n\n}\n", k, op())
	fmt.Fprintf(b, "\nfunc (c Ct%d) Get() int { return c.n %s %d }\n", k, op(), g.qr.IntN(100))
	// Me lends its receiver back: compared with its owner, by address.
	fmt.Fprintf(b, "\nfunc (c *Ct%d) Me() *Ct%d { return c }\n", k, k)
	// Cn is held by value in Nd: its pointers never escape, so x.in.M()
	// lends it inside its node's cell.
	fmt.Fprintf(b, "\ntype Cn%d struct{ n int }\n", k)
	fmt.Fprintf(b, "\nfunc (c *Cn%d) Bump(x int) int {\n\tc.n = c.n %s x\n\treturn c.n\n}\n", k, op())
	fmt.Fprintf(b, "\nfunc (c Cn%d) Get() int { return c.n %s %d }\n", k, op(), g.qr.IntN(100))
	// A string parameter whose address escapes: a cell of the string it
	// borrows.
	fmt.Fprintf(b, "\ntype Lb%d string\n", k)
	fmt.Fprintf(b, "\nfunc lab%d(s Lb%d) *Lb%d { return &s }\n", k, k, k)
	fmt.Fprintf(b, "\nfunc cell%d(x int) *Ct%d {\n\tc := Ct%d{n: x}\n\tc.n++\n\treturn &c\n}\n", k, k, k)
	fmt.Fprintf(b, "\ntype Nd%d struct {\n\tV     int\n\tName  string\n\tItems []int\n\tNext  *Nd%d\n\tKids  []*Nd%d\n\tin    Cn%d\n}\n", k, k, k, k)
	fmt.Fprintf(b, "\nfunc newNd%d(v int, next *Nd%d) *Nd%d {\n\treturn &Nd%d{V: v, Next: next, Name: \"n\" + fmt.Sprint(v)}\n}\n", k, k, k, k)
	// A new pointer named before it is returned: the crate returns it
	// directly (clippy's let_and_return).
	fmt.Fprintf(b, "\nfunc lone%d(v int) *Nd%d {\n\tn := &Nd%d{V: v, Name: \"lone\"}\n\treturn n\n}\n", k, k, k)
	fmt.Fprintf(b, "\nfunc (n *Nd%d) Push(x int) { n.Items = append(n.Items, x) }\n", k)
	// Printed through String; for a nil pointer it panics, which fmt prints
	// as <nil>.
	fmt.Fprintf(b, "\nfunc (n *Nd%d) String() string { return \"nd:\" + n.Name }\n", k)
	fmt.Fprintf(b, "\ntype Sz%d interface{ Sum() int }\n", k)
	fmt.Fprintf(b, "\nfunc (n *Nd%d) Sum() int {\n\tt := n.V\n\tfor i := 0; i < len(n.Items); i++ {\n\t\tt = t %s n.Items[i]\n\t}\n\treturn t\n}\n", k, op())
	fmt.Fprintf(b, "\nfunc ndLen%d(n *Nd%d) int {\n\tc := 0\n\tfor p := n; p != nil; p = p.Next {\n\t\tc++\n\t}\n\treturn c\n}\n", k, k)
	// A nil test of a field inside the node's cell, in a recursive method.
	fmt.Fprintf(b, "\nfunc (n *Nd%d) Depth() int {\n\tif n.Next == nil {\n\t\treturn 1\n\t}\n\treturn 1 + n.Next.Depth()\n}\n", k)
	// Owned pointers (RFC 0016): a tree built from new pointers and a chain
	// longer than the runtime's drop depth, then only read through, tested
	// for nil and lent, so each is a Box with one owner.
	fmt.Fprintf(b, "\ntype Tr%d struct {\n\tL, R *Tr%d\n\tV    int\n\tS    string\n\tK    []*Tr%d\n}\n", k, k, k)
	// A slice of owned pointers (RFC 0017, step 7): a Vec owning each,
	// read by range, whose variable borrows each element, and by index.
	fmt.Fprintf(b, "\nfunc trKids%d(t *Tr%d) int {\n\ts := len(t.K)\n\tfor _, c := range t.K {\n\t\ts += c.V\n\t}\n\tfor i := 0; i < len(t.K); i++ {\n\t\ts += trKids%d(t.K[i])\n\t}\n\treturn s\n}\n", k, k, k)
	fmt.Fprintf(b, "\nfunc mkTr%d(d, v int) *Tr%d {\n\tif d <= 0 {\n\t\treturn &Tr%d{V: v, S: fmt.Sprint(\"t\", v)}\n\t}\n\tt := &Tr%d{L: mkTr%d(d-1, v %s %d), R: mkTr%d(d-1, v+1), V: v}\n\treturn t\n}\n", k, k, k, k, k, op(), 1+g.qr.IntN(9), k)
	fmt.Fprintf(b, "\nfunc (t *Tr%d) Sum() int {\n\tif t == nil {\n\t\treturn 0\n\t}\n\treturn t.V %s t.L.Sum() + t.R.Sum()\n}\n", k, op())
	fmt.Fprintf(b, "\nfunc trDepth%d(t *Tr%d) int {\n\tif t.L == nil {\n\t\treturn 1\n\t}\n\treturn 1 + trDepth%d(t.L)\n}\n", k, k, k)
	// Pointers never nil (RFC 0017, step 2): a receiver and a parameter
	// read through first (&self, &T), a result always new (Box), and
	// nil tests that bind the pointer (if let Some), with && after.
	fmt.Fprintf(b, "\nfunc (t *Tr%d) Leaf() bool { return t.L == nil && t.R == nil }\n", k)
	fmt.Fprintf(b, "\nfunc trRight%d(t *Tr%d, x int) int {\n\tif t.R != nil && t.R.V > x {\n\t\treturn trRight%d(t.R, x) + 1\n\t}\n\tif t.L != nil {\n\t\treturn t.L.V %s x\n\t}\n\treturn t.V\n}\n", k, k, k, op())
	// Writes through owned pointers (RFC 0017, step 3): &mut self, &mut T
	// and Option<&mut T>, and an if let Some whose body changes the node.
	fmt.Fprintf(b, "\nfunc (t *Tr%d) Bump(d int) {\n\tt.V %s= d\n\tif t.L != nil {\n\t\tt.L.V++\n\t}\n}\n", k, op())
	fmt.Fprintf(b, "\nfunc trName%d(t *Tr%d, s string) {\n\tt.S += s\n}\n", k, k)
	// A change through a pointer that may be nil, not used after the if:
	// if let Some(t) = t in Rust, the Option<&mut T> moved in.
	fmt.Fprintf(b, "\nfunc trAddIf%d(t *Tr%d, x int) {\n\tif t != nil {\n\t\tt.V += x\n\t}\n}\n", k, k)
	fmt.Fprintf(b, "\nfunc trMaybe%d(t *Tr%d, d int) {\n\tif t == nil {\n\t\treturn\n\t}\n\tt.V -= d\n\ttrName%d(t, \"m\")\n}\n", k, k, k)
	// A kept parameter (RFC 0017, step 8): a search tree's insert keeps
	// its tree, given a child taken out of the field its result refills.
	fmt.Fprintf(b, "\nfunc trIns%d(t *Tr%d, v int) *Tr%d {\n\tif t == nil {\n\t\treturn &Tr%d{V: v}\n\t}\n\tif v < t.V {\n\t\tt.L = trIns%d(t.L, v)\n\t} else {\n\t\tt.R = trIns%d(t.R, v)\n\t}\n\treturn t\n}\n", k, k, k, k, k, k)
	fmt.Fprintf(b, "\nfunc trPair%d(v int) *Tr%d {\n\tp := &Tr%d{V: v, L: mkTr%d(0, v), R: &Tr%d{V: v + 1}}\n\treturn p\n}\n", k, k, k, k, k)
	// Vs is plain data the box owns (RFC 0017), changed in place.
	fmt.Fprintf(b, "\ntype Ch%d struct {\n\tV  int\n\tVs []int\n\tN  *Ch%d\n}\n", k, k)
	fmt.Fprintf(b, "\nfunc chain%d(n int) *Ch%d {\n\tif n == 0 {\n\t\treturn nil\n\t}\n\treturn &Ch%d{V: n, N: chain%d(n - 1)}\n}\n", k, k, k, k)
	// Moves (RFC 0017, step 4): a chain built in a loop, each new link
	// taking the chain, so Ch frees its links in a loop too.
	fmt.Fprintf(b, "\nfunc chPush%d(n int) *Ch%d {\n\tvar h *Ch%d\n\tfor i := 0; i < n; i++ {\n\t\tx := &Ch%d{V: i}\n\t\tx.Vs = append(x.Vs, i*2)\n\t\tx.N = h\n\t\th = x\n\t}\n\treturn h\n}\n", k, k, k, k)
	// Temporary borrows (RFC 0017, step 5): a walk of the chain, while
	// let Some(n) = p in Rust, with a second borrow kept.
	fmt.Fprintf(b, "\nfunc chSum%d(c *Ch%d) int {\n\ts := 0\n\tvar last *Ch%d\n\tfor p := c; p != nil; p = p.N {\n\t\tif p.V%%3 == 0 {\n\t\t\tcontinue\n\t\t}\n\t\ts += p.V + len(p.Vs)\n\t\tlast = p\n\t}\n\tif last != nil {\n\t\ts += last.V\n\t}\n\treturn s\n}\n", k, k, k)
	// A local value whose address is handed on (RFC 0017): moved into a new
	// box at the &, Box::new(x), since nothing uses it after.
	fmt.Fprintf(b, "\nfunc chCons%d(v int, rest *Ch%d) *Ch%d {\n\tvar c Ch%d\n\tc.N = rest\n\tc.V += v + 1\n\treturn &c\n}\n", k, k, k, k)
	// Value data behind an owned pointer (RFC 0017): copied out (*p, Copy
	// in Rust), replaced whole, and read by a value method.
	fmt.Fprintf(b, "\ntype Pt%d struct{ X, Y int }\n\nfunc (p Pt%d) Sum() int { return p.X*3 + p.Y }\n\nfunc mkPt%d(x int) *Pt%d { return &Pt%d{X: x, Y: x + 1} }\n", k, k, k, k, k)
	// A number behind a pointer with one owner (RFC 0017): the value
	// itself in Rust, i64 or Option<i64>, not a Box.
	fmt.Fprintf(b, "\nfunc intBox%d(v int) *int {\n\tw := v * 2\n\treturn &w\n}\n\nfunc intMaybe%d(v int) *int {\n\tif v < 0 {\n\t\treturn nil\n\t}\n\treturn &v\n}\n", k, k)
	// A local lent to calls that keep nothing, and a value parameter
	// returned by address (RFC 0017): &mut pt_l, pt_l.inc(), Box::new(p).
	fmt.Fprintf(b, "\nfunc ptAdd%d(p *Pt%d, x int) { p.X += x }\n\nfunc (p *Pt%d) Inc() { p.Y++ }\n\nfunc ptBox%d(p Pt%d) *Pt%d { return &p }\n", k, k, k, k, k, k)
	// A call that changes through an owned pointer, beside a read through
	// it: gc makes the call first, so Rust computes it into a local first.
	fmt.Fprintf(b, "\nfunc chInc%d(c *Ch%d) int {\n\tc.V += 7\n\treturn c.V\n}\n", k, k)
	// A returned borrow (RFC 0017, decision 4): the next link, lent back
	// from the argument, Option<&Ch> in Rust.
	fmt.Fprintf(b, "\nfunc chNext%d(c *Ch%d) *Ch%d { return c.N }\n", k, k, k)
	// An exclusive borrow lent back: changed through, here and by the
	// caller, Option<&mut Ch> in Rust.
	fmt.Fprintf(b, "\nfunc chBumpNext%d(c *Ch%d) *Ch%d {\n\tc.V += 1\n\treturn c.N\n}\n", k, k, k)
	// Two inputs, the result lent back from the first: Rust names its
	// lifetime, fn ch_pick<'a>(a: &'a Ch, b: &Ch) -> Option<&'a Ch>.
	fmt.Fprintf(b, "\nfunc chPick%d(a, b *Ch%d) *Ch%d {\n\tif b.V > 1000 {\n\t\treturn nil\n\t}\n\treturn a.N\n}\n", k, k, k)
	// Reads of a place beside calls that change that same place, in one
	// expression: a slice element, a local through &x, a receiver and an
	// element's receiver. gc makes the calls first (the spec leaves the
	// order unspecified), so Rust makes them first too.
	fmt.Fprintf(b, "\nfunc ordFirst%d(s []int) int {\n\ts[0] += 100\n\treturn s[0]\n}\n\nfunc ordAdd%d(p *int) int {\n\t*p += 3\n\treturn *p\n}\n\nfunc (p *Pt%d) Next() int {\n\tp.X++\n\treturn p.X\n}\n\ntype Ord%d struct{ X int }\n\nfunc (o *Ord%d) Next() int {\n\to.X += 2\n\treturn o.X\n}\n", k, k, k, k, k)
	// A struct value owning a chain (RFC 0017): built by a literal, read
	// and changed through, and moved whole.
	fmt.Fprintf(b, "\ntype Hold%d struct {\n\tH *Ch%d\n\tN int\n}\n", k, k)
	fmt.Fprintf(b, "\nfunc chLen%d(c *Ch%d) int {\n\tif c == nil {\n\t\treturn 0\n\t}\n\treturn c.V %% 3 + chLen%d(c.N)\n}\n", k, k, k)
	// A call that prints, beside a read through a nil owned pointer: gc
	// makes the call first (panicking).
	fmt.Fprintf(b, "\nfunc trSay%d(s string) int {\n\tfmt.Println(\"say\", s)\n\treturn len(s)\n}\n", k)
	fmt.Fprintf(b, "\nfunc fill%d(s []int, x int) {\n\tfor i := range s {\n\t\ts[i] = s[i]%sx + i\n\t}\n}\n", k, op())
}

// ptrUses builds a list, aliases its nodes and a counter, and changes
// them through every copy of the pointers: fields, a slice field,
// methods, a counter lent inside its cell, comparisons and nil. Reads
// beside calls that change them hold GoPatina to gc's order.
func (g *gen) ptrUses() {
	k := g.k
	v := func() string { return fmt.Sprintf("v%d", g.qr.IntN(3)) }
	op := func() string { return []string{"+", "-", "*", "^"}[g.qr.IntN(4)] }
	g.line("var ptrL *Nd%d", k)
	g.line("for i := 0; i < %d; i++ {", 2+g.qr.IntN(3))
	g.depth++
	g.line("ptrL = newNd%d(i %s %s, ptrL)", k, op(), v())
	g.depth--
	g.line("}")
	g.line("ptrO := lone%d(%s)", k, v())
	g.line("ptrO.V %s= ptrL.V", op())
	g.line("fmt.Println(ptrO.V, ptrO.Name, ptrO.Next == nil)")
	g.line("ptrA := ptrL")
	g.line("ptrA.V %s= %s", op(), v())
	g.line("ptrL.Push(%s)", v())
	g.line("ptrA.Push(ptrL.V)")
	g.line("ptrL.Items[%d] %s= %s", g.qr.IntN(2), op(), v())
	g.line("fill%d(ptrL.Items, %s)", k, v())
	g.line("ptrL.in.Bump(%s)", v())
	g.line("ptrL.Name += ptrA.Name")
	g.line("ptrL.Next.V, ptrA.V = ptrA.V, ptrL.Next.V")
	g.line("fmt.Println(ptrL.V, ptrL.in.Bump(%s), ptrL.in.n, ptrA.Sum(), ptrL.Next.V, len(ptrA.Items), ptrL.Name)", v())
	g.line("ptrC := cell%d(%s)", k, v())
	g.line("ptrC.Bump(%s)", v())
	// Compared by address: a borrow of ptrC, and a borrow of another Ct.
	g.line("ptrD := ptrC.Me()")
	g.line("var ptrLc Ct%d", k)
	g.line("ptrLp := &ptrLc")
	g.line("ptrLp.Bump(%s)", v())
	g.line("fmt.Println(ptrD.n, ptrC == ptrD, ptrD != ptrC, ptrC == ptrLp, ptrLp == ptrD, ptrLp.n)")
	g.line("ptrLc.Bump(ptrC.n)")
	g.line("fmt.Println(ptrC.n, ptrC.Get(), ptrLc.n, ptrLc.Get())")
	g.line("ptrB := lab%d(Lb%d(%q))", k, k, words[g.qr.IntN(len(words))])
	g.line("*ptrB += Lb%d(ptrL.Name)", k)
	g.line("fmt.Println(*ptrB, len(*ptrB), ptrB == lab%d(*ptrB), ptrB != ptrB)", k)
	g.line("var ptrN *Nd%d", k)
	g.line("if %s%%2 == 0 {", v())
	g.depth++
	g.line("ptrN = ptrL.Next")
	g.depth--
	g.line("}")
	g.line("if ptrL.Next != nil && ptrA.Next.Next != nil {")
	g.depth++
	g.line("fmt.Println(ptrL.Depth(), ptrL.Next.Depth())")
	g.depth--
	g.line("}")
	g.line("fmt.Println(ndLen%d(ptrL), ptrN == nil, ptrN != ptrL, ptrA == ptrL, ptrL.Next.Next == nil, ptrL.in.Get())", k)
	g.line("trT := mkTr%d(%d, %s)", k, 1+g.qr.IntN(4), v())
	g.line("fmt.Println(trT.Sum(), trDepth%d(trT), trT.S, trT.L.V, trT.R == nil, nil != trT.L.L, len(trT.R.S))", k)
	g.line("trAddIf%d(trT, %s)", k, v())
	g.line("trAddIf%d(nil, %s)", k, v())
	g.line("trT = mkTr%d(0, %s)", k, v())
	g.line("fmt.Println(trT.Sum(), trT.L == nil, chLen%d(chain%d(%d)))", k, k, 1100+g.qr.IntN(900))
	g.line("chA := chPush%d(%d)", k, 1+g.qr.IntN(3000))
	g.line("chB := chA")
	g.line("chC := &Ch%d{V: %s, N: chB}", k, v())
	g.line("fmt.Println(chLen%d(chC), chC.V, chC.N.V)", k)
	// A local slice of owned pointers (RFC 0017): a Vec owning each.
	g.line("pt := mkPt%d(%s)", k, v())
	g.line("pc := *pt")
	g.line("pc.X %s= %s", op(), v())
	g.line("*pt = Pt%d{X: %s, Y: pc.Y}", k, v())
	g.line("fmt.Println(pt.Sum(), pc.Sum(), pt.X)")
	g.line("ib := intBox%d(%s)", k, v())
	g.line("*ib %s= %s", op(), v())
	g.line("im := intMaybe%d(%s)", k, v())
	g.line("if im != nil {")
	g.depth++
	g.line("*im += *ib")
	g.depth--
	g.line("}")
	g.line("fmt.Println(*ib, im == nil)")
	g.line("var ptL Pt%d", k)
	g.line("ptL.X = %s", v())
	g.line("ptAdd%d(&ptL, %s)", k, v())
	g.line("ptL.Inc()")
	g.line("ptB := ptBox%d(ptL)", k)
	g.line("ptB.Inc()")
	g.line("fmt.Println(ptL.X, ptL.Y, ptB.Sum())")
	g.line("chE := chCons%d(%s, chPush%d(%d))", k, v(), k, 1+g.qr.IntN(4))
	g.line("fmt.Println(chE.V, chLen%d(chE))", k)
	g.line("chS := []*Ch%d{chPush%d(%d), chPush%d(%d)}", k, k, 1+g.qr.IntN(4), k, 1+g.qr.IntN(4))
	g.line("chS = append(chS, chPush%d(%d))", k, 1+g.qr.IntN(4))
	g.line("chS[1].V %s= %s", op(), v())
	g.line("fmt.Println(len(chS), chLen%d(chS[0]), chS[1].V, chSum%d(chS[2]))", k, k)
	g.line("hold := Hold%d{H: chPush%d(%d), N: %s}", k, k, 1+g.qr.IntN(5), v())
	g.line("hold.H.V %s= hold.N", op())
	g.line("held := hold")
	g.line("fmt.Println(chLen%d(held.H), held.H.V, held.N)", k)
	g.line("chT := 0")
	g.line("for chP := chC; chP != nil; chP = chP.N {")
	g.depth++
	g.line("chT += chP.V %s %s", op(), v())
	g.depth--
	g.line("}")
	g.line("fmt.Println(chC.V+chInc%d(chC), chC.V)", k)
	g.line("chNx := chNext%d(chC)", k)
	g.line("fmt.Println(chLen%d(chNx), chNx != nil, chC.V)", k)
	g.line("fmt.Println(chLen%d(chPick%d(chC, chC)), chC.V)", k, k)
	g.line("chBn := chBumpNext%d(chC)", k)
	g.line("if chBn != nil {")
	g.depth++
	g.line("chBn.V %s= %s", op(), v())
	g.depth--
	g.line("}")
	g.line("fmt.Println(chC.V, chLen%d(chC))", k)
	g.line("ordS := []int{%s, %s}", v(), v())
	g.line("fmt.Println(ordS[0]%sordFirst%d(ordS), ordS[0])", op(), k)
	// A local borrow (RFC 0017): p := &x, changed through and lent, and x
	// read after p's last use: let p = &mut x in Rust.
	g.line("lbX := %s", v())
	g.line("lbP := &lbX")
	g.line("*lbP %s= %s", op(), v())
	g.line("fmt.Println(ordAdd%d(lbP), *lbP)", k)
	g.line("fmt.Println(*lbP, lbX) // the print reads *lbP into a local first")
	g.line("ordX := %s", v())
	g.line("fmt.Println(ordX%sordAdd%d(&ordX), ordX)", op(), k)
	g.line("var ordP Pt%d", k)
	g.line("fmt.Println(ordP.X%sordP.Next(), ordP.X)", op())
	g.line("ordE := []Ord%d{{X: %s}, {X: %s}}", k, v(), v())
	g.line("fmt.Println(ordE[1].X%sordE[1].Next(), ordE[1].X)", op())
	// A loop condition of that shape, and one with a post statement and a
	// continue: loop { let call = ..; if c { break; } .. } in Rust.
	g.line("ordN := 0")
	g.line("for ordN+ordAdd%d(&ordN) < %d {", k, 10+g.qr.IntN(30))
	g.depth++
	g.line("fmt.Println(\"ord loop\", ordN)")
	g.depth--
	g.line("}")
	g.line("ordT := 0")
	g.line("for ordI := 0; ordI+ordAdd%d(&ordI) < %d; ordI++ {", k, 10+g.qr.IntN(30))
	g.depth++
	g.line("if ordI%%2 == 0 {")
	g.depth++
	g.line("continue")
	g.depth--
	g.line("}")
	g.line("ordT += ordI")
	g.depth--
	g.line("}")
	g.line("fmt.Println(ordT)")
	// Cases of that shape: each case's calls in an else of the cases
	// before it, made when Go comes to it.
	g.line("switch {")
	g.line("case ordX+ordAdd%d(&ordX) > %s:", k, v())
	g.depth++
	g.line("fmt.Println(\"ord first\", ordX)")
	g.depth--
	g.line("case ordX+ordAdd%d(&ordX) > %s:", k, v())
	g.depth++
	g.line("fmt.Println(\"ord second\", ordX)")
	g.depth--
	g.line("}")
	g.line("switch ordX%sordAdd%d(&ordX) {", op(), k)
	g.line("case %s:", v())
	g.depth++
	g.line("fmt.Println(\"ord case\", ordX)")
	g.depth--
	g.line("default:")
	g.depth++
	g.line("fmt.Println(\"ord default\", ordX)")
	g.depth--
	g.line("}")
	g.line("if ordX%sordAdd%d(&ordX) > %s {", op(), k, v())
	g.depth++
	g.line("fmt.Println(\"ord\", ordX)")
	g.depth--
	g.line("}")
	g.line("chQ := chC.N")
	g.line("fmt.Println(chT, chSum%d(chC), chQ.V)", k)
	// A field lent to a function that only lends its parameter to a
	// temporary borrow: lent in place, so the chain stays owned.
	g.line("fmt.Println(chSum%d(chC.N))", k)
	g.line("chC.V %s= chT", op())
	g.line("trP := trPair%d(%s)", k, v())
	g.line("trP.Bump(%s)", v())
	g.line("trP.R.Bump(%s)", v())
	g.line("trName%d(trP, %q)", k, []string{"x", "", "yz"}[g.qr.IntN(3)])
	g.line("trMaybe%d(trP.L, %s)", k, v())
	g.line("trMaybe%d(trP.L.L, %s)", k, v())
	g.line("trP.V %s= trP.R.V", op())
	// Takes (RFC 0017, step 6): a field moved out and refilled at once.
	g.line("trP.K = append(trP.K, mkTr%d(%d, %s), &Tr%d{V: %s})", k, g.qr.IntN(3), v(), k, v())
	g.line("trP.K[0].V %s= %s", op(), v())
	g.line("fmt.Println(trKids%d(trP), len(trP.K), trP.K[1].V)", k)
	g.line("var trB *Tr%d", k)
	g.line("for i := 0; i < %d; i++ {", 3+g.qr.IntN(6))
	g.depth++
	g.line("trB = trIns%d(trB, (i*%d %s %s) %% 17)", k, 1+g.qr.IntN(9), op(), v())
	g.depth--
	g.line("}")
	g.line("fmt.Println(trB.Sum(), trB.V)")
	g.line("trL := trP.L")
	g.line("trP.L = nil")
	g.line("trR := trP.R")
	g.line("trP.R = mkTr%d(%d, %s)", k, g.qr.IntN(3), v())
	g.line("fmt.Println(trL.Sum(), trR.V, trP.L == nil, trDepth%d(trR))", k)
	g.line("fmt.Println(trP.Leaf(), trP.R.Leaf(), trRight%d(trP, %s), trRight%d(mkTr%d(%d, %s), %s), trDepth%d(trP), trP.Sum())", k, v(), k, k, 1+g.qr.IntN(4), v(), v(), k)
	// Slices of pointers, and pointers in interfaces (RFC 0013, step 2).
	g.line("ptrL.Kids = append(ptrL.Kids, ptrA, newNd%d(%s, nil), ptrL.Next)", k, v())
	g.line("ptrK := []*Nd%d{ptrL.Next, nil, ptrN}", k)
	g.line("ptrK = append(ptrK, ptrL.Kids[%d])", g.qr.IntN(3))
	g.line("ptrT := 0")
	g.line("for _, kd := range ptrL.Kids {")
	g.depth++
	g.line("kd.V %s= %s", op(), v())
	g.line("ptrT += kd.V")
	g.depth--
	g.line("}")
	g.line("fmt.Println(ptrT, len(ptrL.Kids), ptrK, ptrK[1] == nil, ptrK[0] == ptrL.Next)")
	g.line("var ptrS Sz%d = ptrL", k)
	g.line("var ptrZ2 *Nd%d", k)
	g.line("ptrS2 := []Sz%d{ptrS, ptrL.Next, ptrZ2}", k)
	g.line("fmt.Println(ptrS.Sum(), ptrS, ptrS2[2] != nil, ptrS == Sz%d(ptrL), ptrS2[1] == ptrS)", k)
	g.line("fmt.Printf(\"%%T %%v\\n\", ptrS, ptrS2[2])")
	g.line("if ptrQ, ok := ptrS2[%d].(*Nd%d); ok {", g.qr.IntN(3), k)
	g.depth++
	g.line("fmt.Println(\"nd\", ptrQ == nil, ptrQ == ptrL)")
	g.depth--
	g.line("}")
}
