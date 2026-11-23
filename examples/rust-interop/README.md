# Go inside a Rust project

A normal Rust workspace in which one crate was written in Go and compiled
by GoPatina ([RFC 0014](../../docs/rfcs/0014-library-crates.md)):

```
examples/rust-interop/
  Cargo.toml        the workspace
  app/              quote: a Rust command-line application, written by hand
  rules/            the rules engine: a crate GoPatina generated from go/rules
  go/rules/         the rules engine's Go source
  go/cmd/quote/     the same application in Go, to compare against
  testdata/         rules files and orders
```

`quote` reads a rules file and an order, and prints a receipt. Reading
the files, parsing the order and printing are hand-written Rust in
`app/`. Pricing (discounts, shipping, tax) is done by the `rules` crate,
which `app` uses like any other dependency:

```rust
let rules = rules::parse(rules_text)?;
let quote = rules::price(&order, &rules)?;
```

`rules` is ordinary safe Rust. Its API is in
[`rules/api.txt`](rules/api.txt): Rust types (`&str`, `&[Line]`,
`Result<Quote, Error>`), with no GoPatina types or runtime in sight.

## Try it

```sh
cd examples/rust-interop
cargo run -p quote -- testdata/shop.rules testdata/big.order
cargo test
cargo clippy --workspace --all-targets -- -D warnings
```

Plain cargo builds, tests and lints the whole workspace. GoPatina is
needed only to regenerate `rules` after the Go source changes:

```sh
gopatina build -emit-rust -o examples/rust-interop/rules ./examples/rust-interop/go/rules
```

## What CI checks

The `rust-interop` job in `.github/workflows/toolchains.yml`:

1. Runs `cargo build`, `cargo test` and `cargo clippy -D warnings` on the
   workspace, with no GoPatina tooling on the Rust side.
2. Regenerates `rules` from `go/rules` and requires it identical to the
   committed crate.
3. Runs the Rust `quote` and the Go `quote` on the same inputs, and
   requires the same output, errors and exit status (`interop_test.go`).

## Limits

`go/rules` is written in GoPatina's subset of Go: no maps, goroutines or
generics yet, and in a library's API no interfaces other than `error`.
The workspace sets `overflow-checks = false` for `rules` in its dev
profile, because Go's integers wrap. The [subset reference](../../docs/patina-subset.md#libraries)
lists what a library can hold today.
