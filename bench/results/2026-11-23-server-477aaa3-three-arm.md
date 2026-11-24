# Test-server run, three arms, after owned pointers: 2026-11-23

The second official three-arm run (GoPatina, hand-written Rust and plain
Go), the first after RFC 0016's owned pointers (#125): a pointer with one
owner is now a `Box` (`rt::Own`) instead of a reference-counted cell. Same
method, machine and toolchains as the
[first three-arm run](2026-11-23-server-d207c32-three-arm.md), so the two
compare directly.

| Field | Value |
| --- | --- |
| Date | 2026-11-23 23:52:17 to 2026-11-24 00:32:04 UTC |
| GoPatina | `477aaa3a1d180848aeb3d19e36e3a3cf69d2a9b6` (main, with #125) |
| Machine | Intel Xeon Gold 6152 at 2.10 GHz, 7 vCPUs: the rented virtual server of the earlier runs |
| OS | Linux 6.8.0-147-generic x86_64 |
| Go | go version go1.27.1 linux/amd64 |
| Rust | rustc 1.100.0 for both Rust binaries, read from each binary (`--toolchain same`) |
| Method | `bench/run.sh --equiv --metric perf --runs 100 --toolchain same --rust stable`: 100 rounds of the three binaries in ABCCBA order after 2 warm-ups each; user instructions, user cycles and wall time from `perf stat`; peak resident memory from 20 more runs of each under GNU time |
| Load | the correctness run was paused (SIGSTOP) from 23:51:15 to 00:32:04 UTC. Load averages (1, 5, 15 min): 9.46, 8.44, 8.41 before the pause; 3.59, 6.93, 7.89 after a 60 s settle, at the start; 1.28, 1.21, 1.89 at the end |
| Raw results | [2026-11-23-server-477aaa3-three-arm.json](2026-11-23-server-477aaa3-three-arm.json) (the run's temporary binary paths reduced to file names) |

## Each program

Geometric mean of the runs, with its 5σ interval. Binary sizes are as each
tool builds by default: GoPatina's crates strip their binaries
(`strip = true`); the hand-written Rust's and Go's are not stripped. The
sizes compare defaults, not code.

| Case | Program | Wall time | User instructions | Peak memory | Binary size |
| --- | --- | --- | --- | --- | --- |
| `fib` | Go (`go build`) | 2.44 s [2.34, 2.56] | 12.57 G [12.57, 12.57] | 1.82 MiB [1.72, 1.93] | 2,287 KiB |
| `fib` | GoPatina | 1.27 s [1.21, 1.33] | 8.505 G [8.505, 8.505] | 2.01 MiB [1.9, 2.13] | 339 KiB |
| `fib` | Rust (by hand) | 1.42 s [1.35, 1.49] | 8.505 G [8.505, 8.505] | 1.94 MiB [1.84, 2.05] | 435 KiB |
| `sieve` | Go (`go build`) | 0.272 s [0.253, 0.292] | 0.3846 G [0.3846, 0.3847] | 21.1 MiB [20.9, 21.3] | 2,288 KiB |
| `sieve` | GoPatina | 0.268 s [0.25, 0.286] | 0.3846 G [0.3846, 0.3846] | 20.9 MiB [20.9, 20.9] | 339 KiB |
| `sieve` | Rust (by hand) | 0.266 s [0.248, 0.284] | 0.3846 G [0.3846, 0.3846] | 20.9 MiB [20.9, 20.9] | 435 KiB |
| `suffix-sum` | Go (`go build`) | 0.322 s [0.302, 0.344] | 0.4349 G [0.4348, 0.435] | 155 MiB [155, 155] | 2,288 KiB |
| `suffix-sum` | GoPatina | 0.304 s [0.289, 0.321] | 0.3227 G [0.3227, 0.3227] | 154 MiB [154, 154] | 339 KiB |
| `suffix-sum` | Rust (by hand) | 0.306 s [0.287, 0.325] | 0.3227 G [0.3227, 0.3227] | 154 MiB [154, 154] | 436 KiB |
| `binary-trees` | Go (`go build`) | 3.45 s [3.34, 3.56] | 33.5 G [33.44, 33.57] | 34.8 MiB [33.2, 36.4] | 2,305 KiB |
| `binary-trees` | GoPatina | 4.37 s [4.19, 4.56] | 22.79 G [22.79, 22.79] | 34 MiB [33.9, 34.1] | 346 KiB |
| `binary-trees` | Rust (by hand) | 3.66 s [3.52, 3.81] | 19.16 G [19.16, 19.16] | 33.9 MiB [33.8, 34.1] | 437 KiB |
| `cycles` | Go (`go build`) | 0.163 s [0.152, 0.175] | 0.6459 G [0.6373, 0.6547] | 7.5 MiB [7.19, 7.81] | 2,288 KiB |
| `cycles` | GoPatina | 0.458 s [0.433, 0.485] | 0.9095 G [0.9095, 0.9095] | 185 MiB [185, 185] | 346 KiB |
| `cycles` | Rust (by hand) | 0.225 s [0.213, 0.239] | 0.9249 G [0.9249, 0.9249] | 2.96 MiB [2.87, 3.06] | 438 KiB |

## Every pair

Ratio of the geometric means, its 5σ interval and the TOST verdict at
±0.5%: "equivalent" only if the whole interval lies inside the margin,
"faster" or "slower" if it lies wholly outside, "inconclusive" otherwise.
For memory, "smaller" and "larger" stand for faster and slower.
Instructions, cycles and wall time are over 100 pairs, peak memory over 20.

| Case | Pair | Instructions | Cycles | Wall time | Peak memory |
| --- | --- | --- | --- | --- | --- |
| `fib` | GoPatina / Go | 0.67643 [0.67643, 0.67643] faster | 0.526 [0.519, 0.533] faster | 0.519 [0.491, 0.548] faster | 1.103 [1.034, 1.177] larger |
| `fib` | Go / Rust | 1.47835 [1.47834, 1.47835] slower | 1.710 [1.683, 1.737] slower | 1.728 [1.643, 1.817] slower | 0.939 [0.864, 1.020] inconclusive |
| `fib` | GoPatina / Rust | 1.00000 [1.00000, 1.00000] equivalent | 0.899 [0.885, 0.914] faster | 0.896 [0.851, 0.943] faster | 1.035 [0.957, 1.120] inconclusive |
| `sieve` | GoPatina / Go | 1.00003 [0.99993, 1.00014] equivalent | 1.006 [0.961, 1.053] inconclusive | 0.985 [0.910, 1.067] inconclusive | 0.988 [0.979, 0.998] inconclusive |
| `sieve` | Go / Rust | 0.99997 [0.99986, 1.00008] equivalent | 0.997 [0.953, 1.042] inconclusive | 1.023 [0.956, 1.093] inconclusive | 1.012 [1.002, 1.022] inconclusive |
| `sieve` | GoPatina / Rust | 1.00001 [1.00001, 1.00001] equivalent | 1.003 [0.967, 1.040] inconclusive | 1.007 [0.944, 1.076] inconclusive | 1.000 [1.000, 1.000] equivalent |
| `suffix-sum` | GoPatina / Go | 0.74203 [0.74190, 0.74216] faster | 0.942 [0.923, 0.962] faster | 0.945 [0.894, 0.998] inconclusive | 0.997 [0.996, 0.998] equivalent |
| `suffix-sum` | Go / Rust | 1.34766 [1.34742, 1.34790] slower | 1.055 [1.029, 1.082] slower | 1.054 [1.004, 1.106] inconclusive | 1.003 [1.002, 1.004] equivalent |
| `suffix-sum` | GoPatina / Rust | 1.00001 [1.00001, 1.00001] equivalent | 0.994 [0.979, 1.010] inconclusive | 0.995 [0.950, 1.043] inconclusive | 1.000 [1.000, 1.000] equivalent |
| `binary-trees` | GoPatina / Go | 0.68027 [0.67901, 0.68154] faster | 0.625 [0.611, 0.639] faster | 1.267 [1.208, 1.329] slower | 0.977 [0.933, 1.023] inconclusive |
| `binary-trees` | Go / Rust | 1.74848 [1.74523, 1.75174] slower | 1.914 [1.879, 1.949] slower | 0.942 [0.905, 0.980] faster | 1.024 [0.977, 1.073] inconclusive |
| `binary-trees` | GoPatina / Rust | 1.18945 [1.18945, 1.18945] slower | 1.196 [1.166, 1.226] slower | 1.193 [1.136, 1.253] slower | 1.001 [0.995, 1.006] inconclusive |
| `cycles` | GoPatina / Go | 1.40811 [1.38931, 1.42715] slower | 1.409 [1.370, 1.450] slower | 2.807 [2.648, 2.975] slower | 24.696 [23.694, 25.742] larger |
| `cycles` | Go / Rust | 0.69833 [0.68901, 0.70777] faster | 0.689 [0.667, 0.711] faster | 0.724 [0.683, 0.768] faster | 2.530 [2.371, 2.700] larger |
| `cycles` | GoPatina / Rust | 0.98332 [0.98332, 0.98332] faster | 0.971 [0.943, 0.998] inconclusive | 2.033 [1.926, 2.146] slower | 62.493 [60.484, 64.569] larger |

## What changed, and what each case shows

- **`binary-trees`** is the case owned pointers were for. Against the
  hand-written Rust, GoPatina's peak memory went from 1.473 times Rust's
  to **1.001 [0.995, 1.006]**: 34.0 MiB against 33.9 MiB, inconclusive
  only because the interval reaches 0.1% past the ±0.5% margin. Its wall
  time went from 1.496 to **1.193 [1.136, 1.253]** times Rust's (4.37 s
  against 3.66 s; 5.76 s before), and its user instructions from 26.29 G
  to 22.79 G, 1.189 times Rust's 19.16 G. GoPatina is still slower than
  Rust and than Go (1.267 times Go's wall time): the remaining 19% of
  instructions are not yet measured to a cause. Plain Go is still the
  fastest, 0.942 times Rust's wall time while executing 1.75 times its
  instructions, with its collector running concurrently.
- **`cycles`** (doubly linked lists built and dropped) is unchanged: its
  lists are shared, so they stay reference-counted cells and are never
  freed (RFC 0013's known limitation). GoPatina takes 2.03 times Rust's
  time with 185 MiB at peak against Rust's 2.96 MiB and Go's 7.5 MiB. RFC
  0016's step 2 collects the cycles.
- **`fib`**: GoPatina executes the same user instructions as the
  hand-written Rust (1.00000), and plain Go 1.48 times as many.
  GoPatina's cycles and wall time read about 10% below Rust's, as in the
  earlier runs, although the instructions are the same; the cause is not
  established, and it is not taken as GoPatina being faster than Rust.
- **`sieve`** and **`suffix-sum`**: GoPatina executes the same
  instructions as Rust (to 0.001%) and has the same peak memory; cycles
  and wall time are inconclusive against Rust, as before.
- **Binary size**: GoPatina 339 to 346 KiB (stripped), hand-written Rust
  435 to 438 KiB and Go 2,287 to 2,305 KiB (not stripped).
- The hand-written Rust's own `binary-trees` time read 3.66 s here and
  3.85 s in the first run, on the same binary source and toolchain: the
  virtual server's speed varies between runs, which is why every claim
  above is a ratio within one run.
