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

## The design documents {#rfcs}

- [RFC 0001](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0001-gopatina.md): the design, the borrow checker, and why Rust underneath.
- [RFC 0002](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0002-concurrency.md): goroutines, channels and `select` in patina mode.
- [RFC 0003](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0003-toolchain-compatibility.md): compatibility with Go and Rust releases.
- [RFC 0004](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0004-read-only-slice-suffix.md): read-only slice suffix arguments.
- [RFC 0005](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0005-phase1-semantic-exactness.md): phase 1 semantic exactness fixes.
- [RFC 0006](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0006-proven-ranges.md): proven integer ranges and loop specialization.

What each phase is measured by is in [the evidence](/evidence); what works today is in [how it works](/how-it-works#subset).
