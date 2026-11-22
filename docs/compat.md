# Go feature support in patina mode

This matrix lists Go language features and their status in patina mode
(`-patina`, `-emit-rust` and `-patina=check`). Default mode supports
everything the installed Go toolchain supports, because it is the Go
toolchain.

Phase 1 supports a defined subset, specified in
[patina-subset.md](patina-subset.md). A feature that is not supported is
rejected with a message naming it, and the Go version that introduced it
where that helps ([RFC 0003](rfcs/0003-toolchain-compatibility.md#the-go-side)).
The "Planned phase" column refers to the roadmap in
[RFC 0001](rfcs/0001-gopatina.md#roadmap).
Where Go and Rust semantics differ, and the plan for each difference, is in
[go-rust-disconnects.md](go-rust-disconnects.md).

Status values: **supported** (implemented and tested), **partial**
(supported within the limits in the notes), **planned** (on the roadmap),
**open** (needs an RFC), **excluded** (not planned).

| Feature | Introduced | Status | Planned phase | Notes |
| --- | --- | --- | --- | --- |
| Functions, multiple return values, named results | Go 1.0 | supported | 1 | Not variadic functions |
| Integers, booleans, strings | Go 1.0 | supported | 1 | Every integer type (RFC 0008), wrapping on overflow as in Go; shifts by a constant or a run-time count, with Go's results and panic |
| Floating point | Go 1.0 | partial | 1 | `float64` and `float32`, printed as Go prints them; float-to-integer conversions as gc does them (measured on amd64; arm64 untested); `complex64`/`complex128` are planned |
| Structs, methods | Go 1.0 | supported | 1 | Value and pointer receivers; not embedded fields |
| `if`, `for`, `switch` | Go 1.0 | supported | 1 | Not `fallthrough`, labels, or a `break` before the end of a case |
| Constants, `iota` | Go 1.0 | supported | 1 | Typed by named types too |
| Named types, type aliases | Go 1.0, Go 1.9 | partial | 1 | Of the basic and slice types and of structs, with methods, conversions and `fmt`'s `String` and `Error` (RFC 0007); not of maps, functions, channels, interfaces, pointers or arrays, and not declared inside a function |
| Pointers | Go 1.0 | partial | 1, 2 | As borrows (`&x`, `&T{}`, parameters, receivers); not stored or returned |
| Slices | Go 1.0 | partial | 1, 2 | `make`, `append`, `len`, indexing, range, and [read-only `s[i:]` arguments](patina-subset.md#read-only-slice-suffix-arguments); not general slicing or `cap` |
| Arrays | Go 1.0 | planned | 2 | |
| Maps | Go 1.0 | planned | 2 | |
| Closures | Go 1.0 | planned | 2 | Captures follow ownership rules |
| `defer` | Go 1.0 | planned | 2 | |
| `error` values | Go 1.0 | partial | 1 | Lowered to `Result` for the patterns in [patina-subset.md](patina-subset.md#errors) |
| `panic`, `recover` | Go 1.0 | partial | | `panic` with a string exits with status 2, as in Go; `recover` is open |
| Interfaces, type switches | Go 1.0 | open | | RFC 0001 open question 2; `io.Writer` parameters are supported |
| Goroutines, channels, `select` | Go 1.0 | planned | 3 | RFC 0002 |
| `sync` package | Go 1.0 | planned | 3 | RFC 0002 |
| Generics (type parameters) | Go 1.18 | open | | RFC 0001 open question 6 |
| `min`, `max` built-ins | Go 1.21 | partial | 1 | Integers only; float calls are rejected, as Rust's NaN rules differ |
| `clear` built-in | Go 1.21 | planned | 2 | |
| Per-iteration loop variables | Go 1.22 | supported | 1 | Without closures the difference cannot be observed |
| Range over integers | Go 1.22 | supported | 1 | |
| Range over functions (iterators) | Go 1.23 | open | | |
| Generic type aliases | Go 1.24 | open | | |
| Standard library | Go 1.0 | partial | 1, 4 | `fmt` printing, a few `strings` functions, `errors.New`, `os.Exit`, `os.Stdout`, `os.Stderr`, `slices.Clone`, `bytes.Clone` |
| `unsafe` | Go 1.0 | excluded | | RFC 0001 open question 7 |
| `reflect` | Go 1.0 | excluded | | |
| cgo | Go 1.0 | excluded | | Rust crate interop instead (phase 4) |
| `runtime.SetFinalizer`, `runtime.GC` | Go 1.0 | excluded | | No garbage collector |

This list grows with each Go release; adding a row for every new language
feature is part of supporting that release.
