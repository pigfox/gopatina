# Benchmarks

This is where GoPatina's official benchmark results are published: GoPatina
against equivalent hand-written Rust and plain Go, measured on the
maintainer's test server. How the numbers are produced is described in
[docs/QUALITY.md](docs/QUALITY.md#performance-shadow-rust-and-keep-up) and
[bench/README.md](bench/README.md).

> **No official run has happened yet.** The dedicated, quiet test server does
> not exist. The local verification runs below are evidence from the
> maintainer's workstation, published with their raw JSON, but they are not
> official results.

## Latest official run

None yet.

## Local verification runs (not official)

Two full runs of `bench/run.sh --runs 11` on the maintainer's workstation,
both on GoPatina commit `71630d17ab9bf38de6e7602e24807c859a206c70`. The
machine was in normal use (8 logical CPUs, load average roughly 2.5–5.0,
browser and background agents active), not the quiet dedicated server the
policy requires. Full metadata and raw files are in the
[run notes](bench/results/2026-11-18-local-71630d1.md).

| Field | Value |
| --- | --- |
| Dates | 2026-11-18 (two runs) |
| GoPatina | `71630d17ab9bf38de6e7602e24807c859a206c70` |
| Machine | Intel Core i7-8650U @ 1.90 GHz, 8 logical CPUs, 60 GiB RAM |
| Operating system | Linux 7.0.0-43-generic x86_64 |
| Go | go version go1.27.1 linux/amd64 |
| Rust | rustc 1.93.0 (254b59607 2026-01-19); latest stable and the pinned toolchain |
| Runs | 11 per binary per case, interleaved after one warm-up; median wall time |
| Raw results | [run 1](bench/results/2026-11-18-local-71630d1.json), [run 2](bench/results/2026-11-18-local-71630d1-repeat.json) |

| Benchmark | GoPatina / Rust, run 1 | GoPatina / Rust, run 2 | Worst observed | Plain Go / Rust, run 1 / 2 | GoPatina / Go, run 1 / 2 |
| --- | --- | --- | --- | --- | --- |
| `fib` | 0.890 | 1.039 | 1.039 | 2.043 / 1.885 | 0.436 / 0.551 |
| `sieve` | 1.018 | 1.043 | 1.043 | 0.942 / 1.006 | 1.080 / 1.036 |
| `suffix-sum` | 1.042 | 1.028 | 1.042 | 1.024 / 1.029 | 1.018 / 0.999 |

The harness records ratios only, not absolute times. The target is
GoPatina / Rust within 1.10 on every benchmark. The worst observed ratio
across both runs is 1.043, so the suite meets the target on this machine, but
the spread (notably `fib`, 0.890 to 1.039) is why a quiet dedicated server is
still needed before these become official figures.

`suffix-sum` is the case that exercises the new read-only `s[i:]` path: it
fills a large `[]int`, passes `s[start:]` to a one-argument proven reader and
checks a checksum. Its GoPatina / Rust ratios were 1.042 and 1.028.

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

- 2026-11-18 — two local verification runs above (not official).

Official runs: none yet.
