# RFC 0014: Library crates

- **Status:** Draft
- **Author(s):** GoPatina contributors
- **Created:** 2026-11-22
- **Discussion:** the pull request that adds this file
- **Supersedes / superseded by:** extends RFC 0009 (packages), which
  translates only programs (`package main`)

## Summary

Let `gopatina build -emit-rust` translate a Go package that is not `main`
into a Rust **library crate**: its exported functions, methods, types and
constants become `pub` Rust items with signatures a Rust developer would
write, and the crate builds, tests and lints with plain `cargo`. A
hand-written Rust project can then depend on it like any other crate. The
first user is a showcase, `examples/rust-interop`: an idiomatic Rust
application one component of which is a crate GoPatina generated from Go.

## Motivation

Today GoPatina translates only whole programs: `package main` with a
`func main`, emitted as a binary crate. That proves the generated Rust
runs, but not that it is ordinary Rust a Rust project can use. The claim
GoPatina wants to be able to show is stronger: a component written in Go,
compiled by GoPatina, dropped into a Rust codebase, called by Rust code
with Rust types, and checked by `cargo build`, `cargo test` and
`cargo clippy` with no GoPatina tooling on the Rust side.

It is also how teams would adopt GoPatina without rewriting a system: one
component at a time, written in Go, living beside hand-written Rust. And it
is the natural unit for the development-speed experiment on the roadmap,
whose tasks are components with a fixed API and a shared test suite.

## Guide-level explanation

A Go package of the patina subset:

```go
// Package pricing quotes an order.
package pricing

import "errors"

// ErrEmpty is returned for an order with no lines.
var ErrEmpty = errors.New("pricing: empty order")

// Line is one line of an order.
type Line struct {
	SKU      string
	Quantity int
	Cents    int
}

// Quote returns the order's total in cents, with a discount of
// percent off lines of at least min items.
func Quote(lines []Line, min, percent int) (int, error) {
	if len(lines) == 0 {
		return 0, ErrEmpty
	}
	total := 0
	for _, l := range lines {
		c := l.Quantity * l.Cents
		if l.Quantity >= min {
			c -= c * percent / 100
		}
		total += c
	}
	return total, nil
}
```

`gopatina build -emit-rust -o pricing ./pricing` writes a library crate:

```rust
// src/lib.rs (excerpt)
#[derive(Clone, Default, PartialEq)]
pub struct Line {
    pub sku: String,
    pub quantity: i64,
    pub cents: i64,
}

pub fn quote(lines: &[Line], min: i64, percent: i64) -> Result<i64, Error> { ... }

pub fn err_empty() -> Error { ... }
```

A Rust application depends on it by path, `pricing = { path = "../pricing" }`,
and calls `pricing::quote(&lines, 10, 5)?`, matching on the error with
`e == pricing::err_empty()` or printing it with `{e}`. The crate's
`Cargo.toml`, `rust-toolchain.toml` and lock file are written as for a
program today; it has no dependencies.

## Reference-level explanation

### Which packages

A package whose name is not `main` is translated as a library. Its
exported identifiers form the crate's **API**; every other package it
imports is translated as a private module (RFC 0009), and nothing of the
package that is not exported is `pub`. A `package main` is translated as
today.

### The API, item by item

