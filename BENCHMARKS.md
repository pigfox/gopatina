# Benchmarks

This is where GoPatina's official benchmark results are published: GoPatina
against equivalent hand-written Rust and plain Go, measured on the
maintainer's test server. Both Rust sides are built by the same rustc:
GoPatina's is the Rust it generates from the Go program, the other is
written by hand, so a difference between them is a difference in the Rust
code, not in the compiler. How the numbers are produced is described in
[docs/QUALITY.md](docs/QUALITY.md#performance-shadow-rust-and-keep-up) and
[bench/README.md](bench/README.md).

> **Latest, 2026-11-24: one owner per pointer measured.** On
> `binary-trees`, GoPatina now executes the same user instructions as
> hand-written Rust (0.99998 times, equivalent), down from 1.19 times, and
> its wall time and peak memory can no longer be told apart from Rust's
> (1.022 and 0.999 times, inconclusive at ±0.5%): RFC 0017's first five
> steps, an owned pointer as a plain `Option<Box<T>>`. On `fib`, `sieve`
> and `suffix-sum` it executes the same instructions as Rust, as before. On
> `cycles`, rewritten to link its nodes by index (#135), it executes 0.943
> times Rust's instructions and is faster on wall time (0.860). Plain Go is
> still the fastest on `binary-trees`. The correctness bar is pending. The
> local runs further down are earlier evidence from the maintainer's
> workstation, not official results.

## Latest official run: three arms, one owner per pointer, 2026-11-24

GoPatina (`gopatina build -patina`), hand-written Rust and plain Go
(`go build`) on all five benchmarks: `bench/run.sh --equiv --metric perf
--runs 100 --toolchain same --rust stable`, 100 rounds in ABCCBA order,
GoPatina commit `6e0d0cac1e813a060bd1f55ebf02974460b340bd` (RFC 0017's
steps 1 to 5, and `cycles` rewritten by #135), rustc 1.100.0 for both Rust
binaries, Go 1.27.1, on the same Xeon Gold 6152 (Skylake-SP) virtual
server as the runs below, with the correctness run paused (load averages
2.76, 6.09, 4.84 at the start and 1.65, 1.43, 1.71 at the end). Every
table, every pair and the reading of each case are in the
[run notes](bench/results/2026-11-24-server-6e0d0ca-three-arm.md); raw results:
[2026-11-24-server-6e0d0ca-three-arm.json](bench/results/2026-11-24-server-6e0d0ca-three-arm.json).

Each program, geometric mean with its 5σ interval:

| Benchmark | Program | Wall time | User instructions | Peak memory | Binary size |
| --- | --- | --- | --- | --- | --- |
| `fib` | Go | 2.71 s [2.52, 2.90] | 12.57 G | 1.82 MiB [1.72, 1.93] | 2,287 KiB |
| `fib` | GoPatina | 1.40 s [1.31, 1.51] | 8.505 G | 1.95 MiB [1.85, 2.06] | 339 KiB |
| `fib` | Rust | 1.58 s [1.46, 1.72] | 8.505 G | 1.95 MiB [1.85, 2.06] | 435 KiB |
| `sieve` | Go | 0.274 s [0.256, 0.294] | 0.3848 G [0.3843, 0.3853] | 21.1 MiB [21.0, 21.3] | 2,288 KiB |
| `sieve` | GoPatina | 0.287 s [0.267, 0.309] | 0.3846 G | 20.9 MiB [20.9, 20.9] | 339 KiB |
| `sieve` | Rust | 0.280 s [0.259, 0.302] | 0.3846 G | 20.9 MiB [20.9, 20.9] | 435 KiB |
| `suffix-sum` | Go | 0.327 s [0.308, 0.348] | 0.4349 G | 155 MiB [155, 155] | 2,288 KiB |
| `suffix-sum` | GoPatina | 0.318 s [0.301, 0.337] | 0.3227 G | 154 MiB [154, 154] | 340 KiB |
| `suffix-sum` | Rust | 0.318 s [0.300, 0.337] | 0.3227 G | 154 MiB [154, 154] | 436 KiB |
| `binary-trees` | Go | 3.60 s [3.48, 3.73] | 33.53 G [33.47, 33.59] | 34.6 MiB [33.0, 36.3] | 2,305 KiB |
| `binary-trees` | GoPatina | 4.08 s [3.88, 4.29] | 19.16 G | 33.9 MiB [33.8, 34.0] | 341 KiB |
| `binary-trees` | Rust | 3.99 s [3.79, 4.20] | 19.16 G | 33.9 MiB [33.8, 34.1] | 437 KiB |
| `cycles` | Go | 0.208 s [0.193, 0.224] | 0.2363 G [0.2333, 0.2392] | 8.59 MiB [5.68, 13.0] | 2,293 KiB |
| `cycles` | GoPatina | 0.0579 s [0.0538, 0.0624] | 0.1003 G | 2.31 MiB [2.23, 2.40] | 340 KiB |
| `cycles` | Rust | 0.0674 s [0.0626, 0.0724] | 0.1063 G | 2.42 MiB [2.34, 2.51] | 437 KiB |

Instruction intervals not shown are narrower than the digits given. Binary
sizes are each tool's default: GoPatina's are stripped, the others' are not.

GoPatina against each, ratio with its 5σ interval and TOST verdict at ±0.5%:

| Benchmark | GoPatina / Rust, instructions | GoPatina / Rust, cycles | GoPatina / Rust, wall | GoPatina / Rust, peak memory | GoPatina / Go, wall |
| --- | --- | --- | --- | --- | --- |
| `fib` | 1.00000 equivalent | 0.906 [0.893, 0.919] faster | 0.886 [0.842, 0.933] faster | 1.000 [0.919, 1.088] inconclusive | 0.519 [0.490, 0.550] faster |
| `sieve` | 1.00001 equivalent | 1.038 [0.971, 1.108] inconclusive | 1.028 [0.950, 1.113] inconclusive | 1.000 equivalent | 1.049 [0.956, 1.151] inconclusive |
| `suffix-sum` | 1.00001 equivalent | 0.998 [0.977, 1.020] inconclusive | 1.002 [0.943, 1.065] inconclusive | 1.000 equivalent | 0.972 [0.906, 1.044] inconclusive |
| `binary-trees` | 0.99998 equivalent | 1.029 [0.999, 1.060] inconclusive | 1.022 [0.966, 1.082] inconclusive | 0.999 [0.993, 1.006] inconclusive | 1.133 [1.070, 1.199] slower |
| `cycles` | 0.94338 faster | 0.796 [0.777, 0.816] faster | 0.860 [0.800, 0.924] faster | 0.955 [0.909, 1.003] inconclusive | 0.279 [0.257, 0.302] faster |

What one owner per pointer changed on `binary-trees`, against the
[previous run](bench/results/2026-11-23-server-477aaa3-three-arm.md) on the same server and toolchains:

| `binary-trees`, GoPatina / Rust | Before: `477aaa3`, owned pointers in `rt::Own` | After: `6e0d0ca`, plain `Option<Box<T>>` |
| --- | --- | --- |
| User instructions | 1.189 slower | 0.99998 equivalent |
| Wall time | 1.193 [1.136, 1.253] slower | 1.022 [0.966, 1.082] inconclusive |
| Peak memory | 1.001 [0.995, 1.006] inconclusive | 0.999 [0.993, 1.006] inconclusive |

- **`binary-trees`**: the same instructions as hand-written Rust; wall
  time, cycles and peak memory cannot be told apart from Rust's at this
  precision. Plain Go is still the fastest on wall time, 0.90 times
  Rust's, its collector running concurrently with the program.
- **`cycles`**: the case was rewritten (#135), so it does not compare with
  earlier runs. GoPatina executes 6% fewer instructions than the
  hand-written Rust, for a cause not yet established, and is faster on
  wall time.
- **`fib`, `suffix-sum`**: the same instructions as hand-written Rust.
  `fib`'s 10% fewer cycles than Rust with the same instructions has
  appeared in every run; its cause is not established, and it is not taken
  as GoPatina being faster than Rust.

**`sieve` and the JCC erratum.** GoPatina's `sieve` loop and the
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

## Previous official run: three arms, after owned pointers, 2026-11-23

GoPatina (`gopatina build -patina`), hand-written Rust and plain Go
(`go build`) on all five benchmarks: `bench/run.sh --equiv --metric perf
--runs 100 --toolchain same --rust stable`, 100 rounds in ABCCBA order,
GoPatina commit `477aaa3a1d180848aeb3d19e36e3a3cf69d2a9b6` (with #125's
owned pointers), rustc 1.100.0 for both Rust binaries, Go 1.27.1, on the
same Xeon Gold 6152 virtual server as the runs below, with the correctness
run paused (load averages 3.59, 6.93, 7.89 at the start and 1.28, 1.21,
1.89 at the end). Every table, both pairs not shown here and the reading
of each case are in the [run notes](bench/results/2026-11-23-server-477aaa3-three-arm.md); raw results:
[2026-11-23-server-477aaa3-three-arm.json](bench/results/2026-11-23-server-477aaa3-three-arm.json).

Each program, geometric mean with its 5σ interval:

| Benchmark | Program | Wall time | User instructions | Peak memory | Binary size |
| --- | --- | --- | --- | --- | --- |
| `fib` | Go | 2.44 s [2.34, 2.56] | 12.57 G | 1.82 MiB [1.72, 1.93] | 2,287 KiB |
| `fib` | GoPatina | 1.27 s [1.21, 1.33] | 8.505 G | 2.01 MiB [1.90, 2.13] | 339 KiB |
| `fib` | Rust | 1.42 s [1.35, 1.49] | 8.505 G | 1.94 MiB [1.84, 2.05] | 435 KiB |
| `sieve` | Go | 0.272 s [0.253, 0.292] | 0.3846 G | 21.1 MiB [20.9, 21.3] | 2,288 KiB |
| `sieve` | GoPatina | 0.268 s [0.250, 0.286] | 0.3846 G | 20.9 MiB [20.9, 20.9] | 339 KiB |
| `sieve` | Rust | 0.266 s [0.248, 0.284] | 0.3846 G | 20.9 MiB [20.9, 20.9] | 435 KiB |
| `suffix-sum` | Go | 0.322 s [0.302, 0.344] | 0.4349 G | 155 MiB [155, 155] | 2,288 KiB |
| `suffix-sum` | GoPatina | 0.304 s [0.289, 0.321] | 0.3227 G | 154 MiB [154, 154] | 339 KiB |
| `suffix-sum` | Rust | 0.306 s [0.287, 0.325] | 0.3227 G | 154 MiB [154, 154] | 436 KiB |
| `binary-trees` | Go | 3.45 s [3.34, 3.56] | 33.50 G [33.44, 33.57] | 34.8 MiB [33.2, 36.4] | 2,305 KiB |
| `binary-trees` | GoPatina | 4.37 s [4.19, 4.56] | 22.79 G | 34.0 MiB [33.9, 34.1] | 346 KiB |
| `binary-trees` | Rust | 3.66 s [3.52, 3.81] | 19.16 G | 33.9 MiB [33.8, 34.1] | 437 KiB |
| `cycles` | Go | 0.163 s [0.152, 0.175] | 0.6459 G [0.6373, 0.6547] | 7.50 MiB [7.19, 7.81] | 2,288 KiB |
| `cycles` | GoPatina | 0.458 s [0.433, 0.485] | 0.9095 G | 185 MiB [185, 185] | 346 KiB |
| `cycles` | Rust | 0.225 s [0.213, 0.239] | 0.9249 G | 2.96 MiB [2.87, 3.06] | 438 KiB |

Instruction intervals not shown are narrower than the digits given. Binary
sizes are each tool's default: GoPatina's are stripped, the others' are not.

GoPatina against each, ratio with its 5σ interval and TOST verdict at ±0.5%:

| Benchmark | GoPatina / Rust, wall | GoPatina / Rust, instructions | GoPatina / Go, wall | GoPatina / Go, peak memory |
| --- | --- | --- | --- | --- |
| `fib` | 0.896 [0.851, 0.943] faster | 1.00000 equivalent | 0.519 [0.491, 0.548] faster | 1.103 [1.034, 1.177] larger |
| `sieve` | 1.007 [0.944, 1.076] inconclusive | 1.00001 equivalent | 0.985 [0.910, 1.067] inconclusive | 0.988 [0.979, 0.998] inconclusive |
| `suffix-sum` | 0.995 [0.950, 1.043] inconclusive | 1.00001 equivalent | 0.945 [0.894, 0.998] inconclusive | 0.997 [0.996, 0.998] equivalent |
| `binary-trees` | 1.193 [1.136, 1.253] slower | 1.189 slower | 1.267 [1.208, 1.329] slower | 0.977 [0.933, 1.023] inconclusive |
| `cycles` | 2.033 [1.926, 2.146] slower | 0.983 faster | 2.807 [2.648, 2.975] slower | 24.7 [23.7, 25.7] larger |

What owned pointers changed, against the
[first three-arm run](bench/results/2026-11-23-server-d207c32-three-arm.md) on the same server and toolchains:

| `binary-trees`, GoPatina / Rust | Before: `d207c32`, shared cells | After: `477aaa3`, owned pointers |
| --- | --- | --- |
| Wall time | 1.496 [1.422, 1.574] slower | 1.193 [1.136, 1.253] slower |
| User instructions | 1.372 slower | 1.189 slower |
| Peak memory | 1.473 [1.467, 1.480] larger | 1.001 [0.995, 1.006] inconclusive |

- **`binary-trees`**: peak memory is Rust's, its interval reaching only
  0.1% past the ±0.5% margin. Wall time and instructions are both about
  1.19 times Rust's; the cause of the remaining instructions is not yet
  measured. Plain Go is still the fastest, 0.94 times Rust's wall time,
  its collector running concurrently with the program.
- **`cycles`**: unchanged. Its lists are shared, so they stay
  reference-counted cells that are never freed (RFC 0013's known
  limitation): 185 MiB at peak, and twice Rust's time spent on that fresh
  memory. RFC 0016's second step collects the cycles.
- **`fib`, `sieve`, `suffix-sum`**: the same instructions as hand-written
  Rust. `fib`'s 10% fewer cycles than Rust with the same instructions has
  appeared in every run; its cause is not established, and it is not taken
  as GoPatina being faster than Rust.

## The goal

Identical speed to equivalent hand-written Rust on every benchmark, built
with the latest stable Rust. That is a goal, not a claim.

| Bar | Definition | Test | Status |
| --- | --- | --- | --- |
| Speed | Equivalent to hand-written Rust within ±0.5% (ratio in (1/1.005, 1.005)) | TOST on paired log ratios at 5σ, one-sided α = Φ(−5) ≈ 2.87 × 10⁻⁷ per bound | `fib`, `sieve`, `suffix-sum`: **met on user instructions, the primary metric, at 5σ and 6σ** (2026-11-22, again 2026-11-23); wall time inconclusive or faster. `binary-trees`: **met on user instructions** (0.99998, 2026-11-24, RFC 0017); wall time inconclusive. `cycles` (rewritten by #135): faster than Rust on instructions and wall time (2026-11-24) |
| Correctness (Six Sigma) | At most 3.4 defects per million tests (4.5σ, after the conventional 1.5σ shift), shown at 6σ confidence | 0 defects in ≥ 6,099,050 differential tests against Go: one-sided upper bound 1 − α^(1/N) < −ln(α) / N with α = Φ(−6) ≈ 9.87 × 10⁻¹⁰. Milestone: 0 defects in ≥ 881,098 tests, the same bound at 95% | Target; both the milestone and the bar are pending on the test server |

Hypotheses, formulas and their limits are in
[docs/QUALITY.md](docs/QUALITY.md#the-two-bars). "Identical" means equivalent within ±0.5% at 5σ: in a two one-sided tests (TOST) equivalence test on paired, interleaved runs, the whole 5σ interval of the ratio lies inside ±0.5%.
How the test works is in [bench/README.md](bench/README.md).

## The speed bar on the test server (2026-11-22): met on user instructions

Two runs with CPU counters (enabled for them by the project lead), both
Rust binaries built by rustc 1.100.0 (`--toolchain same`, the official
comparison; the rustc is read from each binary), 100 runs of each binary in
ABBA order, GoPatina commit `bfed1e17ebec1a43e7acc09c833986bda5d3e031`, on
the same Xeon Gold 6152 virtual server as the pilot below. Notes, load and
the asm check are in the [run notes](bench/results/2026-11-22-server-bfed1e1-same.md);
raw results: [5σ](bench/results/2026-11-22-server-bfed1e1-same-5sigma.json),
[6σ](bench/results/2026-11-22-server-bfed1e1-same-6sigma.json).

| Benchmark | User instructions, 5σ | User instructions, 6σ | Verdict |
| --- | --- | --- | --- |
| `fib` | 1.0000083 [1.0000077, 1.0000089] | 1.0000082 [1.0000074, 1.0000090] | equivalent |
| `sieve` | 1.0000064 [1.0000058, 1.0000069] | 1.0000062 [1.0000056, 1.0000069] | equivalent |
| `suffix-sum` | 1.0000072 [1.0000066, 1.0000078] | 1.0000071 [1.0000061, 1.0000082] | equivalent |

Every interval lies far inside ±0.5%: on its primary metric the speed bar is
met, at 5σ and at 6σ. The excess of 6 to 8 millionths is a fixed cost,
consistent with the startup panic-hook setup, the only difference in the
machine code (`bench/asm-check.sh --toolchain stable`).

**Run time is not decided yet.** The bar's hypotheses are stated for run
time, and user instructions count work, not time. Cycles and wall time were
measured too, but beside the long correctness run (load average 7.6 to 9.8),
which disturbs them: all were inconclusive except `fib`'s cycles, which read
about 9% fewer for GoPatina although both binaries execute the same
instructions; that is taken as an effect of the conditions, not as a
result. Deciding run time needs the same runs on a quiet machine.

## Test-server pilot (2026-11-22): inconclusive, from noise

The first run on the test server, the 100-run pilot that sizes the speed
bar's run: `bench/run.sh --equiv --runs 100`, wall time, TOST at ±0.5% and
5σ, GoPatina commit `0af9da87949b1e710ec7a00c4b3a24a79e9d7b49`, on an Intel
Xeon Gold 6152 virtual server (7 vCPUs, 7 GB, Linux 6.8.0-147, Go 1.27.1,
rustc 1.100.0 stable, emitted crates pinned to 1.93.0). Notes and load in
the [run notes](bench/results/2026-11-22-server-0af9da8-pilot.md); raw
results:
[2026-11-22-server-0af9da8-pilot.json](bench/results/2026-11-22-server-0af9da8-pilot.json).

| Benchmark | GoPatina / Rust, wall time | 5σ interval | sd of log ratios | Verdict |
| --- | --- | --- | --- | --- |
| `fib` | 0.984 | [0.948, 1.021] | 0.069 | inconclusive |
| `sieve` | 0.975 | [0.874, 1.087] | 0.204 | inconclusive |
| `suffix-sum` | 1.007 | [0.961, 1.056] | 0.087 | inconclusive |

The wall-time noise on this machine (7% to 20%) is far wider than the ±0.5%
margin: even with no true difference, the bar would need about 4,800, 41,900
and 7,700 pairs at 5σ. The pilot makes no speed claim. The bar's run waits
for CPU counters on the server, or a quieter dedicated machine.

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

## Allocators for static musl binaries, 2026-11-27 (not official)

GoPatina's crates for the three cases that allocate in their hot loop,
built with glibc's malloc (today's default), with musl's own allocator
(static), with mimalloc, and with the pure-Rust dlmalloc (static). The
median of 7 interleaved runs on a laptop in use, so these are directions.
Method and peak memory:
[2026-11-27-allocators.md](bench/results/2026-11-27-allocators.md).

| Case | glibc | musl | mimalloc* | dlmalloc |
| --- | --- | --- | --- | --- |
| `binary-trees` | 3.71 s | 4.64 s | 1.03 s | 5.04 s |
| `iface-keep` | 0.21 s | 0.49 s | 0.10 s | 0.52 s |
| `cycles` | 0.02 s | 0.11 s | 0.02 s | 0.02 s |

\* Built for gnu: mimalloc is C, and building it for musl needs a musl C
compiler.

musl's allocator is 1.25 to 2.3 times slower than glibc's on these cases,
and dlmalloc does not help. mimalloc is fastest, but it brings a C toolchain
into static builds. So `-static` (Rust's musl target, no C toolchain) is
opt-in, and gnu stays the default until a fast allocator for static builds
is settled.

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

## History

- 2026-11-24: the three-arm run after one owner per pointer (RFC 0017, steps 1 to 5; test server): `binary-trees` the same instructions as Rust, wall time and memory inconclusive; `cycles`, rewritten, faster than Rust; the `sieve` JCC note.
- 2026-11-23: the three-arm run after owned pointers (RFC 0016, step 1; test server): `binary-trees` peak memory equal to Rust's, wall time 1.19 times Rust's, down from 1.50; `cycles` unchanged.
- 2026-11-23: the first three-arm run (GoPatina, hand-written Rust, plain Go) on all five benchmarks (test server): `binary-trees` and `cycles` slower than both.
- 2026-11-22: the speed bar met on user instructions at 5σ and 6σ (test server, one rustc for both); run time pending.
- 2026-11-22: the test-server pilot, wall time, inconclusive from noise.
- 2026-11-21: a local equivalence run with CPU counters, a wall-clock run and a machine-code comparison (not official).
- 2026-11-18: two local verification runs (not official).
