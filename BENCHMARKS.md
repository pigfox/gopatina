# Benchmarks

This is where GoPatina's official benchmark results are published: GoPatina
against equivalent hand-written Rust and plain Go, measured on the
maintainer's test server. How the numbers are produced is described in
[docs/QUALITY.md](docs/QUALITY.md#performance-shadow-rust-and-keep-up) and
[bench/README.md](bench/README.md).

> **Latest, 2026-11-23: three arms on all five benchmarks.** On `fib`,
> `sieve` and `suffix-sum`, GoPatina executes the same user instructions as
> hand-written Rust, and on `fib` it takes 0.53 times plain Go's time. On
> the two pointer benchmarks it is **slower than both**: `binary-trees` 1.50
> times Rust's wall time and 1.59 times Go's, `cycles` 2.00 and 2.70 times,
> and plain Go is the fastest of the three on both. The causes are measured
> and [RFC 0016](docs/rfcs/0016-owned-pointers-and-cycle-collection.md)
> fixes them. The correctness bar is pending. The local runs further down
> are earlier evidence from the maintainer's workstation, not official
> results.

## Latest official run: three arms, 2026-11-23

GoPatina (`gopatina build -patina`), hand-written Rust and plain Go
(`go build`) on all five benchmarks: `bench/run.sh --equiv --metric perf
--runs 100 --toolchain same --rust stable`, 100 rounds in ABCCBA order,
GoPatina commit `d207c32cca6f6aee84a4c7d272c9974bf97e65ad`, rustc 1.100.0
for both Rust binaries, Go 1.27.1, on the same Xeon Gold 6152 virtual
server as the runs below, with the correctness run paused. Every table,
both pairs not shown here and the reading of each case are in the
[run notes](bench/results/2026-11-23-server-d207c32-three-arm.md); raw
results:
[2026-11-23-server-d207c32-three-arm.json](bench/results/2026-11-23-server-d207c32-three-arm.json).

Each program, geometric mean with its 5σ interval:

| Benchmark | Program | Wall time | User instructions | Peak memory | Binary size |
| --- | --- | --- | --- | --- | --- |
| `fib` | Go | 2.60 s [2.50, 2.71] | 12.57 G | 1.81 MiB [1.70, 1.91] | 2,287 KiB |
| `fib` | GoPatina | 1.37 s [1.30, 1.44] | 8.505 G | 1.99 MiB [1.88, 2.10] | 339 KiB |
| `fib` | Rust | 1.51 s [1.44, 1.59] | 8.505 G | 1.94 MiB [1.83, 2.04] | 435 KiB |
| `sieve` | Go | 0.266 s [0.251, 0.282] | 0.3847 G | 21.2 MiB [21.1, 21.3] | 2,288 KiB |
| `sieve` | GoPatina | 0.258 s [0.243, 0.273] | 0.3846 G | 20.9 MiB [20.9, 20.9] | 339 KiB |
| `sieve` | Rust | 0.262 s [0.249, 0.276] | 0.3846 G | 20.9 MiB [20.9, 20.9] | 435 KiB |
| `suffix-sum` | Go | 0.311 s [0.296, 0.327] | 0.4349 G | 155 MiB [155, 155] | 2,288 KiB |
| `suffix-sum` | GoPatina | 0.302 s [0.286, 0.318] | 0.3227 G | 154 MiB [154, 154] | 339 KiB |
| `suffix-sum` | Rust | 0.300 s [0.286, 0.315] | 0.3227 G | 154 MiB [154, 154] | 436 KiB |
| `binary-trees` | Go | 3.62 s [3.48, 3.77] | 33.52 G [33.46, 33.59] | 34.5 MiB [32.6, 36.6] | 2,305 KiB |
| `binary-trees` | GoPatina | 5.76 s [5.47, 6.06] | 26.29 G | 50.0 MiB [49.9, 50.2] | 347 KiB |
| `binary-trees` | Rust | 3.85 s [3.65, 4.05] | 19.16 G | 33.9 MiB [33.8, 34.1] | 437 KiB |
| `cycles` | Go | 0.168 s [0.159, 0.177] | 0.6446 G [0.6386, 0.6507] | 7.58 MiB [6.91, 8.32] | 2,288 KiB |
| `cycles` | GoPatina | 0.453 s [0.430, 0.477] | 0.9095 G | 185 MiB [185, 185] | 346 KiB |
| `cycles` | Rust | 0.227 s [0.213, 0.241] | 0.9249 G | 2.95 MiB [2.85, 3.05] | 438 KiB |

Instruction intervals not shown are narrower than the digits given. Binary
sizes are each tool's default: GoPatina's are stripped, the others' are not.

GoPatina against each, ratio with its 5σ interval and TOST verdict at ±0.5%:

| Benchmark | GoPatina / Rust, wall | GoPatina / Rust, instructions | GoPatina / Go, wall | GoPatina / Go, peak memory |
| --- | --- | --- | --- | --- |
| `fib` | 0.904 [0.868, 0.941] faster | 1.00000 equivalent | 0.526 [0.498, 0.554] faster | 1.100 [1.013, 1.195] larger |
| `sieve` | 0.984 [0.931, 1.039] inconclusive | 1.00001 equivalent | 0.968 [0.909, 1.031] inconclusive | 0.985 [0.980, 0.990] smaller |
| `suffix-sum` | 1.006 [0.960, 1.054] inconclusive | 1.00001 equivalent | 0.971 [0.920, 1.025] inconclusive | 0.997 [0.996, 0.998] equivalent |
| `binary-trees` | 1.496 [1.422, 1.574] slower | 1.372 slower | 1.591 [1.523, 1.662] slower | 1.448 [1.366, 1.535] larger |
| `cycles` | 1.999 [1.890, 2.114] slower | 0.983 faster | 2.701 [2.532, 2.882] slower | 24.4 [22.2, 26.8] larger |

- **`fib`, `sieve`, `suffix-sum`**: the same instructions as hand-written
  Rust. `fib`'s 10% fewer cycles than Rust with the same instructions also
  appeared on 2026-11-22; its cause is not established, and it is not taken
  as GoPatina being faster than Rust.
- **`binary-trees`**: plain Go is the fastest, 0.94 times Rust's wall time,
  its collector running concurrently with the program. GoPatina's shared
  `Rc<RefCell<T>>` cells, where the hand-written Rust uses `Box`, cost it
  1.50 times Rust's time.
- **`cycles`**: plain Go is the fastest, 0.74 times Rust's wall time.
  GoPatina never frees the lists (RFC 0013's known limitation): 185 MiB at
  peak, and twice Rust's time spent on that fresh memory.

## The goal

Identical speed to equivalent hand-written Rust on every benchmark, built
with the latest stable Rust. That is a goal, not a claim.

| Bar | Definition | Test | Status |
| --- | --- | --- | --- |
| Speed | Equivalent to hand-written Rust within ±0.5% (ratio in (1/1.005, 1.005)) | TOST on paired log ratios at 5σ, one-sided α = Φ(−5) ≈ 2.87 × 10⁻⁷ per bound | `fib`, `sieve`, `suffix-sum`: **met on user instructions, the primary metric, at 5σ and 6σ** (2026-11-22, again 2026-11-23); wall time inconclusive or faster. `binary-trees`, `cycles`: **not met**, 1.50 and 2.00 times Rust's wall time (2026-11-23; RFC 0016) |
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

- 2026-11-23: the first three-arm run (GoPatina, hand-written Rust, plain Go) on all five benchmarks (test server): `binary-trees` and `cycles` slower than both.
- 2026-11-22: the speed bar met on user instructions at 5σ and 6σ (test server, one rustc for both); run time pending.
- 2026-11-22: the test-server pilot, wall time, inconclusive from noise.
- 2026-11-21: a local equivalence run with CPU counters, a wall-clock run and a machine-code comparison (not official).
- 2026-11-18: two local verification runs (not official).
