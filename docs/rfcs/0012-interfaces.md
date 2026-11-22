# RFC 0012: Interfaces

- **Status:** Draft
- **Author(s):** GoPatina contributors
- **Created:** 2026-11-22
- **Discussion:** the pull request that adds this file
- **Supersedes / superseded by:** settles RFC 0007's "Forward
  compatibility: interfaces"

## Summary

Accept, in patina mode, interface types (named and literal, with embedded
interfaces, `any`), implicit conversions to them, method calls through
them, `nil` interfaces, `==` and `!=`, type assertions (with and without
`ok`) and type switches, `%v` and `%T` through them, and `error` as an
ordinary interface value. Because GoPatina compiles the whole program
(RFC 0009), it knows every type that is ever converted to an interface: an
interface value is one program-wide Rust enum of those **dynamic types**,
behind a reference count, and a method call is a `match` on it. No trait
objects, no `unsafe`, no `Any`.

## Motivation

The coverage measurement at 54c259f (cov11, 2026-11-22) ranks interfaces
first on the roadmap. Behind that rank stand the uses of the provided
packages that are interfaces or need them: `io.Reader` (553 diagnostics),
`errors.New` and `fmt.Errorf` used as values outside a `return` (1,304 and
1,111), `sort.Interface`, `fmt.Stringer`, `context.Context`. (The shares
themselves are reviewed before anything is published; only these counts of
diagnostics are quoted.)

## Guide-level explanation

```go
type Shape interface {
	Area() float64
	Name() string
}

type Circle struct{ R float64 }

func (c Circle) Area() float64 { return 3 * c.R * c.R }
func (c Circle) Name() string   { return "circle" }

type Square struct{ S float64 }

func (s Square) Area() float64 { return s.S * s.S }
func (s Square) Name() string   { return "square" }
func (s Square) String() string { return fmt.Sprintf("Square(%v)", s.S) }

func describe(x any) string {
	switch v := x.(type) {
	case nil:
		return "nil"
	case int:
		return fmt.Sprint("int ", v)
	case Shape:
		return "shape " + v.Name()
	default:
		return fmt.Sprintf("other %T", v)
	}
}

var ErrX = errors.New("x")

func main() {
	shapes := []Shape{Circle{1}, Square{2}}
	total := 0.0
	for _, s := range shapes {
		total += s.Area()
	}
	var a, b any = Square{2}, Square{2}
	fmt.Println(total, shapes[1], describe(3), describe(shapes[0]), describe(nil), describe("s"), a == b)
	fmt.Printf("%v %T %T %v\n", shapes[0], shapes[1], a, any(nil))
	c, ok := a.(Circle)
	fmt.Println(c, ok, errors.New("x") == ErrX, ErrX == ErrX)
}
```

prints, as Go 1.27.1 prints it,

```
7 Square(2) int 3 shape circle nil other string true
{1} main.Square main.Square <nil>
{0} false false true
```

The dynamic types of this program are `Circle`, `Square`, `int`, `string`
and the provided `errors` type, so its interface values are

```rust
pub(crate) enum Dyn {
    Circle(Circle),
    Square(Square),
    Int(i64),
    String(String),
    Error(rt::GoError), // errors.New: compared by identity
}

pub(crate) type Iface = Option<std::rc::Rc<Dyn>>; // None is a nil interface
```

and `s.Area()` is `shape_area(&s)`, a `match` over the variants that have
an `Area` method.

## Reference-level explanation

### Dynamic types

The **dynamic types** of a program are the types of every value converted
to an interface type in it: explicitly (`any(x)`, `Shape(c)`) or implicitly
(assignment, argument, result, composite element, comparison with an
interface, send), as `go/types` records each conversion. They are collected
over the whole translated program (RFC 0009), including the types the
provided packages convert (`errors.New`'s, `fmt.Errorf`'s). Each becomes a
variant of the crate's one `enum Dyn`, named after the Go type; the Go type
name (`main.Square`, `int`) is kept for `%T` and for the panic messages.

A program with no conversion to an interface has no `Dyn`, and pays nothing.

