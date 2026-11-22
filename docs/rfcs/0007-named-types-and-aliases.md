# RFC 0007: Named non-struct types and type aliases

- **Status:** Approved for drafting by the coordinator on the project lead's
  authority (2026-11-22, merged in #52); amended 2026-11-22 before any code
  (see "Amendment", merged in #54); implemented 2026-11-22 (see
  "Implementation notes")
- **Author(s):** GoPatina contributors
- **Created:** 2026-11-22
- **Discussion:** the pull request that adds this file
- **Supersedes / superseded by:** none

## Summary

Accept, in patina mode, named types whose underlying type is a supported
basic type or a slice of a supported type (`type Kind int`,
`type Celsius float64`, `type Name string`, `type Path []Point`), with their
methods, constants, conversions and printing, and type aliases
(`type A = B`) whose target is a supported type. A named type and an alias
both become a Rust `type` alias of the underlying type's Rust type, as gc
itself compiles a named type to its underlying machine type; the methods of
a named non-struct type become functions that take the receiver as their
first parameter. Go semantics
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

In `-emit-rust` mode the types and methods become:

```rust
type Op = i64;

const ADD: Op = 0;

fn op_string(o: Op) -> String { ... }

type Path = Vec<i64>;

fn path_push(p: &mut Path, x: i64) {
    p.push(x);
}

type Count = i64;

// p.Push(3) and fmt.Println(Mul) become:
path_push(&mut p, 3);
println!("{}", op_string(MUL));
```

## Amendment (2026-11-22, before implementation)

The draft merged in #52 made a named type a Rust newtype, `struct T(R)`, with
`.0` added and removed at every operator, index, conversion, `len`,
`append`, `range`, comparison, assignment and print. Planning the code showed
that this touches every expression path of the emitter, and that a single
missed site is a compile error at best and a wrong value at worst. This
amendment represents a named type by its underlying type instead:

- Go's distinction between `Op` and `int` is static. `go/types` has already
  enforced it before GoPatina emits anything, and at run time the two are the
  same: gc compiles both to the same machine type. Nothing in an accepted
  program's behaviour depends on Rust keeping the types apart.
- What does depend on the named type (method calls, `String` and `Error` in
  `fmt`, conversions) is decided by the emitter from the Go types, which it
  has for every expression.
- Every existing, tested path for integers, floats, strings and slices
  (wrapping arithmetic, Go's division and shift rules, ranges, appends, RFC
  0006's proven ranges) then applies to named types unchanged.

The newtype design is kept below under "Rationale and alternatives".

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

- `type T U` becomes `type T = R;`, where `R` is `U`'s Rust type, and
  `type A = B` becomes `type A = R;` likewise. Signatures, variables and
  fields use the underlying Rust type; the aliases name the types for the
  reader.
- Type names follow the existing rules for struct names, including the
  renaming of names Rust reserves (`String`, `Vec`, `Option` and so on).
- A named type whose underlying type is a struct (`type B A`, `A` a struct)
  is an alias of `A`'s Rust struct; `B`'s own methods are functions as
  below, so `A`'s and `B`'s methods never mix, as in Go.

### Values and operators

Values of a named type are values of its underlying type, so operators,
indexing, `len`, `append`, `range`, slice literals, zero values and
constants (`const ADD: Op = 0;`) are emitted exactly as for the underlying
type, with Go's wrapping, division, shift and float rules. Go's implicit
conversions (assignability, untyped constants) need nothing.

Conversions: `T(x)` and `U(t)` are the underlying conversion, and nothing at
all when the Rust types are equal. A conversion to a named slice type from a
slice (`Path(xs)`) is the value itself; only a conversion from a string to
`[]byte` (or a named byte slice) is `as_bytes().to_vec()`.

### Methods

A method `func (r T) M(params) results` of a named non-struct type becomes
`fn t_m(r: R', params) results`, where `R'` is the receiver's Rust parameter
type by the existing parameter rules (`i64`, `&str`, `&[T]`, `&mut Vec<T>`
for a pointer receiver, and so on), and a call `x.M(args)` becomes
`t_m(x', args)` with `x'` the receiver passed in that mode (`&mut x` for a
pointer receiver). The names are made unique by the existing namer. Inside
the method the receiver is an ordinary parameter, so `*p = append(*p, x)`
is `p.push(x)` by the existing pointer and `append` rules. Methods of struct
types are unchanged.

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

## Forward compatibility: interfaces

With aliases, `type Celsius float64` and `type Fahrenheit float64` are the
same Rust type, `f64`. Inside this RFC nothing needs them apart: every
method call, conversion and `fmt` use is resolved from the static Go type.
Interfaces (a later RFC) need the *dynamic* Go type: `var s fmt.Stringer =
c`, `any(c)`, type switches and assertions, `%v` and `%T` through an
interface. The plan, which the later RFC will settle:

- **A value gets its Go type where it enters an interface, and only there.**
  A conversion to an interface type is explicit in `go/types` (an
  assignment, argument, result or composite element whose target is an
  interface, or `any(x)`), so the emitter knows the static type at that
  point. For each named type that is ever converted to an interface, the
  crate gets a wrapper, `struct CelsiusIface(f64);` (named by the namer, so
  it cannot collide), and the conversion wraps: `Box::new(CelsiusIface(c))`
  or the interface RFC's chosen representation (a trait object or a tagged
  enum of the types converted in the program). Code that never converts a
  named type to an interface never sees the wrapper.
- **Method sets attach to the wrapper.** The interface's trait is
  implemented for `CelsiusIface` by calling the free functions this RFC
  emits (`celsius_string(self.0)`), so each method exists once, and
  `Celsius` and `Fahrenheit` get separate impls although both are `f64`.
- **`%v`, `%s` and `%T` through an interface** dispatch on the wrapper:
  `Error`, then `String`, by the same `handleMethods` rule as below, and
  `%T` prints the Go name the wrapper records (`main.Celsius`).
- **Assertions and type switches back** (`s.(Celsius)`, `case Fahrenheit:`)
  test the wrapper's identity (`Any::downcast_ref::<CelsiusIface>()` or the
  enum's variant) and unwrap `.0`, which is the alias representation again.

Nothing in the alias representation makes this harder:

- Each named type keeps a stable Rust identifier: its alias name (`Celsius`)
  and the prefix of its method functions (`celsius_`) are given once by the
  namer for the whole crate, so a later wrapper `CelsiusIface` and its impls
  can be derived from them without renaming anything.
- The boundary is the same either way: a newtype would also have to be boxed
  or tagged to become an interface value, so the wrapper costs nothing this
  RFC avoided. What changes is only where the distinct Rust type first
  appears: at interface conversions instead of everywhere.
- Values that never enter an interface keep the underlying representation,
  so the arithmetic paths and RFC 0006's proven ranges stay as they are.

**Readability.** The emitted Rust for these types uses aliases and calls
like `celsius_fahrenheit(c)` rather than newtypes and `c.fahrenheit()`. The
project's claims are about speed and behaviour (identical to hand-written
Rust, equal to Go), not about the emitted code's style. On 2026-11-22 neither
the README nor gopatina.org claims idiomatic method syntax for the emitted
Rust, and pages describing this feature will show the actual output.

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

