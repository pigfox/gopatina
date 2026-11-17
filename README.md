# GoPatina

> A patina is the thin layer that forms on metal: Go on the surface, Rust underneath.

[![CI](https://github.com/pigfox/gopatina/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/pigfox/gopatina/actions/workflows/ci.yml)

**Mission: Go developers get Rust's performance and safety without the painful
Rust learning curve.**

Without a garbage collector a few ownership rules remain, but GoPatina states
and checks them in Go terms: no Rust syntax, no lifetime annotations, no
`async` keywords to learn.

GoPatina is a Go toolchain front end with an opt-in mode in which Go code
runs the way Rust code does: no garbage collector, ownership and borrowing,
Rust-grade memory safety. Without the flag, it is plain Go, and every
existing Go program builds exactly as before.

> [!IMPORTANT]
> **GoPatina is early.** Phase 1 works for a defined subset of Go
> ([docs/patina-subset.md](docs/patina-subset.md)): functions, structs and
> methods, slices, strings, errors, `fmt` printing. For that subset the
> borrow checker, the Rust code generator, `-emit-rust`, `-patina`,
> `-patina=check` and `gopatina verify` are implemented and tested. Everything
> else, goroutines and maps first among them, is planned and rejected with a
> clear error today. No performance figures are published yet (see
> [Quality and performance policy](#quality-and-performance-policy)).

## Why

Go is simple, fast to compile, and widely known. Rust gives deterministic
memory management with no garbage collector and compile-time memory and
data-race safety, but it takes real effort to learn. Teams that need Rust's
runtime properties today rewrite in Rust, tune Go's garbage collector, or
call Rust from Go through cgo.

GoPatina explores a fourth path: keep writing Go, opt a program into a
stricter mode, and get Rust's runtime model underneath.

## How it works: three modes, two flags

| Mode | Flag | Garbage collector | Backward compatible with Go | Output |
| --- | --- | --- | --- | --- |
| Default | none | Yes (Go's) | Yes, fully | Native binary built by the installed `go` tool |
| Emit Rust | `-emit-rust` | No | No: a defined subset of Go | Readable Rust source (a cargo crate); stops there |
| Patina | `-patina` | No | No: the same subset | Native binary: the same crate, built by rustc |

```sh
gopatina build ./...                 # default: exactly go build ./...
gopatina build -patina ./cmd/app     # native binary, no GC
gopatina build -emit-rust ./cmd/app  # Rust source you can keep
```

- **Default mode** runs the `go` command with the same arguments, streams and
  exit status. Anything `go` accepts, `gopatina` accepts, with the same
  result. This works today.
- **`-emit-rust`** translates a defined subset of Go to readable Rust source,
  a cargo crate a developer can keep, edit and build with cargo, and stops
  there. The output is rustfmt-clean Rust 2024 and byte-for-byte
  deterministic, listed with sha256 in `gopatina-manifest.sha256`.
- **`-patina`** runs the same pipeline and then builds the crate with rustc
  into a native binary that behaves exactly like Rust under the hood: no
  garbage collector, ownership and borrowing, Rust-grade memory safety, and
  direct use of Rust crates.

**Rust underneath, Go on top.** Before any Rust is written, GoPatina's own
**borrow checker** runs on the Go program. It infers ownership, borrows and
lifetimes automatically, and fixes what it safely can: a move instead of a
copy, a clone of a small value, `Rc`/`Arc` for shared values, `Arc<Mutex<T>>`
for values shared across goroutines, a narrower borrow. Every fix is reported
(`gopatina build -patina -explain`), and `gopatina fix` will rewrite the Go
source to make it explicit, like `go fix` (planned). When no safe fix exists,
the error is in Go terms, for example:

```text
./main.go:19:2: buf is changed after buf was handed to log.Add on line 18
        log.Add(buf)  // line 18: log.Add keeps buf
        buf[0] = 'F'  // line 19: this also changes the copy log.Add kept, which shares buf's memory
    patina mode does not copy buf on its own: a copy would not share that memory, so the program could behave differently
    if log.Add should keep the original, hand it a copy: bytes.Clone(buf)
    or finish using buf before handing it over
```

Phase 1 implements the moves, the copies of strings and small structs, and
the inferred shared and exclusive borrows; the `Rc`/`Arc` fixes come with
goroutines (RFC 0002).

rustc's own borrow checker then verifies the generated code as the final
proof.

**Drop-in binaries.** A patina binary is native code, so the target machine
needs neither Go nor Rust, just as with a Go binary. `GOOS`/`GOARCH` choose
the Rust target. Phase 1 builds for each platform's default C runtime (the
`-gnu` targets on Linux); static musl binaries by default, and
cross-compiling with a fetched target, are planned. Because patina code is
valid Go, `gopatina build -patina=check` runs the patina checks and then
builds with the Go compiler, for machines where Rust cannot be installed.
That is never a silent fallback: you choose it, and the output says which
backend built the binary.

The design and its trade-offs are in [RFC 0001](docs/rfcs/0001-gopatina.md):
the [borrow checker](docs/rfcs/0001-gopatina.md#the-gopatina-borrow-checker),
[why Rust underneath](docs/rfcs/0001-gopatina.md#why-rust-underneath) and
[deployment](docs/rfcs/0001-gopatina.md#deployment-and-portability).

Files can carry a patina implementation beside a plain one: both patina flags
set the `patina` build tag (`//go:build patina` and `//go:build !patina`),
and default mode never does.

### Example

`gopatina build -emit-rust -explain` on this program reports its two
decisions and writes the crate below (its `src/main.rs`, as generated).

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

```text
./main.go:9:7: receiver p is an exclusive borrow: Scale changes it, and the caller sees the change
./main.go:14:15: parameter name is a shared borrow: describe only reads it
```

```rust
//! Generated by GoPatina from package main (main.go).
//! Edit the Go source and regenerate, or keep this crate and continue in Rust.

// Go allows unused functions, types, fields, parameters and assignments,
// wraps integers on overflow, and panics on a division by zero at run time;
// Rust warns about the first and rejects the others when it can see them.
#![allow(
    dead_code,
    unused_assignments,
    unused_variables,
    arithmetic_overflow,
    unconditional_panic
)]

mod rt;

#[derive(Clone, Copy, Default, PartialEq)]
pub struct Point {
    pub x: i64,
    pub y: i64,
}

impl Point {
    pub fn scale(&mut self, k: i64) {
        self.x *= k;
        self.y *= k;
    }
}

fn describe(name: &str, p: Point) -> String {
    format!("{} at ({}, {})", name, p.x, p.y)
}

fn main() {
    rt::exit_on_panic();
    let mut p = Point { x: 1, y: 2 };
    p.scale(3);
    println!("{}", describe("origin-ish", p));
}
```

`src/rt.rs` holds the small runtime support the program uses; here, a panic
hook that exits with status 2, as Go does. `gopatina run -patina` and
`go run` both print `origin-ish at (3, 6)`.

## Examples

[EXAMPLES.md](EXAMPLES.md) walks through all three modes with three runnable
programs in [`examples/`](examples/): `hello`, `ownership` (a value used
after it was handed off) and `counter` (goroutines and a shared counter).
For `hello` and `ownership` everything on the page is real output: the
`-explain` report and the crate `-emit-rust` writes for `hello`, and the
Go-style error `-patina=check` gives for `ownership`:

```console
$ gopatina run -patina ./examples/hello
Hello, Gopher! Hello, Gopher!
Ferris 3
```

`counter` uses goroutines, which are planned (RFC 0002): its Tokio crate and
the data-race report are the design, labelled **planned output (design, not
yet implemented)**.

## Status and roadmap

| Phase | Scope | Status |
| --- | --- | --- |
| 0 | RFC 0001; CLI with default-mode pass-through | Done (v0.0.1) |
| 1 | `-emit-rust`, `-patina`, `-patina=check` and `gopatina verify` for the subset in [docs/patina-subset.md](docs/patina-subset.md), with the borrow checker (stage 3) and the Rust code generator (stage 4) | **Implemented** (unreleased) |
| 2 | The rest of the borrow checker: maps, closures, `defer`, slicing, `gopatina fix`, the strict mode | Planned |
| 3 | Goroutines and channels on a multi-threaded async runtime ([RFC 0002](docs/rfcs/0002-concurrency.md)), with the `Arc`/`Mutex` fixes | Planned |
| 4 | Standard library subset and Rust crate interop | Planned |

Every phase ships with 100% unit-test coverage and is benchmarked against
hand-written Rust on the latest stable toolchain.

The open questions (goroutines and channels, interfaces and traits, slice
and map ownership, escape analysis, error handling, generics, `unsafe`) are
listed in the [RFC](docs/rfcs/0001-gopatina.md#open-questions).

## Quality and performance policy

- **100% unit-test coverage**, enforced in CI, plus the integration, golden,
  differential and fuzz tests each phase requires.
- **Shadow Rust and keep up.** Every release, and every pull request that
  changes code generation, is benchmarked against equivalent hand-written
  Rust built with the latest stable Rust toolchain, whose version is recorded
  with each result. The target is to stay within 10% of Rust on each
  benchmark, tracked over time: a goal, not a claim. **No numbers are
  published yet.** The harness ([bench/](bench/README.md)) runs in CI and
  reports ratios only; official numbers come from a dedicated machine.

Details: [docs/QUALITY.md](docs/QUALITY.md) and [bench/](bench/README.md).

**Checksum-verified builds.** Emitted Rust is byte-identical for the same
input, and a `-patina` binary has the same sha256 as plain
`cargo build --release --locked` on the emitted crate: `gopatina verify`
checks both, and CI runs it on every golden program; see
[docs/REPRODUCIBLE.md](docs/REPRODUCIBLE.md).

## Benchmarks

**No official benchmarks have been run yet.** The harness runs on pull
requests and reports ratios only, which are not published as results.
Official results, measured on the maintainer's test server with the full
suite, are published in [BENCHMARKS.md](BENCHMARKS.md); the latest headline
ratios will appear here.

| Benchmark | GoPatina / Rust | GoPatina / Go | Run |
| --- | --- | --- | --- |
| *(none yet)* | | | |

## Supported toolchains

What each GoPatina release supports is stated here and in its release notes
([RFC 0003](docs/rfcs/0003-toolchain-compatibility.md)).

| GoPatina | Go | Rust stable | Rust MSRV | Rust edition |
| --- | --- | --- | --- | --- |
| `main` (Phase 1, unreleased) | the two latest releases (stable, oldstable) | emitted crates pin 1.93.0; CI also tests the latest stable | 1.85 | 2024 |

Default mode works with any Go release, because it runs your installed `go`.
Patina modes refuse Go features they do not support yet, naming the feature
and, where it helps, the Go version that introduced it; see the
[feature matrix](docs/compat.md). Every known place where Go's semantics and
Rust's differ, and what GoPatina will do about it, is listed in
[docs/go-rust-disconnects.md](docs/go-rust-disconnects.md).

## Quick start

Requires Go 1.25 or later; `-patina` and `gopatina verify` also need a Rust
toolchain ([rustup](https://rustup.rs) installs the pinned version on first
use). Phase 1 is not released yet, so install from `main`:

```sh
go install github.com/pigfox/gopatina/cmd/gopatina@main

gopatina build ./...                        # runs go build ./...
gopatina build -emit-rust -o crate .        # writes a cargo crate
gopatina build -patina -explain -o app .    # native binary, with every decision
gopatina run -patina .                      # build and run it
gopatina build -patina=check .              # patina checks, then the Go compiler
gopatina verify .                           # reproducible: -patina == plain cargo
```

`gopatina` uses the `go` and `cargo` commands on your `PATH`, or the ones
named by the `GOPATINA_GO` and `GOPATINA_CARGO` environment variables.

## Prior art

- **[TinyGo](https://tinygo.org/)** compiles Go through LLVM for
  microcontrollers and WebAssembly, keeping Go's semantics and a garbage
  collector. GoPatina removes the collector, in an opt-in mode with different
  rules.
- **gccgo and GoLLVM** are alternative Go compilers using GCC and LLVM. They
  implement full Go, including garbage collection. GoPatina's default mode
  uses whichever `go` you have installed.
- **Go's own memory tuning** (`GOGC`, `GOMEMLIMIT`, escape analysis) reduces
  GC cost without removing it, and is the right tool for most programs.
- **Calling Rust from Go through cgo** works today, with a C ABI boundary on
  every call, two runtimes, and the Rust half written in Rust.
- **Go-to-Rust translators**: various experimental projects translate Go
  source to Rust, usually as one-off migration aids. `-emit-rust` belongs to
  that family; GoPatina adds a written subset with ownership rules, a native
  `-patina` mode with the same rules, and a default mode that keeps ordinary
  Go working.

## Contributing

Contributions are welcome, and at this stage critique of the
[RFC](docs/rfcs/0001-gopatina.md) is the most useful kind. Every change
lands through a pull request with passing CI and is merged only after the
project lead has approved it; commits follow Conventional
Commits and carry a DCO sign-off (`git commit -s`). Language changes go
through the RFC process. See [CONTRIBUTING.md](CONTRIBUTING.md),
[GOVERNANCE.md](GOVERNANCE.md) and the [Code of Conduct](CODE_OF_CONDUCT.md).
Security issues: [SECURITY.md](SECURITY.md).

## License

Licensed under either of

- Apache License, Version 2.0 ([LICENSE-APACHE](LICENSE-APACHE))
- MIT license ([LICENSE-MIT](LICENSE-MIT))

at your option.

Unless you explicitly state otherwise, any contribution intentionally
submitted for inclusion in the work by you, as defined in the Apache-2.0
license, shall be dual licensed as above, without any additional terms or
conditions.