### Interface values

An interface value is `Iface = Option<Rc<Dyn>>`. `None` is the nil
interface. Converting a value `x` of dynamic type `T` is
`Some(Rc::new(Dyn::T(x)))`, a copy, as in Go; copying an interface value
copies the `Rc`. A value stored in an interface cannot be changed through
it (it is not addressable, and pointer-receiver methods are not in a
non-pointer type's method set), so sharing it is exactly Go's copy.
Interfaces holding pointers (`*T` as a dynamic type) come with the
pointers RFC, which defines how a pointer is held.

Converting between interface types (`Shape` to `any`, and back through an
assertion) changes nothing in the representation: every interface type is
`Iface`, and `go/types` has checked the method sets.

### Method calls

For each interface method `M` called in the program, GoPatina writes one
function that matches the dynamic types having `M`:

```rust
fn shape_area(x: &Iface) -> f64 {
    match x.as_deref() {
        Some(Dyn::Circle(v)) => circle_area(v), // or v.area() for a struct
        Some(Dyn::Square(v)) => square_area(v),
        None => rt::nil_deref(), // Go's "invalid memory address or nil pointer dereference"
        _ => unreachable!(),     // go/types: no other dynamic type has Area
    }
}
```

The `unreachable!` arm is provably dead: only values whose type implements
the interface are converted to it. (It is not `unsafe`; it is the
exhaustiveness Rust requires.)

### Comparison

`a == b` for interface values is Go's: both nil; or the same dynamic type
and equal values. A dynamic type that is not comparable (a slice, a map, a
function, a struct holding one) panics with Go's message, `runtime error:
comparing uncomparable type []int`. A provided error from `errors.New` is
compared by identity (`Rc::ptr_eq`), as Go compares the pointers: two
`errors.New("x")` differ, `ErrX == ErrX` holds. Comparing an interface with
a concrete value converts the value first, as Go does.

### Type assertions and switches

- `x.(T)` for a concrete `T`: the variant's value, or Go's panic,
  `interface conversion: interface {} is main.Square, not main.Circle` (with
  `interface is nil` for nil). For an interface `T`: the same `Iface`, or
  the panic naming the missing method, as Go words it.
- `v, ok := x.(T)`: the value and `true`, or the zero value and `false`.
- A type switch is a `match`: a case of a concrete type is its variant, a
  case of an interface type is the variants implementing it, `nil` is
  `None`, and `default` the rest; `v` has the case's type, as Go gives it.

### Printing

`fmt` through an interface uses the dynamic type: `%v` and `%s` the
`Error` or `String` method of the dynamic value when it has one (RFC 0007's
rule, now dynamic), otherwise the value as its type prints; `%T` the Go
type name; `<nil>` for a nil interface. A method that may panic is handled
as RFC 0007 handles it.

### error

`error` becomes an ordinary interface (`Iface`). The existing lowering of
the common error patterns to `Result` (`if err != nil { return err }`)
stays, now as an optimization the emitter applies where it is exact; every
other use of an error value (stored, compared, passed, a custom error type
with an `Error` method) goes through `Iface`. `errors.New` and `fmt.Errorf`
(without `%w`) return the provided `rt::GoError` dynamic type; `%w`,
`errors.Is`, `errors.As` and `errors.Unwrap` come in the provided `errors`
package's first API (RFC 0011), defined by Go's documentation on top of
this RFC.

### Embedded interfaces

An interface embedding others has the union of their methods; nothing more
is needed, since the representation is the same for all.

### Not in this RFC

Generic constraints (interfaces with type sets), interfaces holding
pointers (with the pointers RFC), method values and method expressions
(with closures), `reflect`.

## Compatibility

Default mode is unchanged. Programs accepted today emit the same crate: the
error patterns already lowered to `Result` stay as they are.

## Testing

- **Golden programs:** the example above; every conversion position;
  assertions and comma-ok of concrete and interface types, failing and not;
  type switches with nil, concrete, interface and default cases and with
  several types per case; `==` of every dynamic kind, including the
  uncomparable panic; `%v`, `%s`, `%T` through interfaces, with and without
  `String`/`Error`; nil interfaces, including a method call on one; custom
  error types; interfaces across packages; embedded interfaces.
- **Rejections:** each case above that is out of scope, with its message.
- **Generator:** each program declares an interface with two or three
  implementing types (of the named and struct kinds it already makes),
  converts values to it and to `any`, calls through it, asserts and
  switches, compares, prints with `%v` and `%T`, from a random stream of
  its own.
- **Differential run:** the per-feature 95% gate.
- **Benchmarks:** none uses interfaces; a small benchmark of interface
  calls in a loop checks the cost against hand-written Rust with an enum.

## Drawbacks

- Every value converted to an interface is a reference-counted allocation.
  For small copied values this is more than gc does (gc stores some small
  values without allocation). It can be avoided later by inlining small
  variants; it does not change behaviour.
- One enum for the whole program grows with its dynamic types; a `match`
  on it is a jump table, which is what an enum-based hand-written Rust
  program does too.

## Rationale and alternatives

- **Trait objects** (`Rc<dyn Shape>`): a trait per interface, `Any` for
  assertions and type switches. They need a trait per interface, `dyn Any`
  downcasts with `'static` bounds, and a separate path for interface-to-
  interface conversions, each of which the closed-world enum makes plain
  data. They are the right choice for open-world (separately compiled)
  code, which GoPatina is not.
- **Boxing per interface type** (one enum per interface): conversions
  between interface types would copy between enums; the one enum avoids
  that and matches Go, where every interface value has the same layout.

## Prior art

- gc's interface values are (type descriptor, data) pairs; the enum
  discriminant is the type descriptor.
- Whole-program compilers (MLton, Rust's own enum dispatch idioms) use
  closed-world sum types for dynamic dispatch.

## Unresolved questions

- Whether small `Copy` dynamic types should be stored without `Rc`; decided
  by the benchmark.
- The exact wording of the assertion panic for interface target types,
  measured during implementation for every case.

## Future possibilities

- `*T` dynamic types with the pointers RFC; `io.Reader` and friends in the
  provided `io`; `sort.Interface` with the provided `sort`;
  `context.Context`.

## Amendment: the first implementation (2026-11-22)

The first implementation follows the design, with these decisions made
while writing it:

- **Names.** Conversions are written `boxed(Dyn::V(x))`; the crate-level
  helpers are `iface_show` (`%v`), `iface_type` (`%T`), `iface_name`,
  `iface_get_t`/`iface_try_t` (assertions to a concrete type, as functions
  so `rustfmt` lays out their calls), `iface_assert`/`iface_try` and
  `iface_missing_i` (assertions to an interface type, reporting the first
  missing method by name, as Go's panic does), and one `dyn_m` dispatcher
  per method called through an interface.
- **Reachability (amends RFC 0009).** A reached type keeps, besides
  `String` and `Error`, the methods named like a method of an interface the
  program declares, since a call through an interface may reach them
  without naming them.
- **Not yet.** `error` as an interface value is the next step; until then
  error values keep phase 1's lowering to `Result`. The other cases left
  out are listed, each with its error, in
  [patina-subset.md](../patina-subset.md#interfaces).

**Goroutines: Rc or Arc.** `Rc` is not `Send`, and an interface value that
crosses goroutines (a channel, a closure a goroutine runs, a package-level
variable) must be shareable. The choice is made once per crate: every
conversion goes through `boxed`, and `Iface` is one type alias, so the
reference count is named in exactly two places. When RFC 0002 lands, a
program that starts a goroutine gets `Arc` there (and `Dyn`'s variants are
then `Send + Sync`, which every supported dynamic type is: numbers,
strings, `Vec`s and structs of them, and nested interface values, which are
`Arc` too); a program that starts none keeps `Rc` and pays nothing. Nothing
else changes: dispatchers, comparisons and assertions take `&Iface`.
Until then, package-level variables holding an interface value are
rejected, since a static must be `Sync`.
