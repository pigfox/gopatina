# RFC 0001: GoPatina, Go on the surface, Rust underneath

- **Status:** Draft (design stage; nothing described here is implemented
  beyond the default-mode pass-through CLI)
- **Author(s):** Peter Sjolin
- **Created:** 2026-11-16
- **Discussion:** this file's pull request, and issues labelled `rfc`

> **Mission:** Go developers get Rust's performance and safety without the
> painful Rust learning curve.

## Summary

GoPatina is a Go toolchain front end with three build modes selected by two
flags:

1. **Default (no flag):** plain Go. `gopatina build` runs `go build` with the
   same arguments, so existing Go code keeps working unchanged.
2. **`-patina`:** a defined subset of Go, compiled to a native binary that
   behaves like Rust under the hood and has the same features: no garbage
   collector, ownership and borrowing, Rust-grade memory safety, and
   zero-cost use of Rust crates. How this backend is built is an open
   decision (see [The `-patina` backend](#the--patina-backend-decision-pending)).
3. **`-emit-rust`:** the same subset and rules as `-patina`, compiled
   straight to readable Rust source: a cargo crate that a developer can keep,
   edit and build with cargo.

The aim is that a Go developer can use Go's syntax and API to write programs
that run the way Rust programs do. Patina mode is not backward compatible
with all Go code, by design; default mode is.

## Motivation

Go and Rust are each loved for things the other lacks.

- Go has a small, readable language, fast compilation, an excellent standard
  library and tooling, and a very large pool of developers.
- Rust has deterministic memory management with no garbage collector,
  compile-time memory and data-race safety, and predictable latency, at the
  cost of a steeper learning curve.

Teams that need Rust's runtime properties today must either rewrite in Rust,
tune Go's garbage collector, or call Rust from Go across cgo. GoPatina asks
whether a third path is viable: keep writing Go, opt specific packages into a
stricter mode, and get Rust's runtime model underneath.

## Goals

- **Zero cost to opt out.** Without a patina flag, GoPatina is exactly the Go
  toolchain. No Go program changes meaning because it was built with
  `gopatina` instead of `go`.
- **Go on the surface.** Patina code is syntactically valid Go, formatted by
  `gofmt`, type-checked by `go/types`, and readable by any Go developer.
- **Rust underneath.** Patina binaries have no garbage collector, enforce
  ownership and borrowing at compile time, and are memory safe without a
  runtime collector.
- **A documented subset.** The language accepted in patina mode, and every
  rule that differs from Go, is written down. A program the compiler cannot
  prove safe is rejected with an error that says why and how to fix it.
- **Interop.** Patina code can call Rust crates with no FFI layer in between.
- **Shadow Rust and keep up.** Patina-mode output is benchmarked against
  equivalent hand-written Rust, built with the latest stable Rust toolchain,
  for every release and every change to code generation. The target is to
  stay within 10% of Rust on each benchmark, tracked over time. This is a
  goal, not a claim.
- **Fully tested.** 100% unit-test coverage, enforced in CI, plus the
  integration, golden and fuzz tests each phase requires.

## Non-goals

- Compiling arbitrary existing Go programs without a garbage collector. Many
  idiomatic Go programs rely on shared, cyclic or long-lived heap data that
  has no single owner; patina mode will reject them.
- Replacing the Go toolchain. Default mode delegates to it.
- Matching Go's runtime behaviour in patina mode (scheduler, finalizers,
  `runtime` package internals).
- Performance claims. None are made until there is something to measure.

## Modes

| Mode | Flag | Garbage collector | Backward compatible with Go | Output |
| --- | --- | --- | --- | --- |
| Default | none | Yes (Go's) | Yes, fully | Native binary built by the installed `go` tool |
| Patina | `-patina` | No | No: the patina subset only | Native binary with Rust's runtime model |
| Emit Rust | `-emit-rust` | No | No: the patina subset only | Readable Rust source (a cargo crate) |

`-patina` and `-emit-rust` accept exactly the same language and apply the
same rules; they differ only in what they produce. A program that passes
`-emit-rust` must pass `-patina`, and the reverse.

### Selecting files

Both patina flags set the build tag `patina`, so a package can carry a patina
implementation beside a plain Go one:

```go
//go:build patina

package ringbuf
```

```go
//go:build !patina

package ringbuf
```

Default mode never sets the tag, so it sees only the `!patina` files and
behaves exactly like `go build`.

### Command line

```sh
gopatina build ./...                 # default: same as go build ./...
gopatina build -patina ./cmd/app     # native binary, no GC
gopatina build -emit-rust ./cmd/app  # writes a cargo crate (location TBD)
gopatina run -patina .               # build and run in patina mode
gopatina version
```

The flags are recognised only directly after the subcommand. Every other
argument is passed through unchanged (default mode) or interpreted with Go's
meaning where it applies (patina modes).

## Default mode

`gopatina build`, `gopatina run` and every other subcommand without a patina
flag execute the `go` command found on `PATH` (or named by `$GOPATINA_GO`)
with the same arguments, environment, standard input and output, and exit
code. This is implemented today and is the compatibility guarantee: anything
`go` accepts, `gopatina` accepts, with the same result.

## The patina subset

The rules below are the starting point for discussion, not a final
specification. Each will get its own RFC before it is implemented.

### Values and ownership

- Every heap value has exactly one owner: a variable, a struct field, a slice
  element or a map entry.
- Assigning a struct, slice, map or string value, or passing it by value,
  **moves** it. Using a variable after it has been moved is a compile error.
  Integers, floats, booleans and structs made only of them are copied, as in
  Go (Rust's `Copy`).
- A pointer `*T` taken with `&x` is a **borrow** of `x`. The compiler infers
  whether it is shared (read only) or exclusive (mutable) from how it is
  used, and rejects programs that would hold a shared and an exclusive borrow
  at the same time, or let a borrow outlive its owner.
- Where inference fails, the program is rejected, and the developer adds an
  annotation. The annotation syntax is open; one candidate is a directive
  comment, which keeps the source valid Go:

  ```go
  //patina:borrow buf mut
  func fill(buf []byte) { ... }
  ```

- `new(T)` and `&T{...}` that escape the function allocate an owned box
  (Rust's `Box<T>`), freed when its owner goes out of scope.
- Shared ownership is explicit, through a library type such as
  `patina.Rc[T]` or `patina.Arc[T]`, never implicit.

### What is excluded (initially)

- `unsafe` and `reflect`.
- Cyclic data structures without an explicit shared-ownership type.
- `runtime.SetFinalizer`, `runtime.GC` and other GC-specific APIs.
- cgo.
- Packages outside the supported standard library subset.

### Errors

Go's `error` values and multiple returns stay as they are in the source. In
`-emit-rust` output, `(T, error)` is expected to lower to `Result<T, Error>`,
with `if err != nil { return ..., err }` becoming `?` where the pattern is
exact. `panic` aborts or unwinds as in Rust (to be decided).

## The `-emit-rust` mode

`-emit-rust` translates each patina package to a Rust module and writes a
cargo crate. The output is meant to be **kept and read by people**: stable
names, Go comments carried over, `rustfmt`-formatted, no generated noise. A
team can use it to inspect what their Go code means under Rust's rules, or to
migrate a package to Rust and continue in Rust.

It is the first mode to be built (see [Roadmap](#roadmap)), because its
output can be checked by eye and by `rustc` itself: if the generated crate
compiles, the Rust compiler has verified the ownership GoPatina inferred.

### Example (illustrative only)

Nothing generates this yet. It shows the intended shape of `-emit-rust`
output for a small patina program.

```go
//go:build patina

package main

import "fmt"

type Point struct {
	X, Y int
}

func (p *Point) Scale(k int) {
	p.X *= k
	p.Y *= k
}

func describe(name string, p Point) string {
	return fmt.Sprintf("%s at (%d, %d)", name, p.X, p.Y)
}

func main() {
	p := Point{X: 1, Y: 2}
	p.Scale(3)
	fmt.Println(describe("origin-ish", p))
}
```

```rust
// Illustrative -emit-rust output. Not produced by any current code.
#[derive(Clone, Copy, Debug, Default)]
pub struct Point {
    pub x: i64,
    pub y: i64,
}

impl Point {
    // `*Point` is used only to mutate `p`: inferred as an exclusive borrow.
    pub fn scale(&mut self, k: i64) {
        self.x *= k;
        self.y *= k;
    }
}

// `name` is only read: inferred as a shared borrow.
// `Point` holds only integers, so it is copied, as in Go.
fn describe(name: &str, p: Point) -> String {
    format!("{} at ({}, {})", name, p.x, p.y)
}

fn main() {
    let mut p = Point { x: 1, y: 2 };
    p.scale(3);
    println!("{}", describe("origin-ish", p));
}
```

Note the open questions this already raises: Go's `int` is 64 bits on most
targets but platform-sized in general (`i64` or `isize`); exported Go fields
become `pub` Rust fields with snake-case names; `fmt` verbs map to Rust
format strings only for a known subset.

## The `-patina` backend (decision pending)

> **Decision pending (maintainer).** How `-patina` produces a native binary
> is not decided. It will be settled by its own RFC after Phase 1, using what
> `-emit-rust` teaches about the subset.

Whatever the backend, `-patina` must give the same guarantees as
`-emit-rust` followed by `cargo build`: no GC, ownership and borrowing
enforced, memory safety, direct calls into Rust crates. The candidates:

### Option B: lower through rustc's backend (MIR or LLVM IR)

GoPatina's front end produces Rust's mid-level IR (MIR), or LLVM IR shaped the
way rustc would emit it, and hands it to rustc's code generation.

- **For:** no Rust source round trip, so faster builds and no surface-syntax
  awkwardness; reuses rustc's optimiser and code generator; calls into Rust
  crates share one compiler.
- **Against:** rustc's internal interfaces (MIR, the driver API) are unstable
  and change between releases, so GoPatina would be pinned to specific
  nightly toolchains; the borrow checker runs on MIR, so GoPatina must
  produce MIR that is not just correct but provably so; harder to debug than
  readable source.

### Option C: Go front end, Rust-style checks, Rust runtime

Keep Go's own front end (`go/parser`, `go/types`, `golang.org/x/tools/go/ssa`),
run GoPatina's ownership and borrow checker on Go's SSA form, and generate
code that links against a small runtime written in Rust (allocation,
strings, slices, maps, panics, and later scheduling) instead of Go's
garbage-collected runtime.

- **For:** closest to "Go on top, Rust underneath"; error messages can point
  at Go source positions and use Go terms; independent of rustc internals;
  could reuse Go's existing compiler stages.
- **Against:** GoPatina must implement and maintain its own borrow checker,
  rather than relying on rustc's; more code to own; interop with Rust crates
  needs a stable ABI boundary (Rust's ABI is not stable, so likely through
  `extern "C"` shims or generated glue).

### Relationship to `-emit-rust`

Emitting Rust source and calling cargo (the obvious "option A") is not a
candidate for `-patina`: it is `-emit-rust`. If `-emit-rust` proves good
enough, `-patina` could start life as `-emit-rust` plus `cargo build` while B
or C is built, but that is an implementation step, not the design.

## Open questions

Each will get an RFC.

1. **Goroutines and channels.** Map `go f()` to OS threads (`std::thread`),
   to an async runtime (futures and an executor), or to a GoPatina scheduler
   written in Rust? Unbuffered and buffered channels map naturally to
   `std::sync::mpsc` or crossbeam-style channels, but `select` and
   cancellation do not map one-to-one. Data sent on a channel is moved.
2. **Interfaces and traits.** Go interfaces are satisfied implicitly and
   used as dynamic values. Lower to `dyn Trait` objects, to generics with
   trait bounds where the concrete type is known, or both? How do empty
   interfaces (`any`) and type switches work without reflection?
3. **Slices and maps.** A Go slice aliases its backing array; `append` may or
   may not reallocate. Is a slice `Vec<T>` (owned), `&[T]` / `&mut [T]`
   (borrowed), or a GoPatina type that tracks both? What are the rules for
   sub-slicing, which creates aliases?
4. **Escape analysis and inference.** How far can Go's escape analysis
   and SSA drive ownership inference before annotations are needed? What do
   error messages look like when inference fails?
5. **Error handling.** `(T, error)` to `Result<T, E>`: what is `E`, and how
   are wrapped errors (`errors.Is`, `errors.As`) represented?
6. **Generics.** Go's type parameters and constraints versus Rust's
   monomorphised generics and trait bounds: which constraints can be
   expressed, and which need a trait to be generated?
7. **`unsafe`.** Excluded at first. Should patina mode later allow Rust-style
   `unsafe` blocks, and with what syntax that stays valid Go?
8. **Rust crate interop.** How does Go code import a Rust crate (an import
   path convention, a directive, a manifest) and how are Rust types that have
   no Go equivalent (lifetimes, enums with data, traits) presented?
9. **Standard library.** Which packages are supported first (`fmt`,
   `strings`, `strconv`, `errors`, `sort`, `os`, `io`?), and are they
   reimplemented, translated, or mapped onto Rust's standard library?
10. **Integer semantics.** Go wraps on overflow; Rust panics in debug
    builds. Patina mode must pick Go's semantics (and emit wrapping
    arithmetic) or document the difference.

## Roadmap

| Phase | Scope | Exit criteria |
| --- | --- | --- |
| 0 | This RFC; CLI with default-mode pass-through; both patina flags report "not implemented" | Merged RFC, CI green, pass-through tested |
| 1 | `-emit-rust` for a tiny subset: functions, structs, integers, booleans, strings, `if`/`for`/`switch`, no pointers, no goroutines | Generated crates compile with stable rustc and match Go's output on a test suite |
| 2 | `-patina` native backend for the same subset: evaluate options B and C on it, choose one by RFC, implement it | Same test suite passes natively, with no GC |
| 3 | Ownership and borrowing inference: pointers, slices, maps, methods; annotations where inference fails | Clear compile errors for rejected programs; both modes agree |
| 4 | Concurrency: goroutines, channels, `select`, `sync` | Data-race freedom enforced at compile time |
| 5 | Standard library subset and Rust crate interop | A small real program builds in all three modes |

Each phase starts with an RFC for its rules and ends with a release.

## Prior art

- **TinyGo** compiles Go through LLVM for microcontrollers and WebAssembly.
  It keeps Go's semantics and a (simpler, conservative) garbage collector.
  GoPatina differs by removing the collector and changing the semantics, in
  an opt-in mode.
- **gccgo** and **GoLLVM** are alternative Go compilers using GCC and LLVM
  back ends. They implement full Go, including garbage collection.
  GoPatina's default mode does not compete with them; it uses whichever `go`
  command is installed.
- **Go's own memory tuning.** `GOGC`, the soft memory limit `GOMEMLIMIT`
  (Go 1.19), escape analysis that keeps values on the stack, and the
  experimental, now on hold, `arena` package reduce GC cost without removing
  it. For most programs these are the right tool, and GoPatina is not meant
  to replace them.
- **Rust from Go via cgo.** Rust code exposed through a C ABI can be called
  from Go today. It works, but each call crosses the cgo boundary, the two
  runtimes manage memory separately, and the Rust part is written in Rust.
- **Go-to-Rust translators.** Various experimental projects have translated
  Go source to Rust source, usually as one-off migration aids. GoPatina's
  `-emit-rust` is in that family; what it adds is a written subset with
  ownership rules, a native `-patina` mode with the same rules, and a
  default mode that keeps ordinary Go working.
- **Rust** itself, whose ownership and borrowing model GoPatina adopts
  rather than reinventing.

## Drawbacks

- The patina subset may turn out to be too small to be useful, or the
  annotations too frequent, for real Go code. Phases 1 to 3 are partly an
  experiment to find out.
- Two languages' worth of semantics in one toolchain is a lot to document
  and to teach.
- Option B ties GoPatina to unstable rustc internals; option C means owning a
  borrow checker. Both are large.
- A Go developer still has to learn ownership. GoPatina can make the syntax
  familiar; it cannot make the model free.

## Unresolved questions

Everything under [Open questions](#open-questions), plus the choice of
`-patina` backend.

## Future possibilities

- A `gopatina vet -patina` command that reports which packages of an
  existing program would be accepted in patina mode, and why not.
- Mixed binaries: some packages in patina mode, others in default mode,
  with a defined boundary between them.
- Editor support through `gopls` diagnostics for patina rules.
