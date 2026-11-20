# GoPatina

> A patina is the thin layer that forms on metal: Go on the surface, Rust underneath.

**GoPatina is an open-source Go toolchain front end in which ordinary Go code can opt into running the way Rust code does: no garbage collector, ownership and borrowing, and Rust-grade memory safety.** Without the flag it is plain Go, and every existing Go program builds exactly as before.

**Mission: Go developers get Rust's performance and safety without the painful Rust learning curve.**

**Status: early. Phase 1 works for a defined subset of Go; real programs need features still in progress.** Default mode works for all Go. For the subset, `-emit-rust`, `-patina`, `-patina=check`, `-explain` and `gopatina verify` are implemented and tested, including read-only `s[i:]` suffix arguments; everything else is rejected with a clear error. No performance figures are published yet.

Last updated 2026-11-18. Canonical page: https://gopatina.org/

## Why GoPatina

The language you write in matters more to you, day to day, than what runs underneath it. Go is small, readable and quick to learn, with fast builds and excellent tooling. Rust gives deterministic memory management with no garbage collector, and catches memory errors and data races at compile time, but it is powerful and painful to learn: ownership, borrowing, lifetimes and `async` are a steep climb.

GoPatina keeps Go on top and puts Rust underneath. You keep writing Go; the compiler does the Rust part. **GoPatina is an experiment by [Pigfox](https://pigfox.com) to get Go developers past Rust's learning curve.** Without a garbage collector a few ownership rules remain, but GoPatina states and checks them in Go terms: no Rust syntax, no lifetime annotations, no `async` keywords to learn.

## Three modes, two flags

| Mode | Flag | Garbage collector | Works with all Go | Output | Status |
| --- | --- | --- | --- | --- | --- |
| Default | none | Yes (Go's) | Yes | A Go binary, built by your installed `go` | Works today |
| Emit Rust | `-emit-rust` | No | No: a defined subset | A readable Rust crate you can keep and build with cargo | Works for the phase 1 subset |
| Patina | `-patina` | No | No: the same subset | A native binary built by rustc from that crate | Works for the phase 1 subset |

A variant, `-patina=check`, runs the same checks and then builds with the Go compiler, for machines where Rust cannot be installed. It is never a silent fallback: you choose it, and the output says which compiler built the binary.

## What phase 1 supports

The patina modes accept a defined subset of Go, specified in [docs/patina-subset.md](https://github.com/pigfox/gopatina/blob/main/docs/patina-subset.md):

- `package main` in one directory, with functions, methods and named struct types
- `int`, `int64`, `int32` (`rune`), `byte`, `float64`, `bool`, `string`, and slices of supported types, including read-only `s[i:]` suffix arguments
- `if`, `for` (including `for range` over slices, strings and integers), `switch`, constants and `iota`
- `error` results with `errors.New` and `fmt.Errorf`, and pointers used as borrows
- `fmt` printing (`Println`, `Printf`, `Sprintf`, `Fprintf` and the rest) and a few functions from `strings`, `slices`, `bytes` and `os`

**Not supported yet:** goroutines and channels, maps, closures and function values, interfaces other than `error` and `io.Writer`, generics, `defer`, general slicing (`s[i:j]`), slices with an explicit high or max bound, saved slice views, and most of the standard library. A narrow read-only suffix view, `read(s[i:])`, is supported when passed to a proven local reader. Every unsupported construct is rejected with a Go-style error that names the construct; nothing is translated approximately. See the [feature matrix](https://github.com/pigfox/gopatina/blob/main/docs/compat.md) and [every known place where Go and Rust semantics differ](https://github.com/pigfox/gopatina/blob/main/docs/go-rust-disconnects.md).

**How it is checked.** Each of the 79 test programs in the project's golden suite is built both ways in CI, with `go build` and with `gopatina build -patina`, and both binaries must print the same output, the same first stderr line and exit with the same status; 24 rejection fixtures and 3 Go-only semantics counterexamples pin down what must be refused. Emitted Rust is byte-identical for the same input, and `gopatina verify` checks that a `-patina` binary has the same sha256 as a plain `cargo build --release --locked` of the emitted crate.

## How it works

### What you write on top

Ordinary Go, formatted by `gofmt` and type-checked by Go's own libraries. Patina code is always valid Go, so the plain Go toolchain can always build it too. A package can keep a patina version beside a plain one with the `//go:build patina` constraint.

### What happens underneath

Both patina flags translate the Go program into readable Rust. `-emit-rust` stops there and writes a cargo crate. `-patina` goes on to build that crate with rustc, so the result is a Rust program: no garbage collector and memory safe. Calling Rust crates directly is planned. rustc and LLVM do the optimising, and Rust's own borrow checker verifies every program GoPatina writes.

### The GoPatina borrow checker

Before any Rust is written, GoPatina's own borrow checker runs on the Go code. It decides who owns each value and which values are moved, copied or borrowed, and applies fixes that keep the program's meaning. Phase 1 implements moves, copies of strings and small structs, and inferred shared and exclusive borrows; shared ownership and mutexes for values shared across goroutines come with goroutines. Every decision is reported with `-explain`; `gopatina fix`, which will write fixes back into the Go source, is planned. When no safe fix exists, the error is in Go terms. This is real output for the project's ownership example:

```text
./examples/ownership/main.go:33:2: m.Body is changed after m was handed to out.Queue on line 32
        out.Queue(m)     // line 32: out.Queue keeps m
        m.Body[0] = 'D'  // line 33: this also changes the copy out.Queue kept, which shares m.Body's memory
    patina mode does not copy m on its own: a copy would not share that memory, so the program could behave differently
    if out.Queue should keep the original, hand it a copy: Message{To: m.To, Body: bytes.Clone(m.Body)}
    or finish using m before handing it over
```

### Goroutines

Goroutines are not supported yet; this part is designed, not built. In patina mode, goroutines are designed to run as tasks on Tokio's multi-threaded runtime, the closest match to Go's own scheduler. Channels map to Tokio channels and `select` to `tokio::select!`. The compiler decides which functions must be `async`, so Go code never says `async` or `await`. Data races, which Go finds only at run time with `-race`, are to become compile errors or be fixed automatically.

### Portability

A patina binary is native code, so the target machine needs neither Go nor Rust installed. `GOOS` and `GOARCH` choose the Rust target. Phase 1 builds for each platform's default C runtime (the `-gnu` targets on Linux); static musl binaries by default, and cross-compiling with a fetched target, are planned.

### Compatibility as Go and Rust evolve

Default mode always uses your installed Go, so new Go releases work there immediately. Patina modes support the two latest Go releases and refuse, by name, any Go feature they do not support yet; they never miscompile silently. Generated Rust uses stable Rust only, a declared edition (2024) and a minimum supported Rust version (1.85), and CI tests against Go and Rust release channels every week.

## Try it today

Requires Go 1.25 or later; `-patina` and `gopatina verify` also need a Rust toolchain. Phase 1 is not released yet, so install from `main`:

```sh
go install github.com/pigfox/gopatina/cmd/gopatina@main

gopatina build ./...                        # runs go build ./...
gopatina build -emit-rust -o crate .        # writes a cargo crate
gopatina build -patina -explain -o app .    # native binary, with every decision
gopatina run -patina .                      # build and run it
gopatina build -patina=check .              # patina checks, then the Go compiler
gopatina verify .                           # reproducible: -patina == plain cargo
```

The patina commands work on programs inside the phase 1 subset. See [the examples](https://github.com/pigfox/gopatina/blob/main/EXAMPLES.md) for the three modes on three small programs: real output for the two that fit the subset, and the goroutine example's planned output clearly labelled.

## Roadmap

| Phase | Scope | Status |
| --- | --- | --- |
| 0 | The RFCs; the `gopatina` command with default-mode pass-through | Done (v0.0.1) |
| 1 | `-emit-rust`, `-patina`, `-patina=check` and `gopatina verify` for the [phase 1 subset](https://github.com/pigfox/gopatina/blob/main/docs/patina-subset.md), with the borrow checker and the Rust code generator | Done (unreleased) |
| 2 | The rest of the borrow checker: maps, closures, `defer`, general slicing, `gopatina fix`, the strict mode | Read-only slice suffix arguments done; rest planned |
| 3 | Goroutines and channels on a multi-threaded async runtime, with shared-ownership and mutex fixes | Planned |
| 4 | A standard library subset and Rust crate interop | Planned |

## Quality policy

Every Go package in the project has 100% unit-test coverage, enforced in CI, and each phase adds its own required test layers: golden tests of generated Rust, compile-and-run tests, rejection and fuzz tests for the borrow checker. Every change, including the maintainer's own, goes through a pull request that the project lead approves.

## Benchmarks

**No official test-server run has happened yet.** Two local verification runs on the maintainer's workstation are published in [BENCHMARKS.md](https://github.com/pigfox/gopatina/blob/main/BENCHMARKS.md) with raw JSON and environment details: all three cases, including the read-only `s[i:]` case, stayed within 10% of hand-written Rust (worst ratio 1.043 across both runs), but the `fib` ratio ranged from 0.890 to 1.039. Those are not official results. The goal is identical speed to equivalent hand-written Rust on every benchmark, built with the latest stable Rust. That is a goal, not a claim. Official numbers will come only from the maintainer's dedicated test server, run on the full suite.

## Frequently asked questions

**What is GoPatina?**
GoPatina is an open-source Go toolchain front end by Pigfox. Without a flag it is plain Go. With an opt-in flag, a defined subset of Go compiles through Rust, with no garbage collector and Rust's memory safety.

**Does GoPatina work today?**
Partly. Default mode works for all Go: it passes every command through to the installed go tool. The Rust-based modes work for the phase 1 subset of Go: functions, structs and methods, slices, strings, errors and fmt printing. Goroutines, maps, closures, interfaces, generics and most of the standard library are not supported yet and are rejected with a clear error.

**Will my existing Go code still work?**
Yes. Without a patina flag, gopatina runs go with the same arguments, so every Go program builds exactly as before. Only code you opt into patina mode must follow its stricter rules.

**Do I need to learn Rust to use GoPatina?**
No. You write Go. Without a garbage collector a few ownership rules remain, but GoPatina's borrow checker infers ownership and borrows, fixes what it safely can, and explains the rest in Go terms.

**How fast is GoPatina?**
Two local verification runs have been published in BENCHMARKS.md: all three cases were within 10% of hand-written Rust, with a worst observed ratio of 1.043. These are not official results; official numbers will come from the maintainer's dedicated test server, measured against hand-written Rust and plain Go.

**What does a machine need to run a patina binary?**
Nothing extra. A patina binary is native machine code, so the target needs neither Go nor Rust installed, just like a Go binary. Phase 1 links against the platform's default C runtime; static binaries are planned.

**Who makes GoPatina, and under what license?**
GoPatina was started by Peter Sjolin as an experiment by Pigfox. It is open source under MIT OR Apache-2.0, and every change goes through a reviewed pull request on GitHub.

## Links

- Source code and README: https://github.com/pigfox/gopatina
- Examples: https://github.com/pigfox/gopatina/blob/main/EXAMPLES.md
- The phase 1 subset: https://github.com/pigfox/gopatina/blob/main/docs/patina-subset.md
- Feature matrix: https://github.com/pigfox/gopatina/blob/main/docs/compat.md
- Where Go and Rust semantics differ: https://github.com/pigfox/gopatina/blob/main/docs/go-rust-disconnects.md
- RFC 0001, the design: https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0001-gopatina.md
- RFC 0002, concurrency: https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0002-concurrency.md
- RFC 0003, toolchain compatibility: https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0003-toolchain-compatibility.md
- RFC 0004, read-only slice suffix arguments: https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0004-read-only-slice-suffix.md
- RFC 0005, phase 1 semantic exactness fixes: https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0005-phase1-semantic-exactness.md
- Benchmarks: https://github.com/pigfox/gopatina/blob/main/BENCHMARKS.md
- Pigfox: https://pigfox.com

This site's server is a Go program built with gopatina's default mode; it uses `net/http` and `embed`, which are outside the phase 1 subset.
