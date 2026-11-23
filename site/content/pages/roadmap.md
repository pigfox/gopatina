# GoPatina roadmap: from a Go subset to goroutines

GoPatina grows phase by phase. Each phase starts with an RFC for its rules, ships with 100% unit-test coverage and its own required test layers, and is measured against hand-written Rust.

## Phases {#phases}

| Phase | Scope | Status |
| --- | --- | --- |
| 0 | The RFCs; the `gopatina` command with default-mode pass-through | Done (v0.0.1) |
| 1 | `-emit-rust`, `-patina`, `-patina=check` and `gopatina verify` for the [phase 1 subset](https://github.com/pigfox/gopatina/blob/main/docs/patina-subset.md), with the borrow checker and the Rust code generator | Done (unreleased) |
| 2 | The rest of the borrow checker: maps, closures, `defer`, general slicing, `gopatina fix`, the strict mode | Read-only slice suffix arguments and proven loop specialization done; rest planned |
| 3 | Goroutines and channels on a multi-threaded async runtime, with shared-ownership and mutex fixes | Planned |
| 4 | A standard library subset and Rust crate interop | Planned |

## The standard library: at least TinyGo {#stdlib}

"Okay, so this is the minimum target of GoPatina's coverage. Anything below TinyGo's coverage is unacceptable." (the project lead, 2026-11-23). TinyGo 0.44.0 imports 163 of the Go standard library's 172 packages and passes the tests of 101. Measured with the same yardstick on 2026-11-23, GoPatina imports 37 and passes the tests of none, because it has no test mode yet: below the target on both counts. The order of work that raises them fastest:

1. Read each standard library package one way, from source or from compiled export data, so that one type never arrives twice: this blocks 40 packages first.
2. Function types and values (18 packages), after an RFC on how closures own what they capture.
3. Arrays and indexed slice literals (19 packages).
4. `io.Writer` fields, `io.Reader`, and provided packages such as `time.Time` (16 packages).
5. A patina test mode, `gopatina test -patina`, so packages' own tests can run through GoPatina.

The method, every package's result and how GoPatina's test differs from TinyGo's are in [docs/stdlib](https://github.com/pigfox/gopatina/blob/main/docs/stdlib/README.md).

## Measuring development speed {#development-speed}

GoPatina's goal is Go's development speed with Rust's output, but no study has measured development time yet. The experiment, approved on 2026-11-22 and now being designed: the same specified programs are written twice, in Rust by hand and in Go with GoPatina, in a randomized order, each against the same acceptance tests. For each version it records the time to all tests passing, the compile-and-fix cycles, the ownership errors met, the lines of code, and the result's run time and memory. Every run will be published, including the ones Rust wins; a run by an AI agent will be reported as such, not as human productivity; and human developers will be invited to run the same tasks. Until results exist, GoPatina makes no claim of a development-time number.

## Optimizations {#optimizations}

- **Single-owner pointers as `Box`.** A pointer that escapes is a reference-counted cell today, where hand-written Rust would often use a `Box`; the [`binary-trees` benchmark](/benchmarks#results) measures the cost. Where the analysis proves a pointer has one owner, it can be a `Box` instead.

## The design documents {#rfcs}

- [RFC 0001](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0001-gopatina.md): the design, the borrow checker, and why Rust underneath.
- [RFC 0002](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0002-concurrency.md): goroutines, channels and `select` in patina mode.
- [RFC 0003](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0003-toolchain-compatibility.md): compatibility with Go and Rust releases.
- [RFC 0004](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0004-read-only-slice-suffix.md): read-only slice suffix arguments.
- [RFC 0005](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0005-phase1-semantic-exactness.md): phase 1 semantic exactness fixes.
- [RFC 0006](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0006-proven-ranges.md): proven integer ranges and loop specialization.

What each phase is measured by is in [the evidence](/evidence); what works today is in [how it works](/how-it-works#subset).