- Rust does not keep `Op` and `i64` apart, so a reader of the emitted crate
  sees aliases where Go has distinct types. Go's type checker has already
  enforced the distinction, so this cannot change behaviour.
- Methods of named non-struct types are free functions (`op_string(o)`), not
  method calls (`o.string()`), in the emitted Rust.
- Recovering a panicking `String` method needs `catch_unwind`, which only
  works with `panic = "unwind"`; emitted crates use that today.

## Rationale and alternatives

- **Newtypes** (`struct Op(i64)`, the draft merged in #52): keep the types
  apart in Rust and allow `impl Op`, but need `.0` and a rewrap at every
  operator, conversion, index, range, append, comparison and print, and
  explicit wrapping and unwrapping wherever Go converts implicitly. Each site
  is a chance of a wrong translation, for no change in behaviour. Not chosen
  (see "Amendment"); it can be revisited for readability once the emitter
  has a single place for value conversions.
- **A trait per type** (`trait OpMethods { fn string(self) } impl OpMethods
  for i64`) keeps method syntax on aliases, but two named types with the same
  underlying type and a method of the same name make an unqualified call
  ambiguous, so every call must be qualified anyway, and a `&self` receiver
  of a named string is `&String`, not the `&str` the emitter's string paths
  expect. Not chosen.
- **Free functions** with the receiver as first parameter (chosen) reuse the
  emitter's parameter modes, borrow analysis and calls as they are.

## Prior art

- Go's own compiler, gccgo and TinyGo compile a named type to its underlying
  representation, and a method to a function whose first parameter is the
  receiver; this design does the same.
- Rust's newtype idiom is the usual way to give a primitive its own type and
  methods in hand-written Rust; see "Rationale and alternatives".

## Implementation notes

- **Printing through a method is accepted only where the method cannot
  panic.** Reproducing `fmt`'s `%!v(PANIC=...)` exactly needs the panic hook
  muted during the call and every panic message translated, so the checker
  rejects printing through a `String` or `Error` method unless the method is
  built only from constructs that cannot panic (`if`, `switch`, `return`,
  constants, variables, conversions between basic types, `len`, operators
  other than `/`, `%` and shifts, and `fmt.Sprint`, `Sprintf` and `Sprintln`
  of such values without methods of their own). Enumeration-style `String`
  methods pass. Calling such a method directly is always accepted.
- **Not yet:** printing a slice whose element type has a `String` or `Error`
  method, `%x` and `%X` of such a value (`fmt` prints the method's result in
  hex), `%+v` and `%0v` of one, and printing a pointer through the method set
  of `*T`. Each is rejected with its own message.
- **Found while adding the generator cases:** the emitter wrote a shift by a
  constant at least the operand's width (`x << 70`, `b >> 8` for a byte) with
  Rust's operator, which does not implement Go's result. The bug predates
  this RFC and affected `int`, `int32` and `uint8` alike; it is fixed (Go's
  0 or −1, with the operand still evaluated) and tested with it
  (`testdata/golden/shift_wide`).

## Unresolved questions

- None for this RFC. Printing through a method that can panic waits for the
  runtime work above.

## Future possibilities

- Named map, function, channel and interface types, as those features arrive.
- `%q`, `%x` on strings, `%#v` and `fmt.Formatter` with the `fmt` work in
  later RFCs.
