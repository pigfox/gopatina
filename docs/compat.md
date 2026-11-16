# Go feature support in patina mode

This matrix lists Go language features and their status in patina mode
(`-patina` and `-emit-rust`). Default mode supports everything the installed
Go toolchain supports, because it is the Go toolchain.

**Nothing is supported in patina mode yet**: no code generation exists. The
"Planned phase" column refers to the roadmap in
[RFC 0001](rfcs/0001-gopatina.md#roadmap). A feature that is not supported is
rejected with a message naming it and the Go version that introduced it
([RFC 0003](rfcs/0003-toolchain-compatibility.md#the-go-side)).

Status values: **planned** (on the roadmap), **open** (needs an RFC),
**excluded** (not planned).

| Feature | Introduced | Status | Planned phase | Notes |
| --- | --- | --- | --- | --- |
| Functions, multiple return values | Go 1.0 | planned | 1 | |
| Integers, booleans, strings | Go 1.0 | planned | 1 | Overflow semantics: RFC 0001 open question 10 |
| Floating point | Go 1.0 | planned | 1 | |
| Structs, methods | Go 1.0 | planned | 1 | |
| `if`, `for`, `switch` | Go 1.0 | planned | 1 | |
| Pointers | Go 1.0 | planned | 2 | Borrows, inferred by the borrow checker |
| Slices, arrays | Go 1.0 | planned | 2 | Aliasing rules: RFC 0001 open question 3 |
| Maps | Go 1.0 | planned | 2 | |
| Closures | Go 1.0 | planned | 2 | Captures follow ownership rules |
| `defer` | Go 1.0 | planned | 2 | |
| `error` values | Go 1.0 | planned | 2 | Lowered to `Result` where exact |
| `panic`, `recover` | Go 1.0 | open | | |
| Interfaces, type switches | Go 1.0 | open | | RFC 0001 open question 2 |
| Goroutines, channels, `select` | Go 1.0 | planned | 3 | RFC 0002 |
| `sync` package | Go 1.0 | planned | 3 | RFC 0002 |
| Generics (type parameters) | Go 1.18 | open | | RFC 0001 open question 6 |
| `min`, `max`, `clear` built-ins | Go 1.21 | planned | 2 | |
| Per-iteration loop variables | Go 1.22 | planned | 1 | Matches Rust's loop bindings |
| Range over integers | Go 1.22 | planned | 1 | |
| Range over functions (iterators) | Go 1.23 | open | | |
| Generic type aliases | Go 1.24 | open | | |
| `unsafe` | Go 1.0 | excluded | | RFC 0001 open question 7 |
| `reflect` | Go 1.0 | excluded | | |
| cgo | Go 1.0 | excluded | | Rust crate interop instead (phase 4) |
| `runtime.SetFinalizer`, `runtime.GC` | Go 1.0 | excluded | | No garbage collector |

This list grows with each Go release; adding a row for every new language
feature is part of supporting that release.
