# The patina subset

This page is the specification of the Go that `gopatina build -patina`,
`-emit-rust` and `-patina=check` accept today, and of the ownership rules the
borrow checker applies to it. Anything not listed here is rejected with a
Go-style error that names the construct, never translated approximately.
The feature matrix is in [compat.md](compat.md); the design and the later
phases are in [RFC 0001](rfcs/0001-gopatina.md).

Every program under [`testdata/golden`](../testdata/golden) is built both
ways in CI, with `go build` and with `gopatina build -patina`, and both
binaries must print the same output and exit with the same status. Every
program under [`testdata/reject`](../testdata/reject) must be rejected with
the exact error committed beside it.

## The pipeline

1. **Front end.** `go/build` selects the package's files with the `patina`
   build tag set, `go/parser` parses them, and `go/types` type-checks them at
   the language version of the module's `go.mod`.
2. **Subset checker** (`internal/subset`). Rejects every construct outside
   this page, and plans how errors become `Result`.
3. **Borrow checker** (`internal/borrow`). Decides who owns each value,
   which parameters are borrowed, and which values are moved or copied;
   rejects what it cannot express without changing what the program does.
   `-explain` prints every decision it made on its own.
4. **Code generator** (`internal/emit`). Writes a cargo crate: readable,
   rustfmt-clean Rust 2024, byte-for-byte deterministic, with
   `gopatina-manifest.sha256`.
5. **Build.** `-patina` runs exactly `cargo build --release --locked` on the
   crate. rustc's borrow checker verifies the result; a rustc error is a
   GoPatina bug.

## Language

