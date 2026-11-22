# How GoPatina compiles Go to Rust

GoPatina compiles a defined subset of Go to Rust: you write ordinary Go, GoPatina's own borrow checker works out ownership in Go terms, and the program is translated into readable Rust that rustc builds into a native binary with no garbage collector. Without a patina flag, GoPatina is plain Go.

## What you write on top {#on-top}

Ordinary Go, formatted by `gofmt` and type-checked by Go's own libraries. Patina code is always valid Go, so the plain Go toolchain can always build it too. A package can keep a patina version beside a plain one with the `//go:build patina` constraint.

## Compiling Go to Rust, underneath {#underneath}

Both patina flags translate the Go program into readable Rust. `-emit-rust` stops there and writes a cargo crate you can keep and edit. `-patina` goes on to build that crate with rustc, so the result is a Rust program: no garbage collector and memory safe. rustc and LLVM do the optimising, and Rust's own borrow checker verifies every program GoPatina writes. Calling Rust crates directly is planned.

Where GoPatina can prove that a loop's integer arithmetic cannot wrap, it writes the loop the way hand-written Rust would ([RFC 0006](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0006-proven-ranges.md)), while keeping Go's behaviour exactly; the [benchmarks](/benchmarks) show what that does to the machine code.

## A borrow checker for Go {#borrow-checker}

Before any Rust is written, GoPatina's own borrow checker runs on the Go code. It decides who owns each value and which values are moved, copied or borrowed, and applies fixes that keep the program's meaning. Phase 1 implements moves, copies of strings and small structs, and inferred shared and exclusive borrows; shared ownership and mutexes for values shared across goroutines come with goroutines. Every decision is reported with `-explain`; `gopatina fix`, which will write fixes back into the Go source, is planned. When no safe fix exists, the error is in Go terms. This is real output for the project's ownership example:

```text
./examples/ownership/main.go:33:2: m.Body is changed after m was handed to out.Queue on line 32
        out.Queue(m)     // line 32: out.Queue keeps m
        m.Body[0] = 'D'  // line 33: this also changes the copy out.Queue kept, which shares m.Body's memory
    patina mode does not copy m on its own: a copy would not share that memory, so the program could behave differently
    if out.Queue should keep the original, hand it a copy: Message{To: m.To, Body: bytes.Clone(m.Body)}
    or finish using m before handing it over
```

## Go without a garbage collector {#no-gc}

In patina mode memory is managed the way Rust manages it: each value has one owner and is freed when its owner goes out of scope, so there is no garbage collector and no collection pause. The borrow checker above is what makes that safe for Go code that was written for a garbage collector.

## What phase 1 supports {#subset}

The patina modes accept a defined subset of Go, specified in [docs/patina-subset.md](https://github.com/pigfox/gopatina/blob/main/docs/patina-subset.md):

- programs of several packages: `package main` and the packages it imports, of its own module, other modules and the standard library, translated from their Go source (only what `main` reaches), with functions, methods, named struct types, named types such as `type Op int` and `type Path []int` (with methods, and printed through `String` and `Error` as `fmt` does), and type aliases
- every integer type (`int8` to `int64`, `uint8` to `uint64`, `int`, `uint`, `uintptr`), `float32`, `float64`, `bool`, `string`, and slices of supported types, including read-only `s[i:]` suffix arguments
- package-level variables, initialized in Go's order
- interfaces: interface types and `any`, method calls through them, comparisons, type assertions and type switches, with Go's panics; an interface value is one Rust enum of the program's dynamic types, so a call is a `match`, with no trait objects
- `time.Duration`, with its constants, arithmetic and methods (`String`, `Seconds`, `Round`, `Truncate` and the rest), written in Rust with Go's exact behaviour
- `if`, `for` (including `for range` over slices, strings and integers), `switch`, constants and `iota`
- errors as Go values: `errors.New`, `fmt.Errorf` (with `%w`) and custom error types, kept, compared, printed and switched on, `errors.Is`, `errors.As` and `errors.Unwrap`, with functions returning Rust's `Result`; and pointers used as borrows
- `fmt` printing (`Println`, `Printf`, `Sprintf`, `Fprintf` and the rest), the common functions of `strings` (`Split`, `Cut`, `ToLower`, `ToUpper`, `EqualFold`, `ReplaceAll`, `TrimPrefix`, `TrimSuffix` and more, with case mapping by Go's own Unicode tables), and a few functions from `slices`, `bytes` and `os`

**Not supported yet:** goroutines and channels, maps, closures and function values, package-level error variables, pointers inside interfaces, generics, `defer`, general slicing (`s[i:j]`), slices with an explicit high or max bound, saved slice views, and most of the standard library. Every unsupported construct is rejected with a Go-style error that names the construct; nothing is translated approximately. See the [feature matrix](https://github.com/pigfox/gopatina/blob/main/docs/compat.md) and [every known place where Go and Rust semantics differ](https://github.com/pigfox/gopatina/blob/main/docs/go-rust-disconnects.md).

## Goroutines {#goroutines}

Goroutines are not supported yet; this part is designed, not built. In patina mode, goroutines are designed to run as tasks on Tokio's multi-threaded runtime, the closest match to Go's own scheduler. Channels map to Tokio channels and `select` to `tokio::select!`. The compiler decides which functions must be `async`, so Go code never says `async` or `await`. Data races, which Go finds only at run time with `-race`, are to become compile errors or be fixed automatically. See the [roadmap](/roadmap).

## Portability {#portability}

A patina binary is native code, so the target machine needs neither Go nor Rust installed. `GOOS` and `GOARCH` choose the Rust target. Phase 1 builds for each platform's default C runtime (the `-gnu` targets on Linux); static musl binaries by default, and cross-compiling with a fetched target, are planned.

## Compatibility as Go and Rust evolve {#compatibility}

Default mode always uses your installed Go, so new Go releases work there immediately. Patina modes support the two latest Go releases and refuse, by name, any Go feature they do not support yet; they never miscompile silently. Generated Rust uses stable Rust only, a declared edition (2024) and a minimum supported Rust version (1.85), and CI tests against Go and Rust release channels every week.

The full design is in [RFC 0001](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0001-gopatina.md). To try it, see [Get started](/get-started).
