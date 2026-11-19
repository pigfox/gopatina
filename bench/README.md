# Benchmarks: GoPatina against hand-written Rust

> **Status: no official results yet.** The harness and the first two cases
> exist and run per pull request, reporting ratios only. The test server that
> produces official numbers does not exist yet.

The policy (what is measured, when, against which Rust, and the target) is in
[`docs/QUALITY.md`](../docs/QUALITY.md#performance-shadow-rust-and-keep-up).

## Layout

```
bench/
  cases/
    <name>/
      go/        the Go program (patina subset), with a main package and a test
      rust/      equivalent hand-written Rust, as a cargo crate
      input/     fixed inputs, if the program reads any (none yet)
      expected   the exact output every version must print
  run.sh         the harness
  results/       <date>-<gopatina version>.json and a summary, one pair per
                 official run (published through a pull request)
```

## The suite

Small, well-known programs that exercise different parts of the language,
added as the patina subset grows to support them:

| Case | Exercises | Earliest phase |
| --- | --- | --- |
| `fib` | function calls, integer arithmetic | 1 (in place) |
| `sieve` | a large slice, nested loops, element writes | 1 (in place) |
| `suffix-sum` | a read-only `s[i:]` view passed to a proven reader, slice fill and range | 2 (read-only slice suffix) |
| `nbody` | structs, floating point, loops (needs `math.Sqrt`) | 2 |
| `spectral-norm` | slices, nested loops | 2 |
| `fannkuch-redux` | slices, permutations, mutation through borrows | 2 |
| `binary-trees` | allocation and freeing of many small boxes (where a GC would run) | 2 |
| `json-roundtrip` | strings, maps, the standard library subset | 4 |
| `chan-pipeline` | goroutines and channels | 3 |

The Rust versions are written to be idiomatic and safe (no `unsafe`), as a
Rust developer would write them, not tuned to make GoPatina look good.

## The harness

For each case, [`run.sh`](run.sh):

1. Builds three binaries: `gopatina build -patina` (the generated crate,
   built by rustc in release mode with its pinned toolchain), the
   hand-written Rust crate with `cargo build --release` (the latest stable
   Rust on the machine), and plain `go build`.
2. Checks that all three print the case's `expected` output, so only
   equivalent programs are compared.
3. Runs the three many times (11 by default), interleaved, after a warm-up
   run each, and takes the median wall time of each.
4. Writes one JSON result with, per case, the ratios GoPatina/Rust,
   Go/Rust and GoPatina/Go, and the environment: GoPatina commit,
   `go version`, both `rustc --version`s, operating system. It writes no
   absolute times.

Peak memory, the spread and the official-run format are planned with the
test server.

## Where it runs

**Per pull request (GitHub-hosted runners): ratios only.**
[`.github/workflows/bench.yml`](../.github/workflows/bench.yml) runs on pull
requests that touch code generation or this directory, on releases, and on
demand. When cases exist, it installs the latest stable Rust, records
`rustc --version`, builds and runs GoPatina, hand-written Rust and plain Go in
the same job, and uploads the ratios. Shared runners are noisy, so it
reports no absolute numbers.

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

This runner does not exist yet, and **nothing has been measured officially
yet**.
