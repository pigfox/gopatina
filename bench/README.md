# Benchmarks: GoPatina, hand-written Rust and plain Go

Each case is one Go program, built two ways (`gopatina build -patina` and
plain `go build`), and the same program written by hand in Rust. Official
results are in [`results/`](results) and [`BENCHMARKS.md`](../BENCHMARKS.md).

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
| `fib` | function calls, integer arithmetic: `fib(42)` by plain recursion | 1 (in place) |
| `sieve` | a large slice, nested loops, element writes | 1 (in place) |
| `suffix-sum` | a read-only `s[i:]` view passed to a proven reader, slice fill and range | 2 (read-only slice suffix) |
| `nbody` | structs, floating point, loops (needs `math.Sqrt`) | 2 |
| `spectral-norm` | slices, nested loops | 2 |
| `fannkuch-redux` | slices, permutations, mutation through borrows | 2 |
| `binary-trees` | allocation and freeing of many small trees through pointers (where a GC would run); hand-written Rust uses `Box` | RFC 0013 (in place) |
| `cycles` | doubly linked lists built and dropped, their nodes in one slice and linked by index, as one owner per object requires (RFC 0017) and as Rust programmers write them; hand-written Rust does the same with `Vec` and `Option<usize>` | RFC 0017 (in place) |
| `json-roundtrip` | strings, maps, the standard library subset | 4 |
| `chan-pipeline` | goroutines and channels | 3 |

The Rust versions are written to be idiomatic and safe (no `unsafe`), as a
Rust developer would write them, not tuned to make GoPatina look good.

**What each case measures**, against plain Go:

- `fib`: allocates nothing, so it measures code generation (gc against
  LLVM) on calls and integer arithmetic, not the garbage collector.
- `sieve`: one allocation, then a loop of element writes: bounds checks and
  loop code generation, not the collector.
- `suffix-sum`: one allocation, then reads through `s[i:]` views: bounds
  checks and slicing, not the collector.
- `binary-trees`: millions of small allocations and frees: Go's collector
  against `Box` (GoPatina and hand-written Rust). This is the case where
  memory management decides.
- `cycles`: doubly linked lists built and dropped. Until 2026-11-23 their
  nodes linked by pointer, both ways, which GoPatina kept in
  reference-counted cells it never freed (62 times the hand-written Rust's
  peak memory). Under one owner per object (RFC 0017) a node cannot have
  two names, so the program keeps the nodes in one slice and links them by
  index, the way Rust programmers write such a list, and the hand-written
  Rust does the same. Peak memory and time are the point: one slice per
  list, freed when it is dropped.

**Why `fib(42)` and not `fib(1000)`.** A case should run for a few seconds,
so that start-up time is a small part of what is measured. `fib(42)` takes
about two seconds in Rust. Plain recursion makes 2 × fib(n + 1) − 1 calls:
866,988,873 for `fib(42)`, and about 10^209 for `fib(1000)`, which no
computer could finish. (`fib(1000)` would also overflow `int64`: it has 209
digits.)

## The harness

For each case, [`run.sh`](run.sh):

1. Builds three binaries: `gopatina build -patina` (the generated crate,
   built by rustc in release mode), the hand-written Rust crate with
   `cargo build --release`, and plain `go build` (gc, default flags; the
   `go version` is recorded).
2. Checks that all three print the case's `expected` output, so only
   equivalent programs are compared.
3. Runs the three many times (11 by default), interleaved, after a warm-up
   run each, and takes the median wall time of each.
4. Writes one JSON result with, per case, the ratios GoPatina/Rust,
   Go/Rust and GoPatina/Go, and the environment: GoPatina commit,
   `go version`, both `rustc --version`s, operating system. It writes no
   absolute times.

Each binary also runs once under GNU time, and the result has the ratios of
their peak resident memory to the hand-written Rust's (null without GNU
time).

## Equivalence test: `run.sh --equiv`

The goal is identical speed to equivalent hand-written Rust, and a median of
11 runs cannot show that: on a busy machine the spread between runs is
larger than the differences being looked for. `bench/run.sh --equiv` builds
and checks the same binaries, then runs
[`cmd/gopatina-bench`](../cmd/gopatina-bench) on the three for each case:

1. Runs the three binaries N times each (`--runs`, default 400) after two
   warm-up runs each, in ABCCBA order (A B C, C B A, A B C, ...), so slow
   drift in the machine affects all alike. Every pair of arms is measured
   in the same round.
2. For each pair (GoPatina / Rust, Go / Rust, GoPatina / Go), takes the log
   of each round's ratio, for example ln(GoPatina / Rust), and computes a
   (1 - 2α) confidence interval of its mean with Student's t. α is the
   one-sided normal tail beyond `--sigma` standard deviations (default 5,
   α ≈ 2.87e-7).
3. Runs the two one-sided tests (TOST) against the equivalence margin
   (`--margin`, default 0.005, that is ±0.5%, the project lead's choice).
   "Identical speed" means equivalent within ±0.5% at 5σ. The verdict is **equivalent** if the whole interval lies
   inside ±margin, **slower** or **faster** if it lies wholly outside, and
   **inconclusive** otherwise: more runs or a quieter machine are needed.

**How many runs.** The interval's half-width is about t × sd / √N, with
t ≈ 5 at 5σ and sd the standard deviation of the log ratios (in the report).
To call two programs equivalent, it must fit inside the margin less their
true difference, so N ≈ (5 × sd / (ln 1.005 − |mean|))². At sd = 1%
and no true difference, that is about 100 runs; at sd = 2%, about 400;
at sd = 5%, about 2,500. Measure sd with a short run first (say 100 runs),
then size the real one.

**One rustc for both.** Both Rust binaries are built by the same rustc,
the toolchain `--rust` names (default `stable`): GoPatina's crate through a
`RUSTUP_TOOLCHAIN` override of the rustc it pins. This is `--toolchain same`,
the default and the official comparison. Different rustc releases compile the
same source differently: on 2026-11-22, the hand-written `fib` and `sieve`
compiled to different machine code under rustc 1.93.0 and 1.100.0, while
GoPatina's hot loops matched the hand-written ones under each. A comparison
across releases would measure the compilers. `--toolchain shipped` builds
GoPatina's crate with its pinned rustc instead, what a user gets by default,
and is reported only as a secondary comparison. The harness reads which
rustc built each binary from its `.comment` section, records both, and in
`same` mode stops if they differ. `bench/asm-check.sh --toolchain T`
compares the machine code under toolchain T.

The default metric is wall time. `--metric perf` also counts user
instructions and user cycles with `perf stat`, each with its own test; it
needs the kernel to allow performance counters (`kernel.perf_event_paranoid`
of 2 or lower) and is off by default. Each process's start-up time is part of
what is measured, the same for both binaries, so a short benchmark dilutes a
difference; cases should run for a large fraction of a second or more. The
JSON report lists, per case and metric, the ratio, the interval, the
standard deviation of the log ratios, the margin, sigma and the verdict,
for every pair. For each arm on its own it lists each metric's geometric
mean with its interval at the same sigma, the binary's size in bytes (as
each tool builds it by default, not stripped), and its peak resident
memory, with a test per pair like the other metrics. Peak memory comes
from 20 more runs of each arm under GNU time (`-memruns`), not under
`perf`. A Go program can't measure its child's peak itself: Go starts
children sharing its own memory, and Linux keeps that high-water mark
across `exec`, so every child would read at least the harness's own
size. Without `/usr/bin/time` the report leaves memory out, with a note.

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

The first official runs were on 2026-11-22 ([`results/`](results)).
