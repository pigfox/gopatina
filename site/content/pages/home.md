# GoPatina: write Go, run it like Rust

A patina is the thin layer that forms on metal: Go on the surface, Rust underneath. {.tagline}

**GoPatina is an open-source Go toolchain front end in which ordinary Go code can opt into running the way Rust code does: no garbage collector, ownership and borrowing, and Rust-grade memory safety.** Without the flag it is plain Go, and every existing Go program builds exactly as before. {.definition}

Mission: Go developers get Rust's performance and safety without the painful Rust learning curve. {.mission}

**Status: early. Phase 1 works for a defined subset of Go; real programs need features still in progress.** Default mode works for all Go. For the subset, `-emit-rust`, `-patina`, `-patina=check`, `-explain` and `gopatina verify` are implemented and tested; everything else is rejected with a clear error. No official performance or defect-rate figures yet: [the evidence](/evidence) reports the method and every result so far. {.status}

## Why GoPatina {#why}

The language you write in matters more to you, day to day, than what runs underneath it. Go is small, readable and quick to learn, with fast builds and excellent tooling. Rust gives deterministic memory management with no garbage collector, and catches memory errors and data races at compile time, but it is powerful and painful to learn: ownership, borrowing, lifetimes and `async` are a steep climb.

GoPatina keeps Go on top and puts Rust underneath. You keep writing Go; the compiler does the Rust part. **GoPatina is an experiment by [Pigfox](https://pigfox.com) to get Go developers past Rust's learning curve.** Without a garbage collector a few ownership rules remain, but GoPatina states and checks them in Go terms: no Rust syntax, no lifetime annotations, no `async` keywords to learn.

## Three modes, two flags {#modes}

| Mode | Flag | Garbage collector | Works with all Go | Output | Status |
| --- | --- | --- | --- | --- | --- |
| Default | none | Yes (Go's) | Yes | A Go binary, built by your installed `go` | Works today |
| Emit Rust | `-emit-rust` | No | No: a defined subset | A readable Rust crate you can keep and build with cargo | Works for the phase 1 subset |
| Patina | `-patina` | No | No: the same subset | A native binary built by rustc from that crate | Works for the phase 1 subset |

A variant, `-patina=check`, runs the same checks and then builds with the Go compiler, for machines where Rust cannot be installed. It is never a silent fallback: you choose it, and the output says which compiler built the binary.

## Explore GoPatina {#more}

- [How it works](/how-it-works): how Go is compiled to Rust, GoPatina's borrow checker, and what phase 1 supports.
- [Get started](/get-started): install GoPatina and build a Go program as plain Go, as Rust source, and as a native binary.
- [The evidence](/evidence): GoPatina's speed and correctness claims, reported as a scientific study.
- [Benchmarks](/benchmarks): Go compiled through Rust against hand-written Rust and plain Go.
- [Correctness](/correctness): how every program is checked against Go itself.
- [Roadmap](/roadmap): what is done and what comes next, from maps to goroutines.
- [Frequently asked questions](/faq): short answers, from "does it work today?" to the license.
- [Source code on GitHub](https://github.com/pigfox/gopatina), with the [examples](https://github.com/pigfox/gopatina/blob/main/EXAMPLES.md) and the [design RFCs](https://github.com/pigfox/gopatina/tree/main/docs/rfcs).
