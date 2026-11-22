# RFC 0007: Named non-struct types and type aliases

- **Status:** Draft
- **Author(s):** GoPatina contributors
- **Created:** 2026-11-22
- **Discussion:** the pull request that adds this file
- **Supersedes / superseded by:** none

## Summary

Accept, in patina mode, named types whose underlying type is a supported
basic type or a slice of a supported type (`type Kind int`,
`type Celsius float64`, `type Name string`, `type Path []Point`), with their
methods, constants, conversions and printing, and type aliases
(`type A = B`) whose target is a supported type. A named type becomes a
one-field Rust tuple struct (a newtype), which has the same layout and cost
as its underlying type; an alias becomes a Rust `type` alias. Go semantics
are kept exactly, including `fmt`'s use of a `String` or `Error` method.

## Motivation

The project's correctness and coverage goal is 100% of real Go functions,
other than those that use `reflect` or `unsafe` (the project lead,
2026-11-21). The coverage measurement (`docs/COVERAGE.md`) found that the
largest group of reasons functions are rejected was "struct features", and
named non-struct types are a large part of it: enumerations
(`type Op int` with `iota` constants and a `String` method), units
(`type Duration int64`), and named slices (`type Tokens []Token`) are
everyday Go. A function that uses such a type, or calls a function that
does, cannot be compiled today. In the measurement's greedy roadmap, named
non-struct types are the first step: adding them lets more functions in than
adding any other single feature. (The measured shares are reviewed before
anything is published, so they are not quoted here.)

## Guide-level explanation

```go
type Op int

const (
	Add Op = iota
	Sub
	Mul
)

func (o Op) String() string {
	switch o {
	case Add:
		return "+"
	case Sub:
		return "-"
	}
	return fmt.Sprintf("Op(%d)", int(o))
}

type Celsius float64

func (c Celsius) Fahrenheit() float64 { return float64(c)*9/5 + 32 }

type Path []int

func (p *Path) Push(x int) { *p = append(*p, x) }

type Count = int

func main() {
	var p Path
	p.Push(3)
	var n Count = len(p)
	fmt.Println(Mul, Sub, Op(7), Celsius(100).Fahrenheit(), p, n)
	fmt.Printf("%v %d %s\n", Add, Add, Add)
}
```

prints, in every build mode,

```
Op(2) - Op(7) 212 [3] 1
+ 0 +
```

In `-emit-rust` mode the types become:

```rust
#[derive(Clone, Copy, Default, PartialEq)]
struct Op(i64);

const ADD: Op = Op(0);

#[derive(Clone, Default, PartialEq)]
struct Path(Vec<i64>);

type Count = i64;
```

## Reference-level explanation

### What is accepted

A type declaration at package level, `type T U`, where `U` is one of:

- a basic type the subset supports (`int`, `int64`, `int32`/`rune`,
  `uint8`/`byte`, `float64`, `bool`, `string`, and the types later RFCs add);
- a slice of a supported type;
- another named type accepted by this RFC or a named struct type, in which
  case `T`'s underlying type is that type's underlying type. As in Go, `T`
  does not get the other type's methods.

A type alias at package level, `type A = B`, where `B` is any supported type,
named or not.

Still rejected, each with its own message: a named type whose underlying type
is a map, channel, function, interface, pointer or array (each waits for its
feature); generic types and generic aliases; types and aliases declared inside
a function.

### Representation

- `type T U` becomes `struct T(R);`, where `R` is `U`'s Rust type. It derives
  `Clone`, `Default` and `PartialEq`, and `Copy` when `R` is `Copy`, by the
  same rule as structs. A one-field tuple struct has the same size, alignment
  and calling convention as its field, so there is no run-time cost.
- `type A = B` becomes `type A = R;`.
- Type names follow the existing rules for struct names, including the
  renaming of names Rust reserves (`String`, `Vec`, `Option` and so on).

### Values and operators

The emitter knows every expression's Go type, so it unwraps and rewraps
explicitly instead of implementing Rust operator traits:

- Arithmetic, bitwise, shift and unary operators on `T` are the underlying
  operation on `.0`, wrapped back: `a + b` is `T(a.0.wrapping_add(b.0))`,
  with exactly the wrapping, division-by-zero, shift and float rules the
  underlying type has today. `x++`, `x--` and `x op= y` act on `x.0`.
- Comparisons are on `.0`: `a < b` is `a.0 < b.0`, so a NaN in a named float
  type compares as Go does.
- String concatenation, indexing (`s[i]`), `len`, `append`, `range` and
  slice literals of a named string or slice type act on `.0`; a slice literal
  `Path{1, 2}` is `Path(vec![1, 2])`.
- Conversions: `T(x)` is `T(e)`, where `e` is `x` converted to `R` by the
  existing numeric and string conversion rules (applied to `x.0` when `x` is
  itself a named type), and `U(t)` is `t.0` converted. `string(k)` for an
  integer `k` keeps Go's rune conversion.
- **Assignability.** Go lets a value of an unnamed type be assigned to a named
  type with the same underlying type without a conversion
  (`var p Path = []int{1}`, passing `[]int` to a `Path` parameter, returning
  it), and lets untyped constants take a named type (`var o Op = 2`,
  `o == 2`). The emitter inserts the wrapping `T(...)`, or the unwrapping
  `.0`, wherever `go/types` records such an implicit conversion: assignments,
  arguments, results, composite-literal elements, comparisons and `switch`
  cases.
