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
  results/       <date>-<gopatina version>.json and a summary, one pair per
                 official run (published through a pull request)
```

## The suite (planned)

Small, well-known programs that exercise different parts of the language,
added as the patina subset grows to support them:

| Case | Exercises | Earliest phase |
| --- | --- | --- |
| `fib` | function calls, integer arithmetic | 1 |
| `nbody` | structs, floating point, loops | 1 |
| `spectral-norm` | slices, nested loops | 2 |
| `fannkuch-redux` | slices, permutations, mutation through borrows | 2 |
| `binary-trees` | allocation and freeing of many small boxes (where a GC would run) | 2 |
| `json-roundtrip` | strings, maps, the standard library subset | 4 |
| `chan-pipeline` | goroutines and channels | 3 |

The Rust versions are written to be idiomatic and safe (no `unsafe`), as a
Rust developer would write them, not tuned to make GoPatina look good.

## The harness (planned)

For each case, `run.sh` will:

1. Check that the Go version, run with `go run`, and the Rust version print
   the same `expected` output, so only equivalent programs are compared.
2. Build two binaries: `gopatina build -patina` (the generated crate,
   built by rustc in release mode) and the hand-written Rust crate with
   `cargo build --release`. Plain `go build` is measured too, for reference.
3. Run each binary many times, interleaved and with warm-up runs discarded,
   using [hyperfine](https://github.com/sharkdp/hyperfine), and record wall
   time and peak resident memory.
4. Write one JSON result with, per case and binary: the median, the spread,
   the ratios to hand-written Rust and to plain Go, and the environment:
   GoPatina commit, `go version`, `rustc --version` (latest stable via
   `rustup toolchain install stable`), CPU, operating system, date.

## Where it runs

**Per pull request (GitHub-hosted runners): ratios only.**
[`.github/workflows/bench.yml`](../.github/workflows/bench.yml) runs on pull
requests that touch code generation or this directory, on releases, and on
demand. When cases exist, it installs the latest stable Rust, records
`rustc --version`, builds and runs GoPatina, hand-written Rust and plain Go in
the same job, and uploads the medians and ratios. Shared runners are noisy,
so it reports no absolute numbers. While `bench/cases/` and `bench/run.sh` do
not exist, it skips those steps and says so in a notice.

**Official numbers (the maintainer's test server, outside GitHub).** Release
and published numbers come only from a dedicated test server run by the
maintainer, on code already merged into `main` with the maintainer's
approval. It is never triggered by a pull request. Before each run it checks
the machine's load and that no other CI job is running, and waits until the
machine is quiet. Every official run is the **full suite**, every
benchmark.

**Publishing.** Each official run produces
`results/<date>-<gopatina version>.json` and a human-readable summary table,
with the machine, Go and Rust versions, GoPatina commit and date. They are
proposed in a pull request that the maintainer approves like any other
change. Once merged, the numbers are shown in
[`BENCHMARKS.md`](../BENCHMARKS.md) (latest run in full, history below), in
the README's Benchmarks section, and in the GitHub release notes for that
version, each linking to the raw results file.

This runner does not exist yet, and **nothing has been measured yet**.
