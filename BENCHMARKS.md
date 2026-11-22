# Benchmarks

This is where GoPatina's official benchmark results are published: GoPatina
against equivalent hand-written Rust and plain Go, measured on the
maintainer's test server. How the numbers are produced is described in
[docs/QUALITY.md](docs/QUALITY.md#performance-shadow-rust-and-keep-up) and
[bench/README.md](bench/README.md).

> **No official run has happened yet.** The dedicated, quiet test server does
> not exist. The local verification runs below are evidence from the
> maintainer's workstation, published with their raw JSON, but they are not
> official results, and their wall-clock ratios are noisy.

## Latest official run

None yet.

## The goal

Identical speed to equivalent hand-written Rust on every benchmark, built
with the latest stable Rust. That is a goal, not a claim.

## Latest local verification run (not official)

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

- 2026-11-21: one local verification run and a machine-code comparison (not official).
- 2026-11-18: two local verification runs (not official).

Official runs: none yet.
