# GoPatina benchmarks: Go compiled through Rust vs hand-written Rust

How fast is Go when it is compiled through Rust? GoPatina's goal is identical speed to equivalent hand-written Rust on every benchmark, built with the latest stable Rust, where identical means equivalent within ±0.5% at 5σ. **On 2026-11-22 the test server measured it on user instructions, the primary metric: met, at 5σ and at 6σ, on all three benchmarks.** On run time it is not yet decided. The statistics behind it are in [the evidence](/evidence#statistics).

## The suite {#suite}

| Benchmark | What it exercises |
| --- | --- |
| `fib` | function calls and integer arithmetic |
| `sieve` | a large slice, nested loops and element writes |
| `suffix-sum` | a read-only `s[i:]` view passed to a proven reader |

Each benchmark is written in Go, built with `gopatina build -patina`, and as idiomatic hand-written Rust that is not tuned to flatter GoPatina; plain Go is measured too. All three builds must print the same output before any timing. The cases are in [bench/cases](https://github.com/pigfox/gopatina/tree/main/bench/cases).

## Go vs Rust performance: what has been measured {#results}

| Measured | Result (GoPatina / hand-written Rust) | When and where |
| --- | --- | --- |
| `fib` | the same user instructions to within 0.0003% (1.0000024); cycles and wall time inconclusive | 2026-11-21, laptop, CPU counters, 300 runs per binary |
| `sieve` | 1.302 times the instructions, about 7% slower in cycles and wall time | the same run |
| `suffix-sum` | the same user instructions to within 0.0003% (1.0000020); cycles and wall time inconclusive | the same run |
| Machine code after RFC 0006 | the hot loops of all three are the same instructions as the hand-written Rust's | 2026-11-21, rustc 1.93.0 |
| Test-server pilot, wall time | `fib` 0.984, `sieve` 0.975, `suffix-sum` 1.007: all inconclusive, with 5σ intervals 4% to 21% wide | 2026-11-22, test server, 100 runs per binary |
| Test server, user instructions, one rustc (1.100.0) for both | `fib` 1.0000082, `sieve` 1.0000062, `suffix-sum` 1.0000071: all equivalent at 6σ, every interval far inside ±0.5% | 2026-11-22, test server, CPU counters, 100 runs per binary |
| Test server, cycles and wall time | inconclusive, measured under load from the correctness run | the same runs |
| The speed bar (±0.5% at 5σ and 6σ) | met on user instructions; on run time pending a quiet run | the test server |

`sieve` was slower because its generated inner loop kept one extra bounds check, which preserves Go's integer semantics. [RFC 0006](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0006-proven-ranges.md)'s loop specialization has since removed it where GoPatina can prove the arithmetic cannot wrap, so all three benchmarks now differ from the hand-written Rust only in a 3-instruction panic-hook setup at startup. That is a comparison of code, not of speed. The first test-server run, a 100-run pilot on 2026-11-22, measured wall time with a run-to-run spread of 7% to 20%, far wider than the ±0.5% margin, so it shows neither a difference nor equivalence; deciding the bar needed CPU counters. With them, and one rustc for both binaries, GoPatina executes the same user instructions as the hand-written Rust to within 0.001%, a fixed excess consistent with the startup panic-hook setup. Instructions count work, not time; run time still needs a quiet machine.

## Where the numbers come from {#method}

`bench/run.sh --equiv` runs GoPatina's binary and the hand-written Rust in alternating ABBA order and tests their equivalence with two one-sided tests; `bench/asm-check.sh` compares their machine code. Every result is published with its commit, toolchains, machine, date and raw JSON in [BENCHMARKS.md](https://github.com/pigfox/gopatina/blob/main/BENCHMARKS.md). See also [correctness](/correctness): speed only counts if the program does what the Go program does.
