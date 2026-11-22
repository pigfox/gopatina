# Test-server equivalence runs with CPU counters — 2026-11-22

The speed bar's runs: GoPatina against hand-written Rust, **both built by the
same rustc, 1.100.0** (`bench/run.sh --toolchain same`, the official
comparison), measured with the CPU's performance counters, which the project
lead enabled on the test server for them.

| Field | Value |
| --- | --- |
| Dates | 2026-11-22: 5σ run 05:31:32 to 05:34:24 UTC; 6σ run 05:37:34 to 05:40:25 UTC |
| GoPatina | `bfed1e17ebec1a43e7acc09c833986bda5d3e031` (main, with #40, #44 and #47) |
| Machine | Intel Xeon Gold 6152 at 2.10 GHz, 7 vCPUs, 7 GB RAM: a rented virtual server |
| OS | Linux 6.8.0-147-generic x86_64 (Ubuntu 24.04), `kernel.perf_event_paranoid` = 1 |
| Go | go version go1.27.1 linux/amd64 |
| Rust | rustc 1.100.0 (0ef1bc386 2026-11-09) for both binaries, read from each binary; GoPatina's crates otherwise pin rustc 1.93.0 |
| Method | `bench/run.sh --equiv --metric perf --runs 100 --toolchain same --rust stable` (and `--sigma 6`): 100 runs of each binary in ABBA order after 2 warm-ups each; user instructions (the primary metric), user cycles and wall time, each with its own TOST at ±0.5% |
| Load | the long correctness run was running at the lowest priority throughout: load average 7.62, 7.77, 7.89 before the 5σ run and 9.65, 8.72, 8.25 after; 9.70, 9.03, 8.45 before the 6σ run and 9.80, 8.98, 8.51 after |
| Raw results | [5σ](2026-11-22-server-bfed1e1-same-5sigma.json), [6σ](2026-11-22-server-bfed1e1-same-6sigma.json) (the runs' temporary binary paths removed) |

## User instructions: the primary metric

GoPatina / hand-written Rust, with the interval at the run's confidence:

| Case | 5σ run | 6σ run | Verdict, both runs |
| --- | --- | --- | --- |
| `fib` | 1.0000083 [1.0000077, 1.0000089] | 1.0000082 [1.0000074, 1.0000090] | equivalent |
| `sieve` | 1.0000064 [1.0000058, 1.0000069] | 1.0000062 [1.0000056, 1.0000069] | equivalent |
| `suffix-sum` | 1.0000072 [1.0000066, 1.0000078] | 1.0000071 [1.0000061, 1.0000082] | equivalent |

All three benchmarks execute the same user instructions as the hand-written
Rust to within 0.001%: every interval lies far inside ±0.5%, so the speed bar
is met on its primary metric, at 5σ and at 6σ. The small excess (6 to 8
millionths) is the same in both runs and varies by about one millionth
between runs, so it is a fixed cost, consistent with the one difference
`bench/asm-check.sh --toolchain stable` finds: GoPatina's binary installs its
Go-style panic hook at startup (fib's hot function 30 instructions in both, identical; the `main`
functions differ by that setup: fib 17 vs 16, sieve 73 vs 70, suffix-sum 128
vs 125 instructions). User instruction counts are not changed by other
processes on the machine, so the load does not affect them.

## Cycles and wall time: secondary, measured under load

| Case | Cycles, 5σ | Wall time, 5σ | Cycles, 6σ | Wall time, 6σ |
| --- | --- | --- | --- | --- |
| `fib` | 0.910 [0.892, 0.928] "faster" | 0.932 [0.866, 1.004] inconclusive | 0.903 [0.881, 0.926] "faster" | 0.927 [0.849, 1.011] inconclusive |
| `sieve` | 0.965 [0.898, 1.037] inconclusive | 0.967 [0.885, 1.056] inconclusive | 0.989 [0.900, 1.087] inconclusive | 0.992 [0.899, 1.095] inconclusive |
| `suffix-sum` | 1.000 [0.989, 1.012] inconclusive | 0.993 [0.944, 1.045] inconclusive | 1.006 [0.992, 1.021] inconclusive | 0.994 [0.939, 1.052] inconclusive |

These ran beside the correctness run, and cycles and wall time do depend on
what else the machine is doing. `fib`'s cycles read about 9% fewer for
GoPatina although both binaries execute the same instructions, apart from
the startup hook; that is not taken as a result. It points at the conditions
(the order of runs under load, frequency and cache effects) rather than at
the code. On run time the bar is therefore not decided: that needs the same
runs on a quiet machine.

## The cross-toolchain comparison (secondary)

With `--toolchain shipped` (GoPatina on its pinned rustc 1.93.0, the
hand-written Rust on 1.100.0), the instruction ratios were 1.0315 for `fib`,
0.8203 for `sieve` and 1.0000097 for `suffix-sum`. They measure the
difference between the two rustc releases, not GoPatina: under one rustc,
GoPatina's hot loops match the hand-written ones. The analysis is in #47.
That run's raw JSON is not in this directory, so its numbers are not
reported as results.
