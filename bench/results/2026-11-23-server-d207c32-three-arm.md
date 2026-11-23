# Test-server run, three arms: GoPatina, hand-written Rust and plain Go — 2026-11-23

The first official run with plain Go (`go build`) as a third arm
(`cmd/gopatina-bench -go`, #110), on all five benchmarks, with peak memory
and binary size. Every pair (GoPatina / Rust, Go / Rust, GoPatina / Go) has
its own TOST at ±0.5% and 5σ on each metric.

| Field | Value |
| --- | --- |
| Date | 2026-11-23, 02:54:35 to 03:38:36 UTC |
| GoPatina | `d207c32cca6f6aee84a4c7d272c9974bf97e65ad` (main, with #110) |
| Machine | Intel Xeon Gold 6152 at 2.10 GHz, 7 vCPUs, 7 GB RAM: a rented virtual server on shared hardware, the same as the 2026-11-22 runs |
| OS | Linux 6.8.0-147-generic x86_64 |
| Go | go version go1.27.1 linux/amd64 |
| Rust | rustc 1.100.0 (0ef1bc386 2026-11-09) for both Rust binaries, read from each binary (`--toolchain same`) |
| Method | `bench/run.sh --equiv --metric perf --runs 100 --toolchain same --rust stable`: 100 rounds of the three binaries in ABCCBA order after 2 warm-ups each; user instructions, user cycles and wall time from `perf stat`; peak resident memory from 20 more runs of each under GNU time |
| Load | the correctness run was paused (SIGSTOP) for the whole run and nothing else was scheduled; load averages were not recorded |
| Raw results | [2026-11-23-server-d207c32-three-arm.json](2026-11-23-server-d207c32-three-arm.json) (the run's temporary binary paths reduced to file names) |

## Each program

Geometric mean of the runs, with its 5σ interval. Binary sizes are as each
tool builds by default: GoPatina's crates strip their binaries
(`strip = true`); the hand-written Rust's and Go's are not stripped. The
sizes compare defaults, not code.

| Case | Program | Wall time | User instructions | Peak memory | Binary size |
| --- | --- | --- | --- | --- | --- |
| `fib` | Go (`go build`) | 2.6 s [2.5, 2.71] | 12.57 G [12.57, 12.57] | 1.81 MiB [1.7, 1.91] | 2,287 KiB |
| `fib` | GoPatina | 1.37 s [1.3, 1.44] | 8.505 G [8.505, 8.505] | 1.99 MiB [1.88, 2.1] | 339 KiB |
| `fib` | Rust (by hand) | 1.51 s [1.44, 1.59] | 8.505 G [8.505, 8.505] | 1.94 MiB [1.83, 2.04] | 435 KiB |
| `sieve` | Go (`go build`) | 0.266 s [0.251, 0.282] | 0.3847 G [0.3845, 0.3848] | 21.2 MiB [21.1, 21.3] | 2,288 KiB |
| `sieve` | GoPatina | 0.258 s [0.243, 0.273] | 0.3846 G [0.3846, 0.3846] | 20.9 MiB [20.9, 20.9] | 339 KiB |
| `sieve` | Rust (by hand) | 0.262 s [0.249, 0.276] | 0.3846 G [0.3846, 0.3846] | 20.9 MiB [20.9, 20.9] | 435 KiB |
| `suffix-sum` | Go (`go build`) | 0.311 s [0.296, 0.327] | 0.4349 G [0.4348, 0.435] | 155 MiB [155, 155] | 2,288 KiB |
| `suffix-sum` | GoPatina | 0.302 s [0.286, 0.318] | 0.3227 G [0.3227, 0.3227] | 154 MiB [154, 154] | 339 KiB |
| `suffix-sum` | Rust (by hand) | 0.3 s [0.286, 0.315] | 0.3227 G [0.3227, 0.3227] | 154 MiB [154, 154] | 436 KiB |
| `binary-trees` | Go (`go build`) | 3.62 s [3.48, 3.77] | 33.52 G [33.46, 33.59] | 34.5 MiB [32.6, 36.6] | 2,305 KiB |
| `binary-trees` | GoPatina | 5.76 s [5.47, 6.06] | 26.29 G [26.29, 26.29] | 50 MiB [49.9, 50.2] | 347 KiB |
| `binary-trees` | Rust (by hand) | 3.85 s [3.65, 4.05] | 19.16 G [19.16, 19.16] | 33.9 MiB [33.8, 34.1] | 437 KiB |
| `cycles` | Go (`go build`) | 0.168 s [0.159, 0.177] | 0.6446 G [0.6386, 0.6507] | 7.58 MiB [6.91, 8.32] | 2,288 KiB |
| `cycles` | GoPatina | 0.453 s [0.43, 0.477] | 0.9095 G [0.9095, 0.9095] | 185 MiB [185, 185] | 346 KiB |
| `cycles` | Rust (by hand) | 0.227 s [0.213, 0.241] | 0.9249 G [0.9249, 0.9249] | 2.95 MiB [2.85, 3.05] | 438 KiB |

## Every pair

Ratio of the geometric means, its 5σ interval and the TOST verdict at
±0.5%: "equivalent" only if the whole interval lies inside the margin,
"faster" or "slower" if it lies wholly outside, "inconclusive" otherwise.
For memory, "smaller" and "larger" stand for faster and slower.
Instructions, cycles and wall time are over 100 pairs, peak memory over 20.

| Case | Pair | Instructions | Cycles | Wall time | Peak memory |
| --- | --- | --- | --- | --- | --- |
| `fib` | GoPatina / Go | 0.67643 [0.67643, 0.67643] faster | 0.528 [0.515, 0.541] faster | 0.526 [0.498, 0.554] faster | 1.100 [1.013, 1.195] larger |
| `fib` | Go / Rust | 1.47835 [1.47835, 1.47836] slower | 1.712 [1.671, 1.754] slower | 1.720 [1.648, 1.796] slower | 0.932 [0.865, 1.005] inconclusive |
| `fib` | GoPatina / Rust | 1.00000 [1.00000, 1.00000] equivalent | 0.903 [0.883, 0.924] faster | 0.904 [0.868, 0.941] faster | 1.026 [0.935, 1.125] inconclusive |
| `sieve` | GoPatina / Go | 0.99996 [0.99949, 1.00042] equivalent | 1.005 [0.974, 1.037] inconclusive | 0.968 [0.909, 1.031] inconclusive | 0.985 [0.980, 0.990] smaller |
| `sieve` | Go / Rust | 1.00005 [0.99958, 1.00051] equivalent | 0.986 [0.956, 1.016] inconclusive | 1.016 [0.961, 1.075] inconclusive | 1.015 [1.010, 1.020] larger |
| `sieve` | GoPatina / Rust | 1.00001 [1.00001, 1.00001] equivalent | 0.991 [0.962, 1.020] inconclusive | 0.984 [0.931, 1.039] inconclusive | 1.000 [1.000, 1.000] equivalent |
| `suffix-sum` | GoPatina / Go | 0.74199 [0.74176, 0.74222] faster | 0.943 [0.923, 0.964] faster | 0.971 [0.920, 1.025] inconclusive | 0.997 [0.996, 0.998] equivalent |
| `suffix-sum` | Go / Rust | 1.34774 [1.34733, 1.34815] slower | 1.062 [1.043, 1.082] slower | 1.036 [0.991, 1.082] inconclusive | 1.003 [1.002, 1.004] equivalent |
| `suffix-sum` | GoPatina / Rust | 1.00001 [1.00001, 1.00001] equivalent | 1.002 [0.986, 1.018] inconclusive | 1.006 [0.960, 1.054] inconclusive | 1.000 [1.000, 1.000] equivalent |
| `binary-trees` | GoPatina / Go | 0.78434 [0.78276, 0.78592] faster | 0.772 [0.759, 0.786] faster | 1.591 [1.523, 1.662] slower | 1.448 [1.366, 1.535] larger |
| `binary-trees` | Go / Rust | 1.74960 [1.74608, 1.75312] slower | 1.922 [1.887, 1.957] slower | 0.940 [0.903, 0.979] faster | 1.018 [0.960, 1.079] inconclusive |
| `binary-trees` | GoPatina / Rust | 1.37227 [1.37227, 1.37227] slower | 1.484 [1.454, 1.516] slower | 1.496 [1.422, 1.574] slower | 1.473 [1.467, 1.480] larger |
| `cycles` | GoPatina / Go | 1.41094 [1.39782, 1.42418] slower | 1.430 [1.373, 1.490] slower | 2.701 [2.532, 2.882] slower | 24.406 [22.233, 26.792] larger |
| `cycles` | Go / Rust | 0.69693 [0.69044, 0.70347] faster | 0.684 [0.659, 0.710] faster | 0.740 [0.698, 0.784] faster | 2.572 [2.340, 2.826] larger |
| `cycles` | GoPatina / Rust | 0.98332 [0.98332, 0.98332] faster | 0.979 [0.939, 1.021] inconclusive | 1.999 [1.890, 2.114] slower | 62.761 [60.584, 65.017] larger |

## What each case shows

- **`fib`** (calls and integer arithmetic; nothing allocated) compares code
  generation. GoPatina executes the same user instructions as the
  hand-written Rust (1.00000), and plain Go 1.48 times as many: Go takes
  1.72 times Rust's wall time, and GoPatina 0.53 times Go's. GoPatina's
  cycles and wall time read about 10% below Rust's although the
  instructions are the same, as in the 2026-11-22 runs (9%). The cause is
  not established (code placement is one candidate), and it is not taken
  as GoPatina being faster than Rust.
- **`sieve`** (one slice, element writes): all three execute the same
  instructions to within 0.01%; cycles and wall time are inconclusive for
  every pair.
- **`suffix-sum`** (reads through `s[i:]` views): GoPatina and Rust execute
  the same instructions, Go 1.35 times as many but only 1.06 times the
  cycles; wall time is inconclusive for every pair.
- **`binary-trees`** (millions of small allocations and frees) is where
  memory management decides, and **plain Go is the fastest**: 0.94 times
  Rust's wall time while executing 1.75 times its instructions, as Go's
  collector runs concurrently with the program. GoPatina takes 1.50 times
  Rust's time and 1.59 times Go's, with 1.47 times Rust's peak memory: it
  represents a shared pointer as `Rc<RefCell<T>>` where the hand-written
  Rust uses `Box`. The causes, measured, and the fix are in
  [RFC 0016](../../docs/rfcs/0016-owned-pointers-and-cycle-collection.md).
- **`cycles`** (doubly linked lists built and dropped): **plain Go is the
  fastest** again, 0.74 times Rust's wall time with 0.70 times its
  instructions. GoPatina executes about as many instructions as Rust
  (0.98) but takes 2.00 times its time and 2.70 times Go's, with 185 MiB at
  peak against Rust's 3 MiB and Go's 7.6 MiB: it never frees the lists
  (RFC 0013's known limitation), and the time goes to the fresh memory.
  RFC 0016 collects the cycles.
- **Binary size**: GoPatina 339 to 347 KiB (stripped), hand-written Rust
  435 to 438 KiB and Go 2,287 to 2,305 KiB (not stripped).
