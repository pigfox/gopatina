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
> **GoPatina is at the design (RFC) stage.** It is not a working compiler.
> What exists today is the design, [RFC 0001](docs/rfcs/0001-gopatina.md),
> and a small prototype CLI that passes commands through to the Go toolchain
> and reports that the patina modes are not implemented yet. There are no
> benchmarks, because there is no generated code to measure.

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
gopatina build -patina ./cmd/app     # native binary, no GC        (planned)
gopatina build -emit-rust ./cmd/app  # Rust source you can keep    (planned)
```

- **Default mode** runs the `go` command with the same arguments, streams and
  exit status. Anything `go` accepts, `gopatina` accepts, with the same
  result. This works today.
- **`-emit-rust`** translates a defined subset of Go to readable Rust source,
  a cargo crate a developer can keep, edit and build with cargo, and stops
  there. It is the first mode to be built, because its output can be read by
  people and checked by `rustc` itself.
- **`-patina`** runs the same pipeline and then builds the crate with rustc
  into a native binary that behaves exactly like Rust under the hood: no
  garbage collector, ownership and borrowing, Rust-grade memory safety, and
  direct use of Rust crates.

**Rust underneath, Go on top.** Before any Rust is written, GoPatina's own
**borrow checker** runs on the Go program. It infers ownership, borrows and
lifetimes automatically, and fixes what it safely can: a move instead of a
copy, a clone of a small value, `Rc`/`Arc` for shared values, `Arc<Mutex<T>>`
for values shared across goroutines, a narrower borrow. Every fix is reported
(`gopatina build -patina -explain`), and `gopatina fix` can rewrite the Go
source to make it explicit, like `go fix`. When no safe fix exists, the error
is in Go terms, for example:

```text
./main.go:14:9: variable buf is still used after it was handed to send on line 12
    pass a copy (send(slices.Clone(buf))), or use buf before handing it over
```

rustc's own borrow checker then verifies the generated code as the final
proof.

**Drop-in binaries.** A patina binary is native code, static by default (musl
on Linux), so the target machine needs neither Go nor Rust, just as with a Go
binary, and `GOOS`/`GOARCH` cross-compile as usual. Because patina code is
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
fn describe(name: &str, p: Point) -> String {
    format!("{} at ({}, {})", name, p.x, p.y)
}

fn main() {
    let mut p = Point { x: 1, y: 2 };
    p.scale(3);
    println!("{}", describe("origin-ish", p));
}
```

## Examples

[EXAMPLES.md](EXAMPLES.md) walks through all three modes with three runnable
programs in [`examples/`](examples/): `hello`, `ownership` (a value used
after it was handed off) and `counter` (goroutines and a shared counter).
Default mode works today:

```console
$ gopatina run ./examples/counter
total: 4000
```

For `-emit-rust`, the page shows the Rust crate GoPatina is designed to
generate, including how goroutines become Tokio tasks, and the borrow
checker's `-explain` report; for `-patina=check`, the Go-style errors for a
use after hand-off and a data race. All of that is labelled **planned output
(design, not yet implemented)**, next to the real "not implemented yet"
response of today's CLI.

## Status and roadmap

| Phase | Scope | Status |
| --- | --- | --- |
| 0 | RFC 0001; CLI with default-mode pass-through; patina flags report "not implemented" | **In progress** |
| 1 | `-emit-rust` for a tiny subset (functions, structs, integers, strings); `-patina` builds it with rustc | Planned |
| 2 | The GoPatina borrow checker: ownership and lifetime inference, automatic fixes, Go-style errors | Planned |
| 3 | Goroutines and channels on a multi-threaded async runtime ([RFC 0002](docs/rfcs/0002-concurrency.md)) | Planned |
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
  benchmark, tracked over time: a goal, not a claim. **No numbers exist yet.**

Details: [docs/QUALITY.md](docs/QUALITY.md) and [bench/](bench/README.md).

**Checksum-verified builds.** Emitted Rust is designed to be byte-identical for
the same input, and a `-patina` binary to have the same sha256 as plain
`cargo build --release --locked` on the emitted crate. CI already checks the
reproducible-build setup on the example crates; see
[docs/REPRODUCIBLE.md](docs/REPRODUCIBLE.md).

## Benchmarks

**No benchmarks have been run yet**: GoPatina does not generate code yet.
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
| `main` (Phase 0) | the two latest releases (stable, oldstable) | not used yet | not used yet | 2024 (planned) |

Default mode works with any Go release, because it runs your installed `go`.
Patina modes will refuse Go features they do not support yet, naming the
feature and the Go version that introduced it; see the
[feature matrix](docs/compat.md).

## Quick start (prototype CLI)

Requires Go 1.25 or later.

```sh
go install github.com/pigfox/gopatina/cmd/gopatina@latest

gopatina version
gopatina build ./...            # runs go build ./...
gopatina test ./...             # runs go test ./...
gopatina build -emit-rust .     # exits 2: not implemented yet (see RFC 0001)
gopatina build -patina .        # exits 2: not implemented yet (see RFC 0001)
```

`gopatina` uses the `go` command on your `PATH`, or the one named by the
`GOPATINA_GO` environment variable.

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
