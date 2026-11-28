# RFC 0001: GoPatina, Go on the surface, Rust underneath

- **Status:** Draft. Phase 1 is implemented for the subset specified in
  [docs/patina-subset.md](../patina-subset.md): the borrow checker's
  ownership and borrow inference with automatic moves and copies, `-explain`
  and Go-style errors; the Rust code generator; `-emit-rust`, `-patina`,
  `-patina=check` and `gopatina verify`. Everything else described here is
  planned.
- **Author(s):** Peter Sjolin
- **Created:** 2026-11-16
- **Discussion:** this file's pull request, and issues labelled `rfc`

> **Mission:** Go developers get Rust's performance and safety without the
> painful Rust learning curve.

To be honest about what that means: without a garbage collector, a few
ownership rules remain, and a patina program must respect them. But they are
stated and checked in Go terms. A GoPatina developer writes no Rust syntax,
no lifetime annotations and no `async` keywords; the compiler infers them,
fixes what it safely can, and explains the rest in Go.

## Summary

GoPatina is a Go toolchain front end with three build modes selected by two
flags:

1. **Default (no flag):** plain Go. `gopatina build` runs `go build` with the
   same arguments, so existing Go code keeps working unchanged.
2. **`-emit-rust`:** a defined subset of Go, checked by the GoPatina borrow
   checker and translated to readable Rust source: a cargo crate that a
   developer can keep, edit and build with cargo. It stops there.
3. **`-patina`:** the same pipeline, and then the generated crate is built
   with rustc into a native binary that behaves exactly like Rust under the
   hood: no garbage collector, ownership and borrowing, Rust-grade memory
   safety, and zero-cost use of Rust crates.

