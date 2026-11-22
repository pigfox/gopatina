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
| Packages | `package main` with `func main()`, any number of files, and the packages it imports: of its own module, of other modules and of the standard library, translated from their Go source; only what `main` and the `init` functions reach is translated ([RFC 0009](rfcs/0009-packages.md)); `math/bits` intrinsics give gc's results |
| Imports | `fmt`, `strings`, `errors`, `os`, `io`, `slices`, `bytes` (only the functions below) |
| Types | every integer type (`int`, `int8` to `int64`, `uint`, `uint8` (`byte`) to `uint64`, `uintptr`, `rune`), `float32`, `float64` ([RFC 0008](rfcs/0008-integer-and-float-types.md)), `bool`, `string`, slices of supported types, named struct types, named types of the basic and slice types (`type Op int`, `type Path []int`, [RFC 0007](rfcs/0007-named-types-and-aliases.md)) and type aliases, `error` (as a result), `io.Writer` (as a parameter) |
| Declarations | functions and methods, `init` functions (run in source order before `main`), package-level variables in Go's initialization order ([RFC 0010](rfcs/0010-package-variables.md)) (on structs and on named non-struct types), struct types, named types, type aliases, constants (typed, untyped, `iota`), local variables |
| Statements | assignment (`=`, `:=`, `op=`, tuple assignment), `++`/`--`, `if`/`else` with an init statement, `for` (3-clause, condition, infinite), `for range` over slices, strings and integers, `switch` (with and without a tag, several values per case, `default` anywhere), `break`, `continue`, `return` |
| Expressions | arithmetic, comparison, logical and bitwise operators, shifts (by a constant or a run-time count: Go's 0 or −1 at and past the width, and Go's panic for a negative count), string concatenation, indexing of slices and strings, struct and slice literals, numeric conversions, `[]byte(s)`, method calls, `&x` and `*p`; Phase 2 adds [read-only slice suffix arguments](#read-only-slice-suffix-arguments) |
| Built-ins | `len`, `append` (as `s = append(s, ...)`, including `s = append(s, t...)` and `b = append(b, "text"...)`), `make` for slices, `panic` with a string, `min` and `max` (Go 1.21, integers only) |
| Pointers | only as borrows: `p := &x`, `p := &T{...}`, pointer parameters and receivers |
| `fmt` | `Print`, `Println`, `Printf`, `Sprint`, `Sprintln`, `Sprintf`, `Fprint`, `Fprintln`, `Fprintf` (to an `io.Writer` parameter, `os.Stdout` or `os.Stderr`), `Errorf`; verbs `%v %d %s %t %f %x %X %c %%` with the `-`, `+` and `0` flags, a width and a precision; `%x` and `%X` also of strings and `[]byte` and with the `#` flag; values and slices of a named type printed through its `String` or `Error` method (where the method cannot panic); a width on a slice pads each element, as in Go |
| Other functions | `strings.Contains`, `HasPrefix`, `HasSuffix`, `Index`, `Join`, `Repeat`, `TrimSpace`, and (RFC 0011) `ToLower`, `ToUpper`, `EqualFold`, `Split`, `Cut`, `TrimPrefix`, `TrimSuffix`, `ReplaceAll`, `IndexByte`; `errors.New`; `os.Exit`; `slices.Clone`, `bytes.Clone` |

Not supported yet, each with its own error: goroutines, channels and
`select` (RFC 0002), maps, closures and function values, interfaces other
than `error` and `io.Writer`, generics, `defer`, labels and `goto`,
`fallthrough`, a `break` that leaves a `switch` before the end of its case,
general slicing (`s[i:j]`), arrays, package-level variables of slice type
that the program changes (and the other cases RFC 0010 lists), embedded fields,
type switches and assertions, variadic functions, the other built-ins
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
| `(T, error)` | `Result<T, rt::Error>` |
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
- **Pointers**: borrows of a variable.

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

A function whose last result is `error` returns `Result<T, rt::Error>`. This
is exact only for these patterns, which are all the subset accepts:

- every `return` is `return v..., nil`, `return zero values..., err` with a
  non-nil error (`errors.New`, `fmt.Errorf`, or the `err` of the enclosing
  check), or `return f(...)` of a function with the same results;
- every call checks its error at once: `v, err := f()` followed by
  `if err != nil { ... }`, or `if v, err := f(); err != nil { ... } else
  { ... }`; or ignores it (`v, _ := f()`, or `f()` alone).

`if err != nil { return ..., err }` becomes Rust's `?`; other checks become a
`match`. Inside a check, `err` may be printed, returned, or read with
`err.Error()`.

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
