# GoPatina

> A patina is the thin layer that forms on metal: Go on the surface, Rust underneath.

**GoPatina is an open-source Go toolchain front end in which ordinary Go code can opt into running the way Rust code does: no garbage collector, ownership and borrowing, and Rust-grade memory safety.** Without the flag it is plain Go, and every existing Go program builds exactly as before.

**Mission: Go developers get Rust's performance and safety without the painful Rust learning curve.**

**Status: design stage.** GoPatina is not a working compiler yet. What exists today is the design, in public RFCs, and a prototype `gopatina` command that passes every command through to the Go toolchain. The Rust-based modes report "not implemented yet". No benchmarks have been run.

Last updated 2026-11-16. Canonical page: https://gopatina.org/

## Why GoPatina

The language you write in matters more to you, day to day, than what runs underneath it. Go is small, readable and quick to learn, with fast builds and excellent tooling. Rust gives deterministic memory management with no garbage collector, and catches memory errors and data races at compile time, but it is powerful and painful to learn: ownership, borrowing, lifetimes and `async` are a steep climb.

GoPatina keeps Go on top and puts Rust underneath. You keep writing Go; the compiler does the Rust part. **GoPatina is an experiment by [Pigfox](https://pigfox.com) to get Go developers past Rust's learning curve.** Without a garbage collector a few ownership rules remain, but GoPatina states and checks them in Go terms: no Rust syntax, no lifetime annotations, no `async` keywords to learn.

## Three modes, two flags

| Mode | Flag | Garbage collector | Works with all Go | Output | Status |
| --- | --- | --- | --- | --- | --- |
| Default | none | Yes (Go's) | Yes | A Go binary, built by your installed `go` | Works today |
| Emit Rust | `-emit-rust` | No | No: a defined subset | A readable Rust crate you can keep and build with cargo | Planned |
| Patina | `-patina` | No | No: the same subset | A native binary built by rustc from that crate | Planned |

A planned variant, `-patina=check`, runs the same checks and then builds with the Go compiler, for machines where Rust cannot be installed. It is never a silent fallback: you choose it, and the output says which compiler built the binary.

## How it works

### What you write on top

Ordinary Go, formatted by `gofmt` and type-checked by Go's own libraries. Patina code is always valid Go, so the plain Go toolchain can always build it too. A package can keep a patina version beside a plain one with the `//go:build patina` constraint.

### What happens underneath

Both patina flags translate the Go program into readable Rust. `-emit-rust` stops there and writes a cargo crate. `-patina` goes on to build that crate with rustc, so the result is a Rust program: no garbage collector, memory safe, and able to call Rust crates directly. rustc and LLVM do the optimising, and Rust's own borrow checker verifies every program GoPatina writes.

### The GoPatina borrow checker

Before any Rust is written, GoPatina's own borrow checker runs on the Go code. It infers ownership, borrows and lifetimes, and automatically applies fixes that keep the program's meaning: a move instead of a copy, a clone of a small value, shared ownership for shared values, a mutex for values shared across goroutines. Every fix is reported with `-explain`, and `gopatina fix` can write it back into the Go source. When no safe fix exists, the error is in Go terms, for example (planned wording; not produced by code yet):

```text
main.go:33:2: m.Body is changed after m was handed to out.Queue on line 32
    if the outbox should keep the original text, queue a copy:
        out.Queue(Message{To: m.To, Body: bytes.Clone(m.Body)})
```

### Goroutines

In patina mode, goroutines are designed to run as tasks on Tokio's multi-threaded runtime, the closest match to Go's own scheduler. Channels map to Tokio channels and `select` to `tokio::select!`. The compiler decides which functions must be `async`, so Go code never says `async` or `await`. Data races, which Go finds only at run time with `-race`, become compile errors or are fixed automatically.

### Portability

A patina binary is native code, static by default (musl on Linux), so it drops in wherever a Go binary for the same system would run, with neither Go nor Rust installed. `GOOS` and `GOARCH` cross-compile as usual.

### Compatibility as Go and Rust evolve

Default mode always uses your installed Go, so new Go releases work there immediately. Patina modes support the two latest Go releases and refuse, by name, any Go feature they do not support yet; they never miscompile silently. Generated Rust uses stable Rust only, a declared edition (2024) and a minimum supported Rust version, and CI tests against Go and Rust release channels every week.

## Try it today

Requires Go 1.25 or later. This is everything that works now:

```sh
go install github.com/pigfox/gopatina/cmd/gopatina@latest
gopatina version
gopatina build ./...           # exactly go build ./...
gopatina build -emit-rust .    # exits 2: not implemented yet
```

See [the examples](https://github.com/pigfox/gopatina/blob/main/EXAMPLES.md) for the three modes on three small programs.

## Roadmap

| Phase | Scope | Status |
| --- | --- | --- |
| 0 | The RFCs; the `gopatina` command with default-mode pass-through | Done (v0.0.1) |
| 1 | `-emit-rust` for a tiny subset (functions, structs, integers, strings); `-patina` builds it with rustc | Planned |
| 2 | The GoPatina borrow checker: ownership and lifetime inference, automatic fixes, Go-style errors | Planned |
| 3 | Goroutines and channels on a multi-threaded async runtime | Planned |
| 4 | A standard library subset and Rust crate interop | Planned |

## Quality policy

Every Go package in the project has 100% unit-test coverage, enforced in CI, and each phase adds its own required test layers: golden tests of generated Rust, compile-and-run tests, rejection and fuzz tests for the borrow checker. Every change, including the maintainer's own, goes through a pull request that the project lead approves.

## Benchmarks

**No benchmarks have been run yet**, because GoPatina does not generate code yet. The goal is to shadow Rust and keep up: within 10% of equivalent hand-written Rust on every benchmark, built with the latest stable Rust. That is a goal, not a claim. Official numbers will come only from the maintainer's dedicated test server, run on the full suite and published in [BENCHMARKS.md](https://github.com/pigfox/gopatina/blob/main/BENCHMARKS.md).

## Frequently asked questions

**What is GoPatina?**
GoPatina is an open-source Go toolchain front end by Pigfox. Without a flag it is plain Go. With an opt-in flag, a defined subset of Go is designed to compile through Rust, with no garbage collector and Rust's memory safety.

**Does GoPatina work today?**
Only its default mode. The gopatina command passes every command through to the installed go tool. The Rust-based modes are designed in public RFCs and report that they are not implemented yet.

**Will my existing Go code still work?**
Yes. Without a patina flag, gopatina runs go with the same arguments, so every Go program builds exactly as before. Only code you opt into patina mode must follow its stricter rules.

**Do I need to learn Rust to use GoPatina?**
No. You write Go. Without a garbage collector a few ownership rules remain, but GoPatina's borrow checker infers ownership and lifetimes, fixes what it safely can, and explains the rest in Go terms.

**How fast is GoPatina?**
Nobody knows yet. GoPatina does not generate code yet, so no benchmarks have been run. Official results will be published in BENCHMARKS.md, measured against hand-written Rust and plain Go.

**What does a machine need to run a patina binary?**
Nothing extra. A patina binary is native machine code, built static by default, so the target needs neither Go nor Rust installed, just like a Go binary.

**Who makes GoPatina, and under what license?**
GoPatina was started by Peter Sjolin as an experiment by Pigfox. It is open source under MIT OR Apache-2.0, and every change goes through a reviewed pull request on GitHub.

## Links

- Source code and README: https://github.com/pigfox/gopatina
- Examples: https://github.com/pigfox/gopatina/blob/main/EXAMPLES.md
- RFC 0001, the design: https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0001-gopatina.md
- RFC 0002, concurrency: https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0002-concurrency.md
- RFC 0003, toolchain compatibility: https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0003-toolchain-compatibility.md
- Benchmarks: https://github.com/pigfox/gopatina/blob/main/BENCHMARKS.md
- Pigfox: https://pigfox.com

This site's server is a Go program built with gopatina's default mode; the Rust modes aren't built yet.