Both patina flags have Rust underneath (see
[Why Rust underneath](#why-rust-underneath)). Before any Rust is written, the
[GoPatina borrow checker](#the-gopatina-borrow-checker) infers ownership and
lifetimes from ordinary Go code, fixes automatically what it safely can, and
reports what it cannot in Go terms.

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
- **Rust underneath.** Patina binaries are Rust programs: no garbage
  collector, ownership and borrowing enforced at compile time, memory safety
  without a runtime collector.
- **The compiler does the Rust part.** Ownership, borrows and lifetimes are
  inferred; safe fixes are applied automatically and reported; errors are
  written in Go terms, never as raw rustc output.
- **A documented subset.** The language accepted in patina mode, and every
  rule that differs from Go, is written down. A program the compiler cannot
  prove safe is rejected with an error that says why and how to fix it.
- **Interop.** Patina code can call Rust crates with no FFI layer in between.
- **Shadow Rust and keep up.** Patina-mode output is benchmarked against
  equivalent hand-written Rust, built with the latest stable Rust toolchain,
  for every release and every change to code generation: ratios on CI for
  pull requests, official numbers only on the maintainer's test server after
  merging. The target is identical speed to Rust on each benchmark,
  tracked over time, where identical means equivalent within ±0.5% at 5σ. This is a goal, not a claim; see [`docs/QUALITY.md`](../QUALITY.md) and
  [`bench/`](../../bench/README.md).
- **Fully tested.** 100% unit-test coverage, enforced in CI, plus the
  integration, golden, differential and fuzz tests each phase requires
  ([`docs/QUALITY.md`](../QUALITY.md#other-test-layers-by-phase)).

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
| Emit Rust | `-emit-rust` | No | No: the patina subset only | Readable Rust source (a cargo crate); stops there |
| Patina | `-patina` | No | No: the patina subset only | Native binary: the same crate, built by rustc |

`-patina` and `-emit-rust` accept exactly the same language and run the same
pipeline; `-patina` adds the rustc build. A program that passes `-emit-rust`
passes `-patina`, and the reverse.

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
gopatina build -patina -explain .    # also report every automatic fix
gopatina build -patina=check .       # patina checks, then the Go compiler (GC binary)
gopatina fix -patina ./...           # rewrite Go source to make fixes explicit
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
- Where inference alone is not enough, the
  [borrow checker](#the-gopatina-borrow-checker) first tries a safe automatic
  fix. Only when none exists is the program rejected, with a Go-style error,
  and the developer restructures the code or adds an annotation. The
  annotation syntax is open; one candidate is a directive comment, which
  keeps the source valid Go:

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

## The GoPatina borrow checker

GoPatina has its own borrow checker. It runs on the type-checked Go program
(`go/types`, and Go's SSA form) **before any Rust is generated**, and it
works in Go terms: Go variables, Go source positions, Go words.

### What it infers

- **Ownership:** which variable, field or element owns each value, and where
  a value is moved rather than copied.
- **Borrows:** whether each pointer, slice or method receiver is used for
  reading only (shared) or for writing (exclusive).
- **Lifetimes:** how long each borrow lives, so the generated Rust needs no
  hand-written lifetime annotations. Where Rust's elision rules are not
  enough, the checker writes the lifetimes into the generated code itself.

### What it fixes automatically

When the program as written would not satisfy Rust's rules, the checker
applies a fix if, and only if, it keeps the program's Go meaning:

| Situation in the Go code | Automatic fix in the generated Rust |
| --- | --- |
| A value is passed on and never used again | Move it instead of copying |
| A small value (integers, short strings, small structs) is used after being passed on | Clone it at the hand-off |
| A value is shared by several owners in one goroutine | Wrap it in `Rc<T>` (with `RefCell<T>` if it is mutated) |
| A value is shared across goroutines | Wrap it in `Arc<T>`, or `Arc<Mutex<T>>` if it is mutated |
| A borrow overlaps a later mutation but is not used after it | Narrow the borrow to a scoped block that ends before the mutation |

Every automatic fix is recorded:

- `gopatina build -patina -explain` (or `-emit-rust -explain`) lists each fix
  with its Go source position and the reason.
- `gopatina fix -patina` rewrites the Go source to make the fixes explicit
  (for example, a visible copy, or a `patina.Arc[T]`), in the spirit of
  `go fix`, so the code says what it does.

Fixes have a cost (a clone, a reference count, a lock), so they are never
silent, and a `-patina -strict` mode (name to be decided) can turn them into
errors for code where that cost matters.

### When there is no safe fix

The program is rejected with an error in Go terms, pointing at Go source,
with a concrete suggestion. For example:

```text
./main.go:14:9: variable buf is still used after it was handed to send on line 12
        send(buf)       // line 12: buf moves into send
        log(len(buf))   // line 14: buf used here
    pass a copy (send(slices.Clone(buf))), or use buf before handing it over
```

Raw rustc errors are never the user interface. If one ever appears, it is a
GoPatina bug.

### Rust's checker remains the final proof

The GoPatina checker is designed to be conservative and to explain itself,
but it is not trusted on its own: the generated crate is compiled by rustc,
whose borrow checker verifies the result. A program that GoPatina accepts and
rustc rejects is a GoPatina bug, caught by the test suite (see
[`docs/QUALITY.md`](../QUALITY.md)).

## The `-emit-rust` mode

`-emit-rust` translates each patina package to a Rust module and writes a
cargo crate. The output is meant to be **kept and read by people**: stable
names, Go comments carried over, `rustfmt`-formatted, no generated noise. A
team can use it to inspect what their Go code means under Rust's rules, or to
migrate a package to Rust and continue in Rust.

It is the first mode to be built (see [Roadmap](#roadmap)), because its
output can be checked by eye and by `rustc` itself: if the generated crate
compiles, the Rust compiler has verified the ownership GoPatina inferred.
`-patina` is then little more than `-emit-rust` followed by a rustc build.

### Example

This was written as the intended shape of `-emit-rust` output before the
emitter existed. The [README](../../README.md#example) shows what GoPatina
now generates for the same program: the same Rust, without the comments that
explain the borrows (`-explain` reports them instead), with a crate-level
`allow` for lints Go does not have, and with the panic hook that makes a
panic exit with status 2, as in Go.

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

## Why Rust underneath

Both patina flags generate Rust source and let rustc do the rest. This was
decided by the maintainer on 2026-11-16. Earlier drafts of this RFC left the
`-patina` backend open, with two alternatives: lowering Go directly to rustc's
mid-level IR (MIR) or to LLVM IR, and a Go front end with its own runtime
written in Rust. Both were set aside for now; MIR lowering may be revisited
later as a compile-speed optimisation, behind the same flags.

**For:**

- **Optimisation for free.** rustc and LLVM are a mature optimising
  pipeline; GoPatina does not have to build one.
- **A second, independent checker.** rustc's borrow checker verifies every
  program GoPatina produces, so a mistake in GoPatina's own analysis becomes a
  compile error, not a memory-safety bug.
- **Crate interop.** Generated code and Rust crates live in one cargo
  build, with no FFI layer between them.
- **`-emit-rust` is nearly free.** The readable crate is an intermediate
  product of `-patina` anyway.

**Against:**

- **Slower compiles.** Go's fast builds are part of its appeal; a rustc
  build of the generated crate will be much slower than `go build`.
- **Go patterns with no clean Rust mapping.** Cyclic pointer graphs, `nil`
  pointers and maps, freely shared and mutated pointers, and Go's implicit
  interfaces versus Rust's traits all need rules, wrappers or rejection.
- **Rust errors are unreadable to Go developers.** GoPatina must catch
  problems itself and explain them in Go terms before rustc ever sees the
  code (see [The GoPatina borrow checker](#the-gopatina-borrow-checker)).
- **A toolchain dependency.** Patina modes need a Rust toolchain
  installed, in addition to Go.

## Verified: Rust underneath, byte for byte

"Rust underneath" is a claim that can be checked, and GoPatina is designed so
that anyone can check it with `sha256sum`. There are two guarantees, and both
are implemented for phase 1 and checked in CI on every golden program; the
results are in [`docs/REPRODUCIBLE.md`](../REPRODUCIBLE.md).

### (a) Deterministic emission

The same Go input, GoPatina version and options always produce
byte-identical Rust source. `-emit-rust` writes, beside the crate, a manifest
`gopatina-manifest.sha256` in `sha256sum` format, listing every emitted file:

```text
3b0c…  Cargo.toml
9f2e…  Cargo.lock
4a7d…  rust-toolchain.toml
e1c8…  src/main.rs
```

Anyone can run `sha256sum -c gopatina-manifest.sha256` in the crate. The
emitter must not depend on map iteration order, the clock, the host, the
user, absolute paths or environment variables; this is tested by emitting
every golden input twice, in different directories, and comparing manifests.

### (b) No hidden compiler

The binary from `gopatina build -patina` is byte-identical, with the same
sha256, to the one you get by running plain cargo yourself on the crate that
`-emit-rust` writes:

```sh
gopatina build -emit-rust -o crate ./cmd/app
(cd crate && cargo build --release --locked)
sha256sum crate/target/<target>/release/app   # same as the -patina binary
```

So `-patina` is exactly "`-emit-rust`, then cargo", with nothing added in
between. This needs a reproducible build, which GoPatina sets up in every
emitted crate:

| Setting | Why |
| --- | --- |
| `rust-toolchain.toml` pinning an exact Rust release and target | A different compiler is a different binary |
| `Cargo.lock`, built with `--locked` | The same dependency versions and sources |
| `SOURCE_DATE_EPOCH` set to a fixed value | No build-time timestamps |
| `--remap-path-prefix` for the build directory and `CARGO_HOME` | No host paths in panic messages or debug info |
| `codegen-units = 1`, `incremental = false` | No parallel or incremental code generation differences |
| An explicit target triple (`--target`) | No dependence on the host's default target |
| `strip = true`, `debug = false` (or fixed debuginfo settings) | No symbol tables or debug info that embed paths |

What can still break identity, and how it is controlled:

- **A different toolchain** (rustc, cargo, LLVM version): pinned in
  `rust-toolchain.toml`; `gopatina verify` refuses to compare builds made
  with different toolchains and says so.
- **Host paths** (build directory, user's home, `CARGO_HOME`): remapped.
- **The linker and system libraries** (C runtime objects, the system `cc`
  and its version): the default patina target is static musl, and GoPatina
  is designed to link it with Rust's bundled `rust-lld` and Rust's own musl
  C runtime objects (`-C link-self-contained=yes`), so the host's C
  toolchain is not involved. On targets that use the system linker,
  identity holds only with the same linker and libraries, and the verify
  report names them.
- **Environment and features** (`RUSTFLAGS`, enabled features, build
  scripts): `-patina` passes nothing that the crate's own files do not
  state; build scripts in runtime crates are reviewed for determinism.

### `gopatina verify`

`gopatina verify ./cmd/app` emits the package twice, into different
directories, and requires identical manifests; then it builds the crate
through the `-patina` path, and with plain `cargo build --release --locked`
in another directory, and compares the two binaries' sha256. It fails on any
difference and prints the toolchain, the target and both hashes. CI runs it
on every golden program (`toolchains` workflow) and on `examples/hello`
(`reproducible` workflow).

Phase 1 crates have no dependencies, so no `CARGO_HOME` path can reach the
binary, and cargo passes the crate's own sources to rustc as relative
paths: the build directory does not reach the binary either, and
`gopatina verify` shows it by building in two directories. `-patina` runs
cargo with `RUSTFLAGS`, `CARGO_TARGET_DIR` and the other variables that would
change the build removed, so the crate's own files decide it. The
`--remap-path-prefix` and `SOURCE_DATE_EPOCH` settings above matter once
crates have dependencies (Tokio, RFC 0002); `scripts/repro-check.sh` applies
them to the example crates.

## Deployment and portability

### Binaries need neither Go nor Rust

A compiled binary is native machine code. The machine it runs on needs
neither Go nor Rust installed, exactly as with a Go binary today.

Patina binaries are built **fully static by default**. On Linux this means
Rust's musl targets, which matches Go's static, drop-anywhere binaries: a
patina binary can be copied to any machine a Go binary for the same operating
system and architecture would run on.

**Amendment (2026-11-27): static is opt-in for now.** musl's allocator is
1.25 to 2.3 times slower than glibc's on the benchmarks that allocate
(bench/results/2026-11-27-allocators.md), and the fast alternative measured,
mimalloc, is C, which would bring the host's C toolchain back into static
builds. So `-static` builds the musl targets below, linked by `rust-lld`
with Rust's own musl runtime and no C toolchain. Without it, Linux binaries
use the `-gnu` targets, until a fast allocator for static builds is
settled; then static becomes the default, as planned. (On macOS, as with Go, binaries link
the system library dynamically, because Apple does not support static
linking against it. On Windows, the C runtime is linked statically.)

### Cross-compilation

`GOOS` and `GOARCH` work as they do with `go build`, and map to Rust target
triples:

```sh
GOOS=linux GOARCH=arm64 gopatina build -patina ./cmd/app   # aarch64-unknown-linux-musl
```

| `GOOS`/`GOARCH` | Rust target |
| --- | --- |
| `linux/amd64` | `x86_64-unknown-linux-musl` |
| `linux/arm64` | `aarch64-unknown-linux-musl` |
| `linux/386` | `i686-unknown-linux-musl` |
| `linux/arm` (`GOARM=7`) | `armv7-unknown-linux-musleabihf` |
| `darwin/amd64` | `x86_64-apple-darwin` |
| `darwin/arm64` | `aarch64-apple-darwin` |
| `windows/amd64` | `x86_64-pc-windows-msvc` (static C runtime) |
| `windows/arm64` | `aarch64-pc-windows-msvc` (static C runtime) |
| `freebsd/amd64` | `x86_64-unknown-freebsd` |
| `wasip1/wasm` | `wasm32-wasip1` |

Other pairs are added as they are tested. A pair with no mapping is an error
that names it, never a guess. Cross-compiling needs the Rust standard
library for the target, which toolchain provisioning (below) fetches.

### Building with the Go compiler: `-patina=check`

Patina source is always valid Go, because the patina subset is a subset of
Go. The same code can therefore always be built by the plain Go toolchain.

`gopatina build -patina=check` (name to be finalised) runs GoPatina's borrow
checker and race checks on the program, exactly as `-patina` does, and then
compiles it with the Go compiler. The result is a standard Go binary, with
Go's garbage collector, for machines where Rust cannot be installed. The code
is held to patina's rules, so it can move to a full `-patina` build later
without changes.

This is **never a silent fallback**. `-patina` without Rust available is an
error that suggests the options; `-patina=check` happens only when the
developer asks for it, and the build output says which backend produced the
binary (and the binary records it, in a form to be specified).

### Toolchain provisioning

If a `-patina` or `-emit-rust` build needs a Rust toolchain that the build
machine does not have, gopatina offers, and acts only with the developer's
consent, to either:

- download a pinned, checksummed Rust toolchain into gopatina's own cache
  (in the spirit of Go's toolchain downloads, without touching a system-wide
  Rust installation or `PATH`), or
- build inside the official Rust container image, if a container runtime is
  available.

Neither changes the system without consent. In CI, consent is given once
with a flag or environment variable (name to be decided). The pinned version
follows the compatibility policy of
[RFC 0003](0003-toolchain-compatibility.md).

## Open questions

Each will get an RFC.

1. **Goroutines and channels.** How `go f()`, channels, `select` and `sync`
   map onto Rust, and how data races are prevented, is the subject of
   [RFC 0002](0002-concurrency.md).
2. **Interfaces and traits.** Go interfaces are satisfied implicitly and
   used as dynamic values. Lower to `dyn Trait` objects, to generics with
   trait bounds where the concrete type is known, or both? How do empty
   interfaces (`any`) and type switches work without reflection?
3. **Slices and maps.** A Go slice aliases its backing array; `append` may or
   may not reallocate. Is a slice `Vec<T>` (owned), `&[T]` / `&mut [T]`
   (borrowed), or a GoPatina type that tracks both? What are the rules for
   sub-slicing, which creates aliases?
4. **Escape analysis and inference.** How far can Go's escape analysis
   and SSA drive the borrow checker's inference before fixes or annotations
   are needed? Which automatic fixes are safe enough to apply by default?
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
| 1 | `-emit-rust` for a tiny subset: functions, structs, integers, strings; `-patina` builds the same crate with rustc | Generated crates compile with stable rustc; Go and Rust print the same output on a test suite |
| 2 | The GoPatina borrow checker: ownership and lifetime inference, automatic fixes, `-explain`, `gopatina fix`, Go-style errors | Rejection tests with exact diagnostics; no program accepted by GoPatina is rejected by rustc |
| 3 | Goroutines, channels, `select` and `sync`, per [RFC 0002](0002-concurrency.md) | Data races are compile errors or fixed automatically |
| 4 | Standard library subset and Rust crate interop | A small real program builds in all three modes |

Every phase ships with 100% unit-test coverage and the test layers listed in
[`docs/QUALITY.md`](../QUALITY.md), and is benchmarked against equivalent
hand-written Rust on the latest stable toolchain. Each phase starts with an
RFC for its rules and ends with a release.

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
  `-emit-rust` is in that family; what it adds is a written subset, a
  borrow checker that infers ownership and lifetimes and fixes what it can,
  Go-style errors, and a default mode that keeps ordinary Go working.
- **Rust** itself, whose ownership and borrowing model GoPatina adopts
  rather than reinventing.

## Drawbacks

- The patina subset may turn out to be too small to be useful, or the
  annotations too frequent, for real Go code. Phases 1 to 3 are partly an
  experiment to find out.
- Two languages' worth of semantics in one toolchain is a lot to document
  and to teach.
- GoPatina owns a borrow checker of its own, in addition to relying on
  rustc's. Keeping the two in agreement is real, ongoing work.
- Patina builds will be much slower than `go build`, and need a Rust
  toolchain.
- A Go developer still has to learn ownership. GoPatina can make the syntax
  familiar; it cannot make the model free.

## Unresolved questions

Everything under [Open questions](#open-questions), plus the exact set of
automatic fixes applied by default, and the name of the strict mode.

## Future possibilities

- A `gopatina vet -patina` command that reports which packages of an
  existing program would be accepted in patina mode, and why not.
- Mixed binaries: some packages in patina mode, others in default mode,
  with a defined boundary between them.
- Editor support through `gopls` diagnostics for patina rules.