- **Constants.** A typed constant `const Add Op = iota` is
  `const ADD: Op = Op(0);`, with the value computed by `go/types`, as today.
- Zero values are `T::default()`, which is the underlying type's zero value.

### Methods

Methods on `T` are emitted as `impl T { ... }`, under the rules for methods
on struct types today: a value receiver is `self` or `&self`, a pointer
receiver is `&mut self`, and method calls on addressable values and on
pointers take the borrow they need. Inside a method, the receiver's
underlying value is `self.0`; `*p = append(*p, x)` is
`self.0.push(x)` through the existing `append` rules.

### Printing

`fmt` prints a value of a named type as Go does:

- If `T`'s method set (the value type's method set, not the pointer's, since
  `fmt` receives a value) has `Error() string`, the verbs `%v` and `%s` (and
  `%x`, `%X` once supported) print the result of `Error()`; otherwise, if it
  has `String() string`, those verbs print `String()`. This is `fmt`'s
  `handleMethods` order: `error` before `Stringer`.
- Other verbs (`%d`, `%f`, `%t`, `%c`) and a type with neither method print
  the underlying value exactly as the underlying type prints today. A pointer
  receiver `String` method is not used for a value, as in Go.
- `Println(p)` with `p` a named slice prints the elements, each by these
  rules.
- A `String` or `Error` method that panics: Go's `fmt` recovers it and prints
  `%!v(PANIC=String method: <message>)` (with the verb in use in place of
  `v`, and `Error method` for `Error`; checked against Go 1.27.1, as is
  every output in this RFC). The emitter will print the same,
  using `std::panic::catch_unwind` around the method call in the runtime's
  formatting helper; if that cannot be made exact for every panic GoPatina
  emits, the checker instead rejects calling a `String` method that can
  panic, with its own message, until it can.
- A `Format` or `GoString` method (`fmt.Formatter`, `fmt.GoStringer`) needs
  `fmt.State` or `%#v`, which are not supported; a type that has one is
  rejected when printed.

### Borrow checker

A named type has its underlying type's ownership class (Copy, Value or
owned slice, `docs/patina-subset.md`), so the borrow rules are unchanged.

## Compatibility

Default mode is unchanged. Patina mode only accepts programs it rejected
before; every program it accepts today emits the same Rust.

## Testing

- **Golden programs** (each with its exact Rust and a differential run against
  Go's reference build): an enumeration with `iota`, a `String` method and
  printing with every supported verb; named integer types of each width with
  overflow, division by zero and shifts; a named float type with NaN
  comparisons; a named string type with concatenation, indexing, `len` and
  `range`; a named slice type with a pointer-receiver `append` method and
  printing; named types of named types and conversions between them;
  assignability in every position listed above; an alias of a basic, a
  slice, a struct and a named type; a `String` method that panics; an
  `Error` method taking precedence over `String`; a pointer-receiver
  `String` not used.
- **Rejections:** one test per message above (map, channel, function,
  interface, pointer and array underlying types; generic and local types;
  `Format` and `GoString` methods).
- **Generator** (`internal/progen`): declares named integer, float, string
  and slice types, uses them in arithmetic, conversions, assignability
  positions, methods (including `String`) and printing, so the differential
  run exercises them with random values.
- **Differential run:** a part of the generated tests on the test server
  with the new generator, 0 defects required before the feature is called
  done, and the 6σ run continues on the compiler that includes it.
- **Benchmarks:** none of the benchmarks uses named types, so their code is
  unchanged; `bench/asm-check.sh` confirms it.

## Drawbacks

- Every use of a named type carries `.0` and a rewrap in the Rust, which is
  noisier to read than Go. It is what a careful Rust programmer writes for a
  newtype without operator traits, and it costs nothing at run time.
- Recovering a panicking `String` method needs `catch_unwind`, which only
  works with `panic = "unwind"`; emitted crates use that today.

## Rationale and alternatives

- **Rust `type` aliases for named types** (`type Op = i64;`) would avoid the
  `.0`, but cannot carry methods (Rust forbids inherent `impl` on `i64`) and
  would make `Op` and `int` the same type, losing Go's distinction, which
  matters for method sets and printing. Rejected.
- **Operator traits** (`impl Add for Op`) would make the Rust read like Go,
  but each type needs a dozen impls, and wrapping arithmetic still needs
  explicit calls. The emitter already knows the types; explicit `.0` is
  simpler and exact. It can be revisited for readability later without any
  change in behaviour.
- **Free functions for methods** (`fn op_string(o: i64)`) keep `type` aliases
  but are not how Rust is written, and lose the method-call syntax.

## Prior art

- Rust's newtype idiom is the standard way to give a primitive its own type
  and methods.
- TinyGo and gccgo compile named types to their underlying representation; in
  Go's own compiler a named type is the same machine type as its underlying.

## Unresolved questions

- Whether the panicking-`String` case is emitted exactly or rejected, decided
  during implementation by whether every panic message GoPatina emits can be
  captured.

## Future possibilities

- Named map, function, channel and interface types, as those features arrive.
- `%q`, `%x` on strings, `%#v` and `fmt.Formatter` with the `fmt` work in
  later RFCs.
