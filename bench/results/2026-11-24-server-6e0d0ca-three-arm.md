# Test-server run, three arms, after one owner per pointer: 2026-11-24

The third official three-arm run (GoPatina, hand-written Rust and plain
Go). It is the first since RFC 0017's first five steps (#133, #136, #138,
#139 and #140): an owned pointer is now a plain `Option<Box<T>>`, lent as
`Option<&T>` or `&mut`, instead of GoPatina's runtime wrapper. It is also
the first with `cycles` rewritten for one owner per node (#135). Same
method, machine and toolchains as the
[previous run](2026-11-23-server-477aaa3-three-arm.md).

| Field | Value |
| --- | --- |
| Date | 2026-11-24 09:29:40 to 10:10:16 UTC |
| GoPatina | `6e0d0cac1e813a060bd1f55ebf02974460b340bd` (main, with #140) |
| Machine | Intel Xeon Gold 6152 at 2.10 GHz (Skylake-SP), 7 vCPUs: the rented virtual server of the earlier runs |
| OS | Linux 6.8.0-147-generic x86_64 |
| Go | go version go1.27.1 linux/amd64 |
| Rust | rustc 1.100.0 for both Rust binaries, read from each binary (`--toolchain same`) |
| Method | `bench/run.sh --equiv --metric perf --runs 100 --toolchain same --rust stable`: 100 rounds of the three binaries in ABCCBA order after 2 warm-ups each; user instructions, user cycles and wall time from `perf stat`; peak resident memory from 20 more runs of each under GNU time |
| Load | the correctness run was paused (SIGSTOP) from 09:28:38 to 10:10:16 UTC. Load averages (1, 5, 15 min): 7.52, 7.44, 5.16 before the pause; 2.76, 6.09, 4.84 after a 60 s settle; 1.65, 1.43, 1.71 at the end |
| Raw results | [2026-11-24-server-6e0d0ca-three-arm.json](2026-11-24-server-6e0d0ca-three-arm.json) (the run's temporary binary paths reduced to file names) |

## Each program

Geometric mean of the runs, with its 5σ interval. Binary sizes are as each
tool builds by default: GoPatina's crates strip their binaries
(`strip = true`); the hand-written Rust's and Go's are not stripped. The
sizes compare defaults, not code.

| Case | Program | Wall time | User instructions | Peak memory | Binary size |
| --- | --- | --- | --- | --- | --- |
| `fib` | Go (`go build`) | 2.71 s [2.52, 2.9] | 12.57 G [12.57, 12.57] | 1.82 MiB [1.72, 1.93] | 2,287 KiB |
| `fib` | GoPatina | 1.4 s [1.31, 1.51] | 8.505 G [8.505, 8.505] | 1.95 MiB [1.85, 2.06] | 339 KiB |
| `fib` | Rust (by hand) | 1.58 s [1.46, 1.72] | 8.505 G [8.505, 8.505] | 1.95 MiB [1.85, 2.06] | 435 KiB |
| `sieve` | Go (`go build`) | 0.274 s [0.256, 0.294] | 0.3848 G [0.3843, 0.3853] | 21.1 MiB [21, 21.3] | 2,288 KiB |
| `sieve` | GoPatina | 0.287 s [0.267, 0.309] | 0.3846 G [0.3846, 0.3846] | 20.9 MiB [20.9, 20.9] | 339 KiB |
| `sieve` | Rust (by hand) | 0.28 s [0.259, 0.302] | 0.3846 G [0.3846, 0.3846] | 20.9 MiB [20.9, 20.9] | 435 KiB |
| `suffix-sum` | Go (`go build`) | 0.327 s [0.308, 0.348] | 0.4349 G [0.4348, 0.4349] | 155 MiB [155, 155] | 2,288 KiB |
| `suffix-sum` | GoPatina | 0.318 s [0.301, 0.337] | 0.3227 G [0.3227, 0.3227] | 154 MiB [154, 154] | 340 KiB |
| `suffix-sum` | Rust (by hand) | 0.318 s [0.3, 0.337] | 0.3227 G [0.3227, 0.3227] | 154 MiB [154, 154] | 436 KiB |
| `binary-trees` | Go (`go build`) | 3.6 s [3.48, 3.73] | 33.53 G [33.47, 33.59] | 34.6 MiB [33, 36.3] | 2,305 KiB |
| `binary-trees` | GoPatina | 4.08 s [3.88, 4.29] | 19.16 G [19.16, 19.16] | 33.9 MiB [33.8, 34] | 341 KiB |
| `binary-trees` | Rust (by hand) | 3.99 s [3.79, 4.2] | 19.16 G [19.16, 19.16] | 33.9 MiB [33.8, 34.1] | 437 KiB |
| `cycles` | Go (`go build`) | 0.208 s [0.193, 0.224] | 0.2363 G [0.2333, 0.2392] | 8.59 MiB [5.68, 13] | 2,293 KiB |
| `cycles` | GoPatina | 0.0579 s [0.0538, 0.0624] | 0.1003 G [0.1003, 0.1003] | 2.31 MiB [2.23, 2.4] | 340 KiB |
| `cycles` | Rust (by hand) | 0.0674 s [0.0626, 0.0724] | 0.1063 G [0.1063, 0.1063] | 2.42 MiB [2.34, 2.51] | 437 KiB |

## Every pair

Ratio of the geometric means, its 5σ interval and the TOST verdict at
±0.5%: "equivalent" only if the whole interval lies inside the margin,
"faster" or "slower" if it lies wholly outside, "inconclusive" otherwise.
For memory, "smaller" and "larger" stand for faster and slower.
Instructions, cycles and wall time are over 100 pairs, peak memory over 20.

| Case | Pair | Instructions | Cycles | Wall time | Peak memory |
| --- | --- | --- | --- | --- | --- |
| `fib` | GoPatina / Go | 0.67643 [0.67642, 0.67643] faster | 0.528 [0.520, 0.537] faster | 0.519 [0.490, 0.550] faster | 1.069 [0.995, 1.148] inconclusive |
| `fib` | Go / Rust | 1.47836 [1.47835, 1.47837] slower | 1.716 [1.689, 1.743] slower | 1.708 [1.616, 1.806] slower | 0.936 [0.871, 1.005] inconclusive |
| `fib` | GoPatina / Rust | 1.00000 [1.00000, 1.00000] equivalent | 0.906 [0.893, 0.919] faster | 0.886 [0.842, 0.933] faster | 1.000 [0.919, 1.088] inconclusive |
| `sieve` | GoPatina / Go | 0.99963 [0.99836, 1.00090] equivalent | 1.081 [1.011, 1.157] slower | 1.049 [0.956, 1.151] inconclusive | 0.987 [0.980, 0.994] smaller |
| `sieve` | Go / Rust | 1.00038 [0.99911, 1.00165] equivalent | 0.960 [0.893, 1.031] inconclusive | 0.980 [0.899, 1.068] inconclusive | 1.013 [1.006, 1.020] larger |
| `sieve` | GoPatina / Rust | 1.00001 [1.00001, 1.00001] equivalent | 1.038 [0.971, 1.108] inconclusive | 1.028 [0.950, 1.113] inconclusive | 1.000 [1.000, 1.000] equivalent |
| `suffix-sum` | GoPatina / Go | 0.74202 [0.74195, 0.74210] faster | 0.944 [0.927, 0.962] faster | 0.972 [0.906, 1.044] inconclusive | 0.997 [0.996, 0.998] equivalent |
| `suffix-sum` | Go / Rust | 1.34768 [1.34754, 1.34781] slower | 1.057 [1.034, 1.081] slower | 1.031 [0.972, 1.093] inconclusive | 1.003 [1.002, 1.004] equivalent |
| `suffix-sum` | GoPatina / Rust | 1.00001 [1.00001, 1.00001] equivalent | 0.998 [0.977, 1.020] inconclusive | 1.002 [0.943, 1.065] inconclusive | 1.000 [1.000, 1.000] equivalent |
| `binary-trees` | GoPatina / Go | 0.57151 [0.57047, 0.57254] faster | 0.542 [0.530, 0.555] faster | 1.133 [1.070, 1.199] slower | 0.980 [0.934, 1.028] inconclusive |
| `binary-trees` | Go / Rust | 1.74973 [1.74656, 1.75291] slower | 1.898 [1.858, 1.939] slower | 0.902 [0.859, 0.948] faster | 1.020 [0.973, 1.069] inconclusive |
| `binary-trees` | GoPatina / Rust | 0.99998 [0.99998, 0.99998] equivalent | 1.029 [0.999, 1.060] inconclusive | 1.022 [0.966, 1.082] inconclusive | 0.999 [0.993, 1.006] inconclusive |
| `cycles` | GoPatina / Go | 0.42449 [0.41921, 0.42984] faster | 0.232 [0.217, 0.248] faster | 0.279 [0.257, 0.302] faster | 0.269 [0.177, 0.408] smaller |
| `cycles` | Go / Rust | 2.22237 [2.19473, 2.25037] slower | 3.434 [3.211, 3.673] slower | 3.081 [2.858, 3.321] slower | 3.551 [2.327, 5.418] larger |
| `cycles` | GoPatina / Rust | 0.94338 [0.94338, 0.94339] faster | 0.796 [0.777, 0.816] faster | 0.860 [0.800, 0.924] faster | 0.955 [0.909, 1.003] inconclusive |

## What changed, and what each case shows

- **`binary-trees`**: GoPatina now executes the same user instructions as
  the hand-written Rust, **0.99998 times, equivalent** (19.16 G each).
  In the previous run it executed 1.189 times as many. Its wall time,
  **1.022 [0.966, 1.082]** times Rust's (4.08 s against 3.99 s), cycles
  1.029 and peak memory 0.999 times Rust's are all inconclusive at ±0.5%:
  the intervals cannot tell the two apart at this precision. In the
  previous run its wall time was 1.193 times Rust's, a "slower" verdict.
  Plain Go is still the fastest on wall time, 0.902 times Rust's while
  executing 1.75 times its instructions, its collector running
  concurrently. GoPatina takes 1.133 times Go's wall time.
- **`cycles`** was rewritten in #135, the Go program and the hand-written Rust both: a list's nodes
  live in one slice and link by index, as Rust programmers write such a
  list, so these numbers do not compare with earlier runs. GoPatina
  executes 0.943 times the hand-written Rust's instructions, and its wall
  time is **0.860 [0.800, 0.924]**, a "faster" verdict, with peak memory
  inconclusive at 0.955. The cause of the 6% fewer instructions is not
  established here. Plain Go takes 3.08 times Rust's wall time on this
  version of the case.
- **`fib`**: GoPatina executes the same user instructions as the
  hand-written Rust (1.00000), and plain Go 1.48 times as many.
  GoPatina's cycles (0.906) and wall time (0.886) read about 10% below
  Rust's, as in every earlier run, although the instructions are the
  same. The cause is not established, and it is not taken as GoPatina
  being faster than Rust.
- **`suffix-sum`**: the same instructions as Rust (to 0.001%) and the same
  peak memory; cycles and wall time are inconclusive.
- **`sieve` and the JCC erratum.** GoPatina's `sieve` loop and the
  hand-written Rust's are the same machine code, instruction for
  instruction, and they execute the same instructions (1.00001 times).
  Even so, GoPatina's has taken a few percent more cycles in some runs
  (1.038 here, inconclusive). The cause is where the loop sits in the
  binary, not what it does. In GoPatina's binary, as examined, the loop's
  closing conditional jump is a `jb` at 0x156fe, and it ends exactly on
  the 32-byte boundary 0x15700. The test server's CPU is a Skylake-SP
  (Xeon Gold 6152). Intel's microcode fix for the jump conditional code
  (JCC) erratum keeps any jump that crosses or ends on a 32-byte boundary
  out of the decoded micro-op cache, so that loop is decoded again by the
  slower legacy decoders on every pass. The hand-written Rust's loop sits
  elsewhere. Where a loop sits depends on everything placed before it, so
  an unrelated change to GoPatina's output, its runtime or the toolchain
  can move the loop and flip the effect either way. The same binaries can
  also read differently on another CPU. This is not a difference in the
  code GoPatina generates, so it is reported here rather than tuned for:
  neither build pads its jumps away from 32-byte boundaries (LLVM's
  `-x86-branches-within-32B-boundaries`), and the benchmarks are not
  tuned for one CPU.
- **Binary size**: GoPatina 339 to 341 KiB (stripped), hand-written Rust
  435 to 437 KiB and Go 2,287 to 2,305 KiB (not stripped).
- The hand-written Rust's own `binary-trees` time read 3.99 s here and
  3.66 s in the previous run, from the same source and toolchain. The
  virtual server's speed varies between runs, which is why every claim
  above is a ratio within one run.