| Go | Rust |
| --- | --- |
| exported function `F` | `pub fn f(...)` (snake case, as today) |
| exported method `(T) M`, `(*T) M` with `*T` a borrow | `impl T { pub fn m(&self / &mut self, ...) }` |
| exported struct type | `pub struct`, exported fields `pub`, unexported fields private |
| exported named non-struct type (`type Cents int`) | `pub type Cents = i64;` (RFC 0007) |
| exported constant | `pub const` |
| exported sentinel error variable (`var ErrX = errors.New(...)`) | `pub fn err_x() -> Error`, the same value each call (Go's `==`) |

### Types at the boundary

| Go type in a signature | Parameter | Result |
| --- | --- | --- |
| `int`, `int64`, `uint8` and the other integer types, `float64`, `float32`, `bool` | `i64`, `u8`, `f64`, ... by value (RFC 0008) | the same |
| `string` | `&str` if the function only reads it, else `String` | `String` |
| `[]T` | `&[T]` if it only reads the elements, `&mut [T]` if it changes them, `Vec<T>` if it keeps the slice | `Vec<T>` |
| a struct of the package | `&T`, `&mut T` or `T`, by the same rule | `T` |
| `*T` that does not escape (RFC 0013's borrows) | `&T` or `&mut T` | not allowed |
| the last result `error` | | `Result<T, Error>` |

Where a parameter is borrowed or owned is decided, as for every function
today, by GoPatina's borrow checker from what the function does with it;
the Rust signature states it, so a Rust caller sees in the type whether
the callee keeps its argument. `Error` is a type of the crate: an opaque
Go error value that implements `Display` (its `Error()` text),
`std::error::Error`, `Debug` (the same text) and `PartialEq` (Go's `==`), and
offers `is(&Error) -> bool` (`errors.Is`) and `unwrap() -> Option<Error>`.

### Ownership at the boundary

The caller owns everything it passes by value and everything it gets back:
results are always owned (`String`, `Vec<T>`, structs), never borrowed
from an argument or from the crate. A borrowed parameter is borrowed for
the call only. The crate holds no state between calls (see what is
refused), so no lifetimes appear in the API. A Go panic in the crate is a
Rust panic with Go's message, which the caller can let unwind or catch;
the crate does not install a panic hook (a program does, as today).

### What is refused

Each with its own error naming the item, at `gopatina build`:

- an exported item whose signature has an interface other than `error`, a
  pointer that escapes (RFC 0013's heap pointers, which are not `Send` and
  would expose `rt::Ptr`), a function type, a map or a channel;
- an exported package-level variable other than a sentinel error: Rust
  callers would need a global with Go's initialization order;
- `init` functions and package-level variables initialized by calls, in
  the package or the packages it imports: a library has no start-up to
  run them at (a later RFC can add an explicit `pub fn init()`);
- an exported generic or variadic function (not in the subset anyway);
- everything outside the subset, as today.

Types that hold nothing shared (numbers, strings, slices, structs of
them) are `Send` and `Sync` as Rust derives them; the crate states no
other thread-safety promise until goroutines (RFC 0002).

### Versioning the API

The API is part of the crate's contract with its Rust users, and in
GoPatina it is computed: a change to a Go function's body can change a
parameter from `&str` to `String`. So:

- the crate's version comes from `-version vX.Y.Z` (default `0.0.0`), and
  `Cargo.toml` records the GoPatina version and the Go package path;
- the crate includes `api.txt`, every `pub` item's Rust signature, one per
  line, sorted. `gopatina build -emit-rust -api-check old/api.txt` fails if
  the new API removes or changes a signature without a new major version
  (adding items is allowed in a minor version), so a mode change cannot
  ship silently;
- the generated crate is regenerated in CI and must match the committed
  one byte for byte, as the golden crates do today.

## Compatibility

Programs are unaffected: a `package main` is translated exactly as
before, and every golden crate stays byte-identical. Library mode is new
output for input that is rejected today.

## Testing

- Goldens of library packages under `testdata/golden-lib`, each with its
  crate and `api.txt`, built, tested and linted with `cargo` like the
  program goldens, and checked for byte-identical regeneration.
- For each golden library, a differential test: a Go test program and a
  Rust test call the same exported functions on the same inputs, and their
  outputs must match.
- The generator gains library batches: random exported functions over the
  subset's types, called from a generated Rust test and from Go.
- `examples/rust-interop`: a cargo workspace with a hand-written Rust
  application and the generated crate; CI regenerates the crate and
  compares, runs `cargo build`, `cargo test` and `cargo clippy -D warnings`
  on the workspace with no GoPatina tooling, and runs the Go original and
  the Rust application on the same inputs.
- 100% unit-test coverage of the new code, as always.

## Drawbacks

- Signatures depend on function bodies, through the borrow checker's
  modes. That is honest (the type says what the function does) but makes
  the API less stable than a hand-designed one; `api.txt` and
  `-api-check` exist to make every change visible.
- `Error` is opaque: a Rust caller cannot match on a Go error type's
  fields, only compare with exported sentinels, test `is` and read the
  text. Typed errors are future work.
- The refusals (no interfaces, no heap pointers, no globals in the API)
  exclude packages that are idiomatic Go.

## Rationale and alternatives

- **A C ABI (cgo-style `extern "C"`).** Works from any language, but
  throws away Rust types, ownership and safety at the boundary; the point
  here is that the output is ordinary safe Rust.
- **Exposing the internal representation** (`Iface`, `rt::Ptr`) in the
  API. Smaller design, but every Rust caller would depend on GoPatina's
  internals; an opaque `Error` and refusing heap pointers keep the API to
  types a Rust developer expects.
- **Always owned parameters** (`String`, `Vec<T>`) for a stable API.
  Simpler versioning, but callers would clone needlessly, and it hides
  what the borrow checker knows.

## Prior art

- `wasm-bindgen` and `uniffi` generate bindings from annotated Rust; here
  the source is Go and the output is Rust itself, not bindings.
- C2Rust translates C into Rust crates, including libraries, keeping
  `unsafe`; GoPatina's output has no `unsafe`.
- Go's own `-buildmode=c-shared` exports Go functions through a C ABI with
  the Go runtime attached.

## Unresolved questions

- Whether `Error` should expose typed errors of the package (an enum of
  its error types) in a later version.
- The crate name: the package name (`pricing`) or the last element of its
  import path, when they differ.

## Future possibilities

- `pub fn init()` for packages with `init` functions.
- Heap pointers in the API once `Ptr` can be `Send` (RFC 0002's `Arc`).
- Calling Rust crates from Go (phase 4's interop, the other direction).
