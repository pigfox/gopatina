# Benchmarks: GoPatina against hand-written Rust

> **Status: no results yet.** Neither patina mode generates code, so there is
> nothing to measure. This directory describes the harness and the suite that
> will be built in Phase 1, so the policy is in place before the first line of
> code generation lands.

The policy (what is measured, when, against which Rust, and the target) is in
[`docs/QUALITY.md`](../docs/QUALITY.md#performance-shadow-rust-and-keep-up).

## Layout (planned)

```
bench/
  cases/
    <name>/
      go/        the Go program (patina subset), with a main package
      rust/      equivalent hand-written Rust, as a cargo crate
      input/     fixed inputs, if the program reads any
      expected   the exact output both versions must print
  run.sh         the harness
  results/       one JSON file per run (committed for releases only)
```

## The suite (planned)

Small, well-known programs that exercise different parts of the language,
added as the patina subset grows to support them:

| Case | Exercises | Earliest phase |
| --- | --- | --- |
| `fib` | function calls, integer arithmetic | 1 |
| `nbody` | structs, floating point, loops | 1 |
| `spectral-norm` | slices, nested loops | 3 |
| `fannkuch-redux` | slices, permutations, mutation through borrows | 3 |
| `binary-trees` | allocation and freeing of many small boxes (where a GC would run) | 3 |
| `json-roundtrip` | strings, maps, the standard library subset | 5 |
| `chan-pipeline` | goroutines and channels | 4 |

The Rust versions are written to be idiomatic and safe (no `unsafe`), as a
Rust developer would write them, not tuned to make GoPatina look good.

## The harness (planned)

For each case, `run.sh` will:

1. Check that the Go version, run with `go run`, and the Rust version print
   the same `expected` output, so only equivalent programs are compared.
2. Build three binaries: `gopatina build -patina`, `gopatina build
   -emit-rust` followed by `cargo build --release`, and the hand-written Rust
   crate with `cargo build --release`. Plain `go build` is measured too, for
   reference.
3. Run each binary repeatedly (with warm-up runs discarded) using
   [hyperfine](https://github.com/sharkdp/hyperfine), and record wall time
   and peak resident memory.
4. Write one JSON result with, per case and binary: the mean, the standard
   deviation, the ratio to hand-written Rust, and the environment: GoPatina
   commit, `go version`, `rustc --version` (latest stable via
   `rustup toolchain install stable`), CPU, operating system.

## CI

[`.github/workflows/bench.yml`](../.github/workflows/bench.yml) runs on
releases, on pull requests that touch code generation or this directory, and
on demand. When cases exist, it installs the latest stable Rust, records
`rustc --version`, runs the harness and uploads the results. While
`bench/cases/` and `bench/run.sh` do not exist, it skips those steps and says
so in a notice. Shared CI
runners are noisy, so release numbers will come from a dedicated machine,
with the CI job used to catch large regressions.
