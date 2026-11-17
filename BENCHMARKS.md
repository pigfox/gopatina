# Benchmarks

This is where GoPatina's official benchmark results are published: GoPatina
against equivalent hand-written Rust and plain Go, measured on the
maintainer's test server. How the numbers are produced is described in
[docs/QUALITY.md](docs/QUALITY.md#performance-shadow-rust-and-keep-up) and
[bench/README.md](bench/README.md).

> **No official benchmarks have been run yet.** Phase 1 generates code for a
> subset of Go, and the harness (bench/run.sh, with the fib and sieve cases)
> runs on pull requests, reporting ratios only; those are not results. This
> page will stay empty until the first official run on the test server, and
> no figure appears here without its raw results.

## Latest run

None yet.

When there is one, it is shown here in this form:

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

Times are wall-clock medians over many runs. Ratios below 1.00 mean
GoPatina was faster. The target is GoPatina / Rust within 1.10 on every
benchmark: a goal, not a claim.

## History

Past runs, newest first, each with the same fields and a link to its raw
results. None yet.