| Area | Supported in phase 1 |
| --- | --- |
| Packages | `package main` with `func main()`, or another package as a library crate ([Libraries](#libraries)), any number of files, and the packages it imports: of its own module, of other modules and of the standard library, translated from their Go source; only what `main` and the `init` functions reach is translated ([RFC 0009](rfcs/0009-packages.md)); `math/bits` intrinsics give gc's results |
| Imports | `fmt`, `strings`, `errors`, `os`, `io`, `slices`, `bytes`, `time` (only the functions and types below) |
| Types | every integer type (`int`, `int8` to `int64`, `uint`, `uint8` (`byte`) to `uint64`, `uintptr`, `rune`), `float32`, `float64` ([RFC 0008](rfcs/0008-integer-and-float-types.md)), `bool`, `string`, slices of supported types, named struct types, named types of the basic and slice types (`type Op int`, `type Path []int`, [RFC 0007](rfcs/0007-named-types-and-aliases.md)) and type aliases, `error` (an interface value, see [Errors](#errors)), `io.Writer` (as a parameter), interface types: named and literal, embedded, `any` ([RFC 0012](rfcs/0012-interfaces.md); see [Interfaces](#interfaces)), `time.Duration` with its constants and its methods `String`, `Nanoseconds`, `Microseconds`, `Milliseconds`, `Seconds`, `Minutes`, `Hours`, `Truncate`, `Round` and `Abs` (RFC 0011: a provided type, its methods in Rust with Go's behaviour) |
| Declarations | functions and methods, `init` functions (run in source order before `main`), package-level variables in Go's initialization order ([RFC 0010](rfcs/0010-package-variables.md)) (on structs and on named non-struct types), struct types, named types, type aliases, constants (typed, untyped, `iota`), local variables |
| Statements | assignment (`=`, `:=`, `op=`, tuple assignment), `++`/`--`, `if`/`else` with an init statement, `for` (3-clause, condition, infinite), `for range` over slices, strings and integers, `switch` (with and without a tag, several values per case, `default` anywhere), type switches (`switch v := x.(type)`, with `nil`, concrete and interface cases and several types per case), `break`, `continue`, `return` |
| Expressions | arithmetic, comparison, logical and bitwise operators, shifts (by a constant or a run-time count: Go's 0 or −1 at and past the width, and Go's panic for a negative count), string concatenation, indexing of slices and strings, struct and slice literals, numeric conversions, `[]byte(s)`, conversions to interfaces (`any(x)`, implicit ones), method calls (also through an interface), type assertions (`x.(T)` and `v, ok := x.(T)`), `&x` and `*p`; Phase 2 adds [read-only slice suffix arguments](#read-only-slice-suffix-arguments) |
| Built-ins | `len`, `append` (as `s = append(s, ...)`, including `s = append(s, t...)` and `b = append(b, "text"...)`), `make` for slices, `panic` with a string, `min` and `max` (Go 1.21, integers only) |
| Pointers | borrows (`p := &x`, `p := &T{...}`, pointer parameters and receivers) and, [RFC 0013](rfcs/0013-pointers.md), pointers that escape: returned, stored in fields and variables, reassigned, compared, `nil`; see [Pointers](#pointers) |
| `fmt` | `Print`, `Println`, `Printf`, `Sprint`, `Sprintln`, `Sprintf`, `Fprint`, `Fprintln`, `Fprintf` (to an `io.Writer` parameter, `os.Stdout` or `os.Stderr`), `Errorf`; verbs `%v %d %s %t %f %g %x %X %c %%` (`%g` of a float, without a precision or the `+` or `0` flag) with the `-`, `+` and `0` flags, a width and a precision; `%x` and `%X` also of strings and `[]byte` and with the `#` flag; values and slices of a named type printed through its `String` or `Error` method (where the method cannot panic); a width on a slice pads each element, as in Go; interface values with `Println`, `Sprintln`, `Fprintln` and plain `%v` and `%T` |
| Other functions | `strings.Contains`, `HasPrefix`, `HasSuffix`, `Index`, `Join`, `Repeat`, `TrimSpace`, and (RFC 0011) `ToLower`, `ToUpper`, `EqualFold`, `Split`, `Cut`, `TrimPrefix`, `TrimSuffix`, `ReplaceAll`, `IndexByte`; `errors.New`, `Is`, `As`, `Unwrap`; `os.Exit`; `slices.Clone`, `bytes.Clone`; (RFC 0011) `math.Sqrt`, `Abs`, `Floor`, `Ceil`, `Trunc`, `Copysign`, `Signbit`, `IsNaN`, `IsInf`, `Inf`, `NaN`, `Float64bits`, `Float64frombits` and the math constants, each giving Go's result bit for bit, NaNs included, on amd64 and arm64 |

Not supported yet, each with its own error: goroutines, channels and
`select` (RFC 0002), maps, closures and function values, the interface
cases listed under [Interfaces](#interfaces), generics, `defer`, labels and `goto`,
`fallthrough`, a `break` that leaves a `switch` before the end of its case,
general slicing (`s[i:j]`), arrays, package-level variables of slice type
that the program changes (and the other cases RFC 0010 lists), embedded fields,
variadic functions, the other built-ins
(`cap`, `copy`, `new`, `clear`, ...), `min`/`max` with float arguments, functions without
a body (assembly), and complex numbers.

## How Go values become Rust values

| Go | Rust |
| --- | --- |
| `int`, `int64` / `int8` / `int16` / `int32` / `uint8` / `uint16` / `uint32` / `uint`, `uint64`, `uintptr` | `i64` / `i8` / `i16` / `i32` / `u8` / `u16` / `u32` / `u64` |
| `float32` / `float64` / `bool` | `f32` / `f64` / `bool` |
| `string` | `String` (owned), `&str` (a borrowed parameter) |
| `[]T` | `Vec<T>` (owned), `&[T]` or `&mut [T]` (a borrowed parameter) |
| `struct` | a `struct` deriving `Clone, Default, PartialEq` (and `Copy` when every field is) |
| `type T U` (U not a struct), `type A = B` | `type T = R;`, the underlying type's Rust type; methods of `T` are functions `t_m(recv, ...)` taking the receiver first (RFC 0007) |
| `*T` | `&T` or `&mut T`; `p := &T{...}` owns the value |
| `(T, error)` | `Result<T, Iface>` (see [Errors](#errors)) |
| `io.Writer` | `&mut dyn std::io::Write` |

Go's integers wrap on overflow, so emitted crates turn Rust's overflow checks
off in every profile; a signed division by a value that is not a constant
uses `wrapping_div` and `wrapping_rem`, so `math.MinInt64 / -1` wraps as in
Go. A panic, including an index out of range or a division by zero, prints
`panic: <message>` to standard error and exits with status 2, as Go does.
Floats print with Go's formatting (`rt::Float`), slices print as `[a b c]`,
and `%x` of a negative number prints a minus sign, all as in Go.
Go 1.21's `min` and `max` are supported for integers (`std::cmp::min` and
`std::cmp::max`, folded from the left for more than two arguments); calls with
float arguments are rejected rather than translated with Rust's NaN rules.

`int` is 64 bits, as on every target patina mode builds for. Converting a
float that does not fit to an integer is implementation-specific in Go;
patina mode uses Rust's `as`, which saturates.

## Ownership: what the borrow checker decides

Values fall into four kinds:

- **Copy**: numbers, booleans, structs of them. Copied, as in Go.
- **Value**: strings, and structs of strings and Copy values. One owner;
  copying one never changes a program's meaning, because nothing in it can
  be changed through another copy (Go strings are immutable).
- **Shared**: anything holding a slice. Two Go copies share the backing
  array, so a change through one is seen through the other. A Rust copy
  would not share it, so the checker never copies these on its own.
- **Pointers**: borrows of a variable, or, when they escape, boxes with
  one owner (see [Pointers](#pointers)).

Each parameter and receiver gets a mode from what the function does with it:

| What the function does | Mode | Rust |
| --- | --- | --- |
| only reads it | shared borrow | `&T`, `&str`, `&[T]`, `&self` |
| changes memory the caller shares (elements, `*p`, `p.X`) | exclusive borrow | `&mut T`, `&mut [T]`, `&mut self` |
| keeps it (stores, returns or hands it on), or changes its own copy | owned | `T`, `mut x: T`, `self` |

At every hand-off (an assignment, an owned parameter, a `return`, a literal
field or an `append`), the checker moves the value when nothing uses it
afterwards, and otherwise copies it when a copy keeps Go's meaning:

- a Value is always copied (cloned);
- a Shared value is copied only into a function that neither keeps it nor
  changes what it shares, so the copy dies before anyone could tell;
- an element of a slice is never moved out: a Value element is copied, a
  Shared one is rejected.

Otherwise the program is rejected, with both lines and a fix, for example
[`testdata/reject/ownership`](../testdata/reject/ownership/want.txt):

```text
./main.go:33:2: m.Body is changed after m was handed to out.Queue on line 32
        out.Queue(m)     // line 32: out.Queue keeps m
        m.Body[0] = 'D'  // line 33: this also changes the copy out.Queue kept, which shares m.Body's memory
    patina mode does not copy m on its own: a copy would not share that memory, so the program could behave differently
    if out.Queue should keep the original, hand it a copy: Message{To: m.To, Body: bytes.Clone(m.Body)}
    or finish using m before handing it over
```

It also rejects, in Go terms:

- using a variable while a pointer to it is still in use;
- passing a variable to a call together with a later argument that changes
  it (when the earlier argument is a string, the checker copies it first
  instead, and when the later argument only reads a number, it computes the
  arguments into variables first, as Go evaluates them);
- reading a variable and changing it in operands whose order Go leaves
  unspecified (`fmt.Println(c.N, c.Next())`);
- a range loop over a slice that both appends to it and changes it, or
  replaces it; and copying a Shared element out of a range loop that then
  changes it.

Within one expression, such as the operands of `+` or an `if` condition,
a read of a variable, or of a place under one (a field, an element, `*p`),
before a call that changes it reads the changed value: gc makes an
expression's calls before it reads the variables. The Go spec leaves that
order unspecified; GoPatina matches gc. The generated Rust makes such calls
first, into locals (`let call = add(&mut a, 10); a.n + call`), a
`switch` tag's before the `match`, an `else if` condition's inside its
`else`, and a loop condition's each time round, before its test:
`loop { let call = ..; if x + call >= 20 { break; } .. }`, the post
statement still run before each `continue`; and a `case` expression's in
an `else` of the cases before it, when Go comes to it.

A range loop over a slice that the body does not change iterates over a
borrow of it; one that the body changes iterates over the indices of the
length it had when the loop started, as Go does.

### Read-only slice suffix arguments

`read(s[i:])` can pass a temporary view without copying if it is a
standalone call to a local function with one `[]int` parameter and no
results. `s` must be a `[]int` local or parameter; `i` must be an `int`
local or parameter, or an integer literal or named integer constant that
fits `int`. The callee may read the length or elements, range over or
print the slice, or forward it to another proven reader. It cannot write
through the view, reassign it, or keep it after the call. See the
[slice suffix RFC](rfcs/0004-read-only-slice-suffix.md) for the exact rules.

The view ends when the call returns. It uses `len(s)`, not capacity, so
`i == len(s)` is empty (including `i == 0` on a nil slice), and a dynamic
out-of-bounds start panics before the callee runs. Explicit high bounds,
three-index slices, saved views, and computed bounds are not supported.

`gopatina build -patina -explain` (or `-emit-rust -explain`) lists every
decision: each parameter's mode with the reason, each move and each copy.

## Errors

`error` is an interface ([RFC 0012](rfcs/0012-interfaces.md)): an error value
is an `Iface` like any other, so it can be stored, passed, compared, kept in
fields and slices, printed, switched on and asserted, and any type with an
`Error() string` method can be one. `errors.New` and `fmt.Errorf` make
values of Go's `*errors.errorString`, each equal only to itself.

A function whose last result is `error` returns `Result<T, Iface>`:

- `return v..., nil` is `Ok(...)`; `return zero values..., e` with an error
  that cannot be nil (`errors.New`, `fmt.Errorf`, a value of a concrete
  type, or the `err` of the enclosing check) is `Err(e)`; `return zero
  values..., err` with an error that may be nil is `Ok` or `Err` by its
  value; `return f(...)` passes `f`'s `Result` on. A value returned with an
  error that is or may be non-nil must be the zero value, so a caller sees
  exactly what Go's caller sees.
- A call whose error is checked at once (`v, err := f()` followed by
  `if err != nil { ... }`, or `if v, err := f(); err != nil { ... }`) is a
  `match`, and `if err != nil { return ..., err }` is Rust's `?`.
- Any other call keeps its error as a value: its results are split into
  the values, zero when the error is not nil, and the error.

The provided `errors` API ([RFC 0011](rfcs/0011-provided-packages.md)):
`fmt.Errorf` with one `%w` of an error variable makes Go's `*fmt.wrapError`;
`errors.Unwrap`, `errors.Is` and `errors.As` (to `&v` for a local `v` of an
interface or error type) follow Go's errors package, through the wrapped
errors and the program's own `Unwrap() error` and `Is(error) bool` methods.

Package-level variables of an interface type that nothing changes, such as
sentinel errors (`var ErrNotFound = errors.New("not found")`), are
thread-local statics initialized in Go's order, read as copies of the
value; one that the program changes, or an interface value inside a
package-level struct or slice, is not supported yet.

Not supported yet: a precision on an error printed with `%v` or `%s`, an error result
that is not the last, more than one `%w` in a format and error types with
`Unwrap() []error` (`errors.Join`'s form), and `errors.As` in a program with
an error type that has an `As` method.

## The crate

`gopatina build -emit-rust -o dir ./pkg` writes:

| File | |
| --- | --- |
| `Cargo.toml` | edition 2024, `rust-version = "1.85"`, no dependencies, the reproducible release profile (`codegen-units = 1`, `debug = false`, `strip = true`, `incremental = false`) |
| `Cargo.lock` | for `--locked` |
| `rust-toolchain.toml` | Rust 1.93.0, minimal profile, the target |
| `.cargo/config.toml` | the explicit target (from `GOOS`/`GOARCH`) |
| `src/main.rs` | the program, with the Go comments |
| `src/rt.rs` | the runtime support it uses, and nothing else |
| `gopatina-manifest.sha256` | the sha256 of every other file, for `sha256sum -c` |

`-o` defaults to `rust` in the package directory, and gopatina refuses to
write into a directory that holds files it did not write.

`gopatina verify ./pkg` emits the package twice into different directories
and requires identical manifests, then builds it through the `-patina` path
and with plain `cargo build --release --locked` in another directory, and
requires byte-identical binaries. CI runs it on every golden program.

Targets: `linux/amd64` and `linux/arm64` (the `-gnu` targets), `darwin/amd64`,
`darwin/arm64`, `windows/amd64`, `windows/arm64`. The static musl targets of
RFC 0001 are planned.

## Interfaces

[RFC 0012](rfcs/0012-interfaces.md). GoPatina compiles the whole program,
so it knows every type ever converted to an interface: an interface value
is one enum of those types, `Dyn`, held by value
(`Iface = Option<Dyn>`, `None` for a nil interface, or `Option<Box<Dyn>>`
where a dynamic value can hold an interface value), and a method call
through an interface is a `match` on it. A copy of an interface value only
read while its source does not change is a borrow of the source
(`let b = &a;`); any other copy is a clone, as Go copies it.

A pointer converted to an interface moves into it: the interface owns
the object ([RFC 0017](rfcs/0017-one-owner.md)), and `x.(*T)`,
`errors.As(err, &q)` and a type switch's `case *T:` binding borrow it
(`Option<&T>`, or `Option<&mut T>` where the program changes through it).
A pointer converted while it is still used, a field's pointer converted,
and a copy of an interface value holding a pointer while the original is
still used would give the object two owners, and are errors. A nil
pointer variable may be converted and still used.

Supported: interface types (named, literal, embedded, `any`) whose methods
take and return supported types; converting values of the program's own
types, the basic types and slices to them, explicitly or implicitly;
method calls through them (a nil interface panics as in Go); `==` and
`!=` (two values of an uncomparable type panic as in Go); `x.(T)` and
`v, ok := x.(T)` to concrete and interface types, with Go's panics;
type switches; interface values in variables, parameters, results, struct
fields and slices; `Println` and plain `%v` and `%T` of them.

Not supported yet, each with its own error:

- interface methods with a parameter that holds a slice (in Go the callee
  shares it), or with `error`, `io.Writer` or pointer parameters or results;
- converting a provided type (`time.Duration`), a type whose `String` may
  panic, or a struct with a field `fmt` prints through a method;
- `Print`, `Sprint` and `Fprint` of an interface value (they put spaces
  between operands by their dynamic types), and flags, widths and verbs
  other than `%v` and `%T` on one;
- a `switch` on an interface value (as opposed to a type switch), and a
  type switch with an init statement;
- package-level variables holding an interface value that the program
  changes, or inside a struct or slice;
- `errors.As` to a pointer type in a program with its own `Unwrap`
  method;
- interfaces with type sets (generic constraints), and method values.

## Libraries

[RFC 0014](rfcs/0014-library-crates.md). A package other than `main`,
given to `gopatina build -emit-rust`, becomes a Rust library crate that a
Rust project depends on like any other: its exported functions, methods,
structs, constants and sentinel errors are `pub` items with Rust types.
Strings and slices are taken as `&str` and `&[T]` when the function only
reads them, and as `String` and `Vec<T>` when it keeps them; results are
owned. A function whose last result is an `error` returns
`Result<T, Error>`, where `Error` is an opaque Go error that prints its
`Error()` text and compares as Go's `==`; an exported sentinel error is a
function, `pub fn err_not_found() -> Error`. The crate's `api.txt` lists
every `pub` signature and, under each function, every parameter it takes
ownership of and why. The crate is named after the last element of the
package's import path (`-crate-name` overrides it), versioned with
`-version vX.Y.Z`, and a workspace using it sets
`[profile.dev.package.<crate>] overflow-checks = false`, since Go's
integers wrap.

Not supported yet in a library, each with its own error: `init`
functions; package-level variables the library changes or initializes by
calling the program; exported variables other than sentinel errors; and,
in the API, interfaces other than `error`, pointers that escape, and
`error` anywhere but a function's last result (a parameter or an exported
field).
`gopatina build -patina` of a library is refused: it has no binary.

## Pointers

[RFC 0013](rfcs/0013-pointers.md). A pointer that only lends a variable to
a call stays a borrow, as before. A pointer type `*T` some pointer of which
escapes (is returned, stored in a field, reassigned, compared, set to
`nil`, or held by a package-level variable) is a heap pointer, and every
object has one owner ([RFC 0017](rfcs/0017-one-owner.md)).

**Owned pointers** ([RFC 0016](rfcs/0016-owned-pointers-and-cycle-collection.md)).
A heap pointer is a plain `Option<Box<T>>`, a `Box` with one owner,
`None` for `nil`, as Rust writes a pointer that may be absent; no runtime
type stands between, and no reference count. A use that would give an
object a second owner is an error naming the use, why, and how to write
it; a pointer type whose one owner cannot be inferred yet is not
supported yet, with the reason. Its pointers
are only ever new (`&T{...}` or a function's result), moved once into the
variable, field or result that owns them, and then only read through,
tested for `nil` and lent to functions; `T` is a struct of numbers,
booleans, strings and heap pointers, declared in any package of the
program. That is a tree built and then read.
A parameter or receiver that is lent one is an `Option<&T>`; a read
through it is `n.unwrap().f`, through a variable or field
`p.as_ref().unwrap().f`, a lend `p.as_deref()`, and `p == nil` is
`p.is_none()`. Two owned pointers compare by address, as Rust compares
references, and comparing is only a read of both: `std::ptr::eq(a, b)`
for two never `nil` (`&*b` for a `Box`, the `T` it owns),
`a.is_some_and(|p| std::ptr::eq(p, b))` where one may be, and
`a.map(std::ptr::from_ref) == b.map(std::ptr::from_ref)` where both may
be. Two owners are never equal; an owner and a borrow of it are. Unwrapping `None` is Go's nil dereference, with Go's panic
message. Such a tree can only be built by nested new pointers, so a chain
is never deeper than the calls that built it, and Rust's own drop frees
it.

**Pointers never nil** ([RFC 0017](rfcs/0017-one-owner.md), step 2).
Where the program shows that an owned pointer is never `nil`, it is held
without the `Option`, as Rust writes it:

- a function whose every `return` is a new pointer (`&T{...}`, `new(T)`,
  a call of such a function, or a local variable holding one) returns a
  `Box<T>`, and `Some(...)` wraps it where an `Option` holds it;
- a local variable declared from such a value, never assigned again and
  never compared with `nil`, is a `Box<T>`;
- a parameter or receiver that the function only reads through or lends,
  never compares with `nil` or assigns, and reads through before anything
  else it does could be seen (in Go's order: an expression's calls first)
  is a `&T`, and a method with such a receiver takes `&self`. A caller
  holding an `Option` unwraps it at the call, which panics exactly where
  Go's callee would. So every call must pass it with nothing to see after
  it among the arguments (only constants and variables), or pass a
  pointer that is itself never `nil`, and never pass `nil` itself.

`if p != nil { ... }`, for an owned pointer `p` that may be `nil` (a
variable, a parameter or a field of one), is `if let Some(p) =
p.as_deref() { ... }`, the body reading through the binding;
`if p != nil && c { ... }` with no `else` nests `if c` inside it (Rust
1.85, the crates' oldest, has no let chains). The binding is not made
when the body assigns the variable, returns or stores `p` itself, or
tests it again.

**Changes through an owned pointer** ([RFC 0017](rfcs/0017-one-owner.md),
step 3). A number, boolean or string field changed through an owned
pointer (`p.v = x`, `p.v += x`, `p.v++`) keeps the type owned. The
change is made in place, as Rust does through an exclusive borrow. A
parameter or receiver that the function changes through, directly or by
lending it to a function that does, is a `&mut T`, or an
`Option<&mut T>` where it may be `nil`, and a method that changes its
receiver takes `&mut self`. A variable written through is
`p.as_mut().unwrap().v = x`, and the body of `if p != nil` that changes
`p` binds it with `as_deref_mut()`. An `Option<&mut T>` is reborrowed
(`as_deref_mut()`) where it is used again, and moved at its last use.

A call that lends a pointer to be changed while another argument reads
it, `f(n, g(n))` or `f(n, n.v)`, computes that argument first, as a
statement does for any pointer, and other fields of the same variable
do not overlap (`f(w.left, w.right.v)`). Two uses are errors, each naming
the line, why, and the rewrite:

- lending one object twice to a call that changes it through one of them
  (`n.m(n)`, `f(n, n)`, `f(n, n.next)`): two names for it while it is
  changed;
- assigning a parameter another pointer: the parameter holds a borrow of
  the caller's object, which a new object cannot replace; a new local
  variable can.

**Moves** ([RFC 0017](rfcs/0017-one-owner.md), step 4). An owned
pointer held in a local variable moves to a new owner when it is assigned
to another variable (`b := a`, `b = a`, `var b = a`), to a field
(`x.next = head`), or given to a new pointer's literal
(`&T{next: head}`). From there its old name is dead: using it is an
error that names the line it moved on, until the variable is assigned
again. A field may also be assigned a new pointer
(`t.next = &T{...}`), which frees the old one. In Rust these are plain
moves (`x.next = head;`, `head = Some(x);`). A struct type whose
pointers are moved into its fields can form a chain longer than any call
depth, so it gets the `Drop` a Rust programmer writes for a linked list:
the links taken out one at a time in a loop (with a stack of links for
two or more such fields), and its literals name every field, since a type
with a `Drop` has no `..Default::default()`.

**Temporary borrows** ([RFC 0017](rfcs/0017-one-owner.md), step 5). A
local variable that only ever holds a pointer another place owns (it is
assigned a variable, a field read through one, or `nil`, and at least
once something other than `nil`), and that only reads through it (fields
read, methods that only read, tests for `nil`, lent to functions that
only read it, assigned to another such variable) borrows it, as Rust's
`let mut p = head.as_deref()` does: an `Option<&T>`. Its owner keeps the
object, and is still usable after the borrow's last use. `for p :=
head; p != nil; p = p.next { ... }` is Rust's `while let Some(n) = p {
...; p = n.next.as_deref(); }`, the body reading through the binding.
Changing or moving the owner while the borrow is still in use is an
error, as in Rust: "head is changed on line 9 while p, a borrow of it,
is still in use on line 10". A borrow given another borrow (`end = p`)
shares what that one borrows, so moving on to the next node is no
change to the owner.

**Takes** ([RFC 0017](rfcs/0017-one-owner.md), step 6). A pointer moved
out of a field into a local variable (`l := t.left`, `l = t.left`, `var
l = t.left`) whose very next statement in the same block assigns that
field again (`t.left = nil`, `t.left = &T{...}`) is a take: the variable
owns the object, and the field never names it meanwhile. Rust writes
`let l = t.left.take();`, which leaves `None`, so a refill with `nil` is
already done; another value is stored after it.

**Slices of owned pointers** ([RFC 0017](rfcs/0017-one-owner.md), step
7). A struct whose pointers have one owner may hold a slice of them in a
field (`Kids []*Node`): a `Vec<Option<Box<T>>>` owning each. `x.f =
append(x.f, ...)` pushes new pointers or moves local ones in
(`x.kids.push(Some(...))`). An element is read, changed in place and lent
by index (`x.kids[i].as_ref().unwrap().v`), and through elements of
elements, and an element is given a new pointer (`ps[0] = leaf(7)` is
`ps[0] = Some(leaf(7))`). The value of a `range` over the slice borrows
each element in turn (`for k in &x.kids`), and it must be a temporary
borrow: one kept elsewhere would give the element two owners, an error.
A cycle, such as vertices whose edges point back at them, has no single
owner, and is refused once its types are owned
(`testdata/reject/owned_graph_cycle`).

**Second owners are errors** ([RFC 0017](rfcs/0017-one-owner.md)). A
use that gives an object a second owner, which one owner per object
cannot express, is an error naming the use and how to write it with one:

- a pointer copied out of a place and kept, `k0 := root.Kids[0]` changed
  through, `[]*Node{root, root.Kids[1]}`, `ps[0] = root` with `root` used
  after, one tag stored in every leaf, a ring closed through `Prev` and
  `Next` (`testdata/reject/owned_aliases_kept`);
- a range variable that keeps or changes through an element;
- a variable's address held in a variable, field or literal while the
  variable is still used by name (`cp := &cx`, then `cx` beside `cp`:
  `testdata/reject/owned_address_kept`);
- a borrow a function lends back kept in a field or literal
  (`testdata/reject/owned_returned_borrow_kept`). Kept in a local, it is a
  borrow of the argument's variable, `keepC := hold(&c)` as
  `let keep_c = hold(Some(&mut c))`, and a change to `c` while `keepC` is
  in use is the borrow checker's error
  (`testdata/reject/owned_returned_borrow_addr`).

A local that only reads through a pointer another place owns is a
temporary borrow, and may be compared with nil or with another pointer
(`for p := head; p != stop; p = p.Next`). A local that changes through
one, a `&mut` borrow, is not supported yet, and the error says so. An
interface value owns the pointer it is given ([Interfaces](#interfaces)).

**Field addresses lent** ([RFC 0017](rfcs/0017-one-owner.md)). The
address of a field of data inside an owned pointer's box, lent to a call
that keeps nothing (`inc(&p.n)`, `(&p.in).Inc()`, `p.in.Inc()`), is
Rust's `inc(&mut p.n)` and `p.r#in.inc()`; through a pointer that may be
nil, `inc(&mut q.as_mut().unwrap().n)`. Rust unwraps such a pointer whole
to lend its field, so another argument that reads through it is computed
first, as Go evaluates it (`let arg = q.as_ref().unwrap().next...;`, a
string's as a copy), and one that changes it is an error
(`testdata/reject/owned_lend_through_nil`). The field's pointer type must
not escape elsewhere in the program: an escaping `*int` makes every
`&x.f` of type `*int` a pointer into a struct, not supported yet.

**Value methods of types holding a slice** ([RFC 0017](rfcs/0017-one-owner.md)).
Go's copy of a value holding a slice shares the slice. So a value method
called through an owned pointer (`p.Total()`, `ns.Set(9)` on a named
slice) borrows the box's value rather than cloning it: `&self` where the
method only reads, `&mut self` where it changes the slice's elements
(`s.clear()`, `nums_set(&mut ns, 9)`). A value method that keeps or
returns its receiver still copies it, and keeps the type shared.
`*p` of such a value may be printed (`fmt.Println(*ns)`).

**Walking to a returned borrow** ([RFC 0017](rfcs/0017-one-owner.md)).
A function may walk from its pointer input to the node it returns: a
local only ever given the input, a field of itself (`p = p.Next`) or
`nil`, and only read through, compared and returned, is a borrow of the
input all the way, and returning it lends the input back:
`func find(head *Node, v int) *Node` is
`fn find(head: Option<&Node>, v: i64) -> Option<&Node>`, walking with
`p = p.unwrap().next.as_deref()`. A walker that changes through the node
it holds, a `&mut` walk, is not supported yet.

**Kept parameters** ([RFC 0017](rfcs/0017-one-owner.md), step 8). A
parameter that its function keeps (returns, moves into a variable, a
field, a literal or a slice, or passes to a parameter kept in turn) is
moved in: an `Option<Box<T>>` by value, `mut` where the function changes
through it. Its caller gives it a new pointer, a local variable or kept
parameter (dead from there: a later use is the use-after-move error), or
the field the call's result is assigned back to, taken out for the call
(`t.left = insert(t.left.take(), v)`). A take may also move into a field
(`n.next = s.top.take(); s.top = n`). A search tree's insert and a stack's
push read as Rust writes them.

**Local slices of owned pointers** ([RFC 0017](rfcs/0017-one-owner.md)).
A local variable or parameter of type `[]*T` owns its elements as a
field's slice does: a `Vec<Option<Box<T>>>` built by a literal of new
pointers or moved locals (`vec![Some(...), ...]`), appended to
(`s.push(Some(x))`), changed in place and lent by index, and lent whole
(`&[Option<Box<T>>]`, ranged over with `for n in ns`). A copy of such a
slice that is still used is an error: Go's copy shares the elements, so
each would have two owners.

**Struct values owning pointers** ([RFC 0017](rfcs/0017-one-owner.md)).
A struct of the program that is not behind a pointer, and whose fields
are owned pointers, slices of them and plain data, owns its pointers as a
Rust struct owns its `Box` fields: built by a literal, given new pointers
in its fields, lent, returned and moved whole. It may also be a field of
an owned pointer's struct (`o.In.Back = &Outer{}` is
`o.r#in.back = Some(Box::new(..))`); storing an object in such a field
inside itself (`o.In.Back = o`) is a cycle, refused
(`testdata/reject/owned_struct_value_cycle`). A copy of one that is still
used is an error, as for a slice of pointers: each pointer would have two
owners.

**The address of a local value handed on** ([RFC 0017](rfcs/0017-one-owner.md)).
A local struct value whose address is returned, assigned, put in a
literal or appended moves into a new box at the `&` when nothing uses it
after: `n := Node{V: 1}; n.Name = "n"; return &n` is
`let mut n = Node { .. }; n.name = ..; Some(Box::new(n))`. Until the `&`
it is a plain local, read and changed through its fields. The `&` must be
its only one, and must not run again with the value already moved: not in
a loop or function literal the value is declared outside of. Each of
several names declared together takes its own box (`a, b := "a", "b";
return &a, &b`), unless one call gives them all (`a, b := f()`). A local
slice of data moves the same way when nothing else uses it
(`ns := Ints{1, 2}; return &ns` is `vec![1, 2]`); through its pointer it
is indexed (`(*p)[i]` is `p[i]`), changed in place and measured
(`len(*p)`). An `&x` given to a parameter that its callee keeps, with
nothing using `x` after, moves the same way: `keepIn(h, &tt)` is
`keep_in(&mut h, Some(Box::new(tt)))`. Other addresses of locals still
make the type shared.

**Local borrows** ([RFC 0017](rfcs/0017-one-owner.md)). A pointer to a
local, `p := &x`, that the function uses itself (through `*p` and `p.f`,
by a method with a pointer receiver, or lent to a function that keeps
nothing) and never assigns, returns or stores is a Rust borrow of `x` for
its life: `let p = &mut x` where something changes through it, `&x` where
it only reads. While `p` is still in use, `x` is not: changing `x` is an
error, and so is reading it beside an exclusive `p` that is used again
after. A print that reads a place through `p` and then `x`, as in
`fmt.Println(p.N, x.N)`, reads `p.N` into a local first
(`let n = p.n; println!("{} {}", n, x.n)`), so the borrow ends before `x`
is read, as Go's copy of the argument already does:

```
./main.go:24:17: n is used on line 24 while m, a borrow of it, is still in use on line 25
    m changes n through its borrow, so n cannot be used until m's last use (RFC 0017)
    finish with m before n is used, or use n through m
```

**Returned borrows** ([RFC 0017](rfcs/0017-one-owner.md), decision 4). A
function or method that returns one of its pointer inputs, or a place
through it (`n.Next`, `n.Kids[0]`, `*pp`), on every path or `nil`, and
otherwise only reads through that input, lends it back: Rust's
`fn second(n: &Node) -> Option<&Node> { n.next.as_deref() }`. The caller's
result is a borrow of its argument, not a new owner, so the argument stays
the owner and the result is held only while the argument is not changed.
Where Rust's elision would not tie the result to that input (a function
with more than one borrowed input, or a method lending back a parameter,
not its receiver), the lifetime is named:
`fn after<'a>(a: Option<&'a Node>, b: &Node) -> Option<&'a Node>`. A
function that changes through the input it lends back, or lends it to a
call, or whose caller changes through the result, lends it exclusively:
`fn bumped(n: &mut Node) -> Option<&mut Node>`. The caller's result is then
a local borrow of the argument, as `p := &x` is: the argument is not used
while the result is.

Moving a pointer out of an element, a slice of pointers held in a
package-level variable, and a field handed to a keeper without being
refilled are not supported yet: those are later steps. `-explain` notes
that each struct type's pointers have one owner.

A statement that reads through a heap pointer and calls the program makes
its calls first, as gc does. Dereferencing `nil` panics with Go's message,
`runtime error: invalid memory address or nil pointer dereference`, and
exit status 2; a method call on a nil pointer works until the method
dereferences it.

Slices of pointers (`[]*T`) are vectors of heap pointers. A pointer to a
named type of the program can be an interface's dynamic type: method calls
through the interface, assertions (`x.(*T)`), type switches, `==` (which
compares the pointers), `%T`, `errors.As` into a `*T`, and a nil `*T` in a
non-nil interface, all as in Go.

A pointer is printed only through its type's `String` or `Error` method
(directly, in a slice, or in an interface), because Go prints any other
pointer by its address. The method may dereference its receiver: for a nil
pointer, Go's fmt prints `<nil>` when the method panics, and so does patina
mode (the one place it catches a panic).

Not supported yet, each with its own error:

- a pointer into a struct field or a slice element that escapes (`&x.f`,
  `&s[i]` returned or stored), and a pointer to a pointer variable that
  escapes;
- printing a pointer whose type has no `String` or `Error` method, or an
  interface value that can hold one; and a pointer to a type other than a
  named type of the program in an interface;
- converting `*T` to an interface where `T` holds a slice and has value
  methods (a call would run on a copy that does not share its elements);
- a pointer type whose objects would have more than one owner, with the
  use that gives them a second one;
- calling, through a heap pointer, a value method that changes its copy
  of a value holding a slice;
- a condition that reads through a heap pointer and calls a function of
  the program, and a package-level pointer variable that the program
  changes;
- a package-level variable holding pointers in a slice, an array or a
  struct;
- the address of a range variable that escapes.

With one owner per object, no cycle of pointers can form, and Rust's own
drop frees every object when its owner goes.

