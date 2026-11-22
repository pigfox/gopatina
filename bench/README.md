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
      hot        functions asm-check.sh compares, one per line
  run.sh         the harness (--equiv: the equivalence test)
  asm-check.sh   the machine-code comparison
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

Peak memory and the official-run format are planned with the test server.

## Equivalence test: `run.sh --equiv`

The goal is identical speed to equivalent hand-written Rust, and a median of
11 runs cannot show that: on a busy machine the spread between runs is
larger than the differences being looked for. `bench/run.sh --equiv` builds
and checks the same binaries, then runs
[`cmd/gopatina-bench`](../cmd/gopatina-bench) on GoPatina against the
hand-written Rust for each case:

1. Runs the two binaries N times each (`--runs`, default 400) after two
   warm-up runs each, in ABBA order (A B, B A, A B, ...), so slow drift in the
   machine affects both alike. Each pair is two runs next to each other.
2. Takes the log of each pair's ratio, ln(GoPatina / Rust), and computes a
   (1 - 2α) confidence interval of its mean with Student's t. α is the
   one-sided normal tail beyond `--sigma` standard deviations (default 5,
   α ≈ 2.87e-7).
3. Runs the two one-sided tests (TOST) against the equivalence margin
   (`--margin`, default 0.01, that is ±1%, a setting until the project lead
   chooses another). The verdict is **equivalent** if the whole interval lies
   inside ±margin, **slower** or **faster** if it lies wholly outside, and
   **inconclusive** otherwise: more runs or a quieter machine are needed.

The default metric is wall time. `--metric perf` also counts user
instructions and user cycles with `perf stat`, each with its own test; it
needs the kernel to allow performance counters (`kernel.perf_event_paranoid`
of 2 or lower) and is off by default. Each process's start-up time is part of
what is measured, the same for both binaries, so a short benchmark dilutes a
difference; cases should run for a large fraction of a second or more. The
JSON report lists, per case and metric, the ratio, the interval, the
standard deviation of the log ratios, the margin, sigma and the verdict.

## Machine-code check: `asm-check.sh`

`bench/asm-check.sh` compiles each case's emitted crate and its hand-written
crate with the same rustc and release flags, disassembles the functions
listed in the case's `hot` file, masks addresses, and reports for each the
instruction counts and how many lines differ. Identical machine code is the
strongest evidence of identical speed and is not affected by noise; a
difference points at the code generation to fix. It is a report, not a
gate.

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
