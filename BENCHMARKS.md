# Benchmarks

This is where GoPatina's official benchmark results are published: GoPatina
against equivalent hand-written Rust and plain Go, measured on the
maintainer's test server. How the numbers are produced is described in
[docs/QUALITY.md](docs/QUALITY.md#performance-shadow-rust-and-keep-up) and
[bench/README.md](bench/README.md).

> **No official run has happened yet.** The dedicated test server is being
> set up. The local verification runs below are evidence from the
> maintainer's workstation, published with their raw JSON, but they are not
> official results, and their wall-clock ratios are noisy.

## Latest official run

None yet.

## The goal

Identical speed to equivalent hand-written Rust on every benchmark, built
with the latest stable Rust. That is a goal, not a claim. "Identical" means equivalent within ±0.5% at 5σ: in a two one-sided tests (TOST) equivalence test on paired, interleaved runs, the whole 5σ interval of the ratio lies inside ±0.5%.
How the test works is in [bench/README.md](bench/README.md).

## Latest local equivalence run (not official)

Measured on 2026-11-21 on the maintainer's laptop, at GoPatina commit
`6cb858fff4bc8a1a659ba240a452d6d45ea6a5c0`, with **CPU performance counters**,
which the project lead had enabled for that day. The laptop was in normal use,
with a load average between 11 and 21. Each binary ran 300 times, interleaved
in ABBA order, and each metric went through a TOST equivalence test at a **±1%
margin** (the default at the time) and 5σ. Notes and the reading against ±0.5%
are in the [run notes](bench/results/2026-11-21-local-6cb858f-equiv-counters.md).
Raw results:
[2026-11-21-local-6cb858f-equiv-counters.json](bench/results/2026-11-21-local-6cb858f-equiv-counters.json).

| Benchmark | User instructions, GoPatina / Rust | User cycles | Wall time |
| --- | --- | --- | --- |
| `fib` | 1.0000024 [1.0000018, 1.0000029]: equivalent | 1.069 [1.001, 1.140]: inconclusive | 1.043 [0.977, 1.114]: inconclusive |
| `sieve` | 1.302 [1.302, 1.302]: slower | 1.073 [1.055, 1.092]: slower | 1.067 [1.025, 1.110]: slower |
| `suffix-sum` | 1.0000020 [1.0000015, 1.0000026]: equivalent | 0.992 [0.962, 1.022]: inconclusive | 0.977 [0.880, 1.084]: inconclusive |

- **`fib` and `suffix-sum`** execute the same instructions as the
  hand-written Rust to within 0.0003%. That is equivalent at ±0.5% too. On
  this busy machine, their cycle and wall-time intervals are too wide to
  decide at either margin.
- **`sieve`** executes 1.302 times the instructions and is about 7% slower,
  at 5σ on all three metrics, and so at either margin. The cause is the extra
  bounds check that keeps Go's integer semantics in the generated inner loop.
  Since then, RFC 0006's loop specialization removes it (see below).


## Local wall-clock run and machine code, 2026-11-21 (not official)

One full run of `bench/run.sh --runs 11` on the maintainer's workstation on
2026-11-21, GoPatina commit `6cb858fff4bc8a1a659ba240a452d6d45ea6a5c0`, plus a
comparison of the machine code. Metadata, method and raw JSON are in the
[run notes](bench/results/2026-11-21-local-6cb858f.md).

| Field | Value |
| --- | --- |
| Date | 2026-11-21 |
| GoPatina | `6cb858fff4bc8a1a659ba240a452d6d45ea6a5c0` |
| Machine | Intel Core i7-8650U @ 1.90 GHz, 8 logical CPUs, 60 GiB RAM |
| Operating system | Linux 7.0.0-43-generic x86_64 |
| Go | go version go1.27.1 linux/amd64 |
| Rust | rustc 1.93.0 (254b59607 2026-01-19); latest stable and the pinned toolchain |
| Runs | 11 per binary per case, interleaved after one warm-up; median wall time |
| Raw results | [2026-11-21-local-6cb858f.json](bench/results/2026-11-21-local-6cb858f.json) |

| Benchmark | GoPatina / Rust | Plain Go / Rust | GoPatina / Go | Machine code against hand-written Rust |
| --- | --- | --- | --- | --- |
| `fib` | 0.730 | 1.159 | 0.630 | Hot function identical |
| `sieve` | 1.123 | 0.966 | 1.163 | One extra bounds check per inner-loop pass |
| `suffix-sum` | 1.010 | 1.020 | 0.990 | Loop identical; 3 extra one-time startup instructions (panic hook) |

**The wall-clock ratios are noisy.** The machine was in normal use and each
binary runs for a fraction of a second. `fib`'s ratio has been 0.890, 1.039
and 0.730 in three runs although its hot function is the same machine code as
the hand-written Rust, so the spread is larger than any real difference.
These ratios cannot show whether GoPatina is identical to, faster or slower
than Rust; a statistical equivalence test with many runs and measured noise,
on a quiet machine, is needed for that.

The machine-code comparison is the clearer evidence: `fib` and `suffix-sum`
compile to the same hot loops as the hand-written Rust. `sieve` does not: Go's
`int` becomes a signed `i64` indexed as `j as usize`, where the hand-written
Rust uses `usize`, so the generated inner loop keeps a bounds check the
hand-written one does not. That is the next gap to close.

## Machine code after RFC 0006 (2026-11-21, not a speed measurement)

`bench/asm-check.sh` on the branch that implements
[RFC 0006](docs/rfcs/0006-proven-ranges.md), rustc 1.93.0:

| Benchmark | Function | GoPatina | Hand-written Rust | Difference |
| --- | --- | --- | --- | --- |
| `fib` | `main::fib` | 30 instructions | 30 instructions | identical |
| `sieve` | `main::main` | 58 instructions | 56 instructions | the 3-instruction startup panic-hook setup, and one alignment no-op |
| `suffix-sum` | `main::main` | 128 instructions | 125 instructions | the 3-instruction startup panic-hook setup |

`sieve`'s inner loop no longer has the extra bounds check: the nest is
specialized for `n <= 3037000500`, where `i*i` cannot wrap, and its hot loop
is the same instructions as the hand-written Rust. This compares code, not
speed; the equivalence run on the test server is pending.

## Earlier local verification runs (not official)

Two runs on 2026-11-18, GoPatina commit `71630d17ab9bf38de6e7602e24807c859a206c70`,
same workstation and toolchains, before the idiomatic-emission change (#31).
[Run notes](bench/results/2026-11-18-local-71630d1.md), raw results:
[run 1](bench/results/2026-11-18-local-71630d1.json),
[run 2](bench/results/2026-11-18-local-71630d1-repeat.json).

| Benchmark | GoPatina / Rust, run 1 | GoPatina / Rust, run 2 | Plain Go / Rust, run 1 / 2 | GoPatina / Go, run 1 / 2 |
| --- | --- | --- | --- | --- |
| `fib` | 0.890 | 1.039 | 2.043 / 1.885 | 0.436 / 0.551 |
| `sieve` | 1.018 | 1.043 | 0.942 / 1.006 | 1.080 / 1.036 |
| `suffix-sum` | 1.042 | 1.028 | 1.024 / 1.029 | 1.018 / 0.999 |

`suffix-sum` exercises the read-only `s[i:]` path: it fills a large `[]int`,
passes `s[start:]` to a one-argument proven reader and checks a checksum.

## Future official runs

When the dedicated test server exists, results will be shown in this form:

| Field | Value |
| --- | --- |
| Date | YYYY-MM-DD |
| GoPatina | version and full commit sha |
| Machine | CPU, cores, memory, operating system |
| Go | output of `go version` |
| Rust | output of `rustc --version` (latest stable at the time of the run) |
| Raw results | link to `bench/results/<date>-<gopatina version>.json` |

| Benchmark | GoPatina `-patina` (median) | Hand-written Rust (median) | Plain Go (median) | GoPatina / Rust | GoPatina / Go |
| --- | --- | --- | --- | --- | --- |
| *(every benchmark in the suite, one row each)* | | | | | |

## History

- 2026-11-21: a local equivalence run with CPU counters, a wall-clock run and a machine-code comparison (not official).
- 2026-11-18: two local verification runs (not official).

Official runs: none yet.
