---
title: Go inside a Rust project: one crate, plain cargo
description: A hand-written Rust app whose pricing engine is a crate GoPatina compiled from Go: plain cargo builds, tests and lints it, checked against Go.
---
# Go inside a Rust project

Open [examples/rust-interop](https://github.com/pigfox/gopatina/tree/main/examples/rust-interop) as a Rust developer and you find an ordinary cargo workspace with two crates. `quote` is a command-line application written by hand in Rust: it reads a rules file and an order, and prints a receipt. `rules` is the pricing engine it depends on, with discounts, shipping and tax. That second crate was written in Go and compiled by GoPatina.

## What the Rust side sees

`quote` uses `rules` the way it would use any crate:

```rust
let rules = rules::parse(rules_text)?;
let quote = rules::price(&order, &rules)?;
```

The crate's API, listed in its [api.txt](https://github.com/pigfox/gopatina/blob/main/examples/rust-interop/rules/api.txt), is in Rust types: `parse(text: &str) -> Result<Vec<Rule>, Error>` and `price(lines: &[Line], rules: &[Rule]) -> Result<Quote, Error>`, with plain structs of `String`, `i64` and `Vec`. `Error` prints the Go error's text and works with `?`. There is no GoPatina runtime to install and no foreign function interface: the generated code is safe Rust, and cargo builds it like the rest of the workspace.

## What CI checks

The [rust-interop job](https://github.com/pigfox/gopatina/blob/main/.github/workflows/toolchains.yml) runs on every pull request:

- `cargo build`, `cargo test`, `cargo clippy` with warnings denied, and `cargo fmt` on the workspace, with no GoPatina tooling on the Rust side;
- the `rules` crate regenerated from its Go source, which must match the committed crate byte for byte;
- the Rust `quote` and the [Go original](https://github.com/pigfox/gopatina/tree/main/examples/rust-interop/go/cmd/quote) run on the same seven input sets: two receipts, an empty order, a malformed order line, a malformed rules file, a discount over 100% and a usage error. Their output, error messages and exit status must match, and they do ([interop_test.go](https://github.com/pigfox/gopatina/blob/main/examples/rust-interop/interop_test.go)).

## How it works

A Go package other than `main` now compiles to a Rust library crate ([RFC 0014](https://github.com/pigfox/gopatina/pull/100), implemented in [#102](https://github.com/pigfox/gopatina/pull/102)). Its exported functions, types and constants become public Rust items. GoPatina's borrow checker decides how each argument crosses the boundary: `&str` and `&[T]` when the function only reads it, `String` and `Vec<T>` when it keeps it, and `api.txt` says why for each one it keeps. A Go error result becomes `Result<T, Error>`, and an exported sentinel error becomes a function that returns it, so `e == rules::err_empty()` works as Go's `==` does.

## The limits

- The Go package has to stay inside GoPatina's subset: no maps, goroutines or generics yet.
- RFC 0014 refuses, by name, a library with `init` functions or package variables it changes, and an API with interfaces other than `error`, pointers that escape, or an `error` anywhere but the last result.
- `Error` is opaque: a Rust caller compares it with the package's sentinel errors, tests it with `is`, and reads its text, but cannot match on typed Go errors yet.
- Go's integers wrap, so the workspace sets `overflow-checks = false` for the generated crate in its dev profile; release builds already have it.
- A struct with unexported fields is built in Rust from its `Default` and its public fields, as Go code outside the package builds it from the zero value.

Building the example also found three defects, each fixed with its own golden test: struct field comments that rustfmt aligns differently ([#103](https://github.com/pigfox/gopatina/pull/103)), and library panics, `api.txt` and error parameters ([#104](https://github.com/pigfox/gopatina/pull/104)). The [correctness page](/correctness#defects) lists every defect found so far.
