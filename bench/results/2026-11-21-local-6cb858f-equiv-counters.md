# Local equivalence run with CPU counters — 2026-11-21

**Not official.** The dedicated test server will produce official numbers.
This run was made on the maintainer's laptop, with CPU performance counters
that the project lead had enabled for it on 2026-11-21
(`kernel.perf_event_paranoid` set to 1; they are normally off). The machine
was in normal use, with a load average between 11 and 21 during the run.

| Field | Value |
| --- | --- |
| Date | 2026-11-21, 18:10 to 18:15 (UTC−8) |
| GoPatina | `6cb858fff4bc8a1a659ba240a452d6d45ea6a5c0` (compiler); harness `cmd/gopatina-bench` as added in #33 |
| Machine | Intel(R) Core(TM) i7-8650U CPU @ 1.90GHz, 8 logical CPUs, 60 GiB RAM |
| OS | Linux 7.0.0-43-generic x86_64 |
| Go | go version go1.27.1 linux/amd64 |
| Rust | rustc 1.93.0 (254b59607 2026-01-19); latest stable on the machine and the version GoPatina's emitted crates pin |
| Method | `bench/run.sh --equiv --runs 300`: 300 runs of each binary in ABBA order after 2 warm-ups each; each run under `perf stat -e instructions:u,cycles:u`, with its wall time measured around it; TOST equivalence test per metric on the paired log ratios, margin ±1%, 5σ (one-sided α ≈ 2.87e-7) |
| Raw results | [2026-11-21-local-6cb858f-equiv-counters.json](2026-11-21-local-6cb858f-equiv-counters.json) |

This run was tested at a ±1% margin, the default when it was made. The
project lead has since set the margin to ±0.5% (2026-11-21); the intervals
are read against both below.

GoPatina / hand-written Rust, with the 5σ interval; verdicts at ±1%:

| Case | User instructions | User cycles | Wall time |
| --- | --- | --- | --- |
| `fib` | 1.0000024 [1.0000018, 1.0000029]: equivalent | 1.069 [1.001, 1.140]: inconclusive | 1.043 [0.977, 1.114]: inconclusive |
| `sieve` | 1.302 [1.302, 1.302]: slower | 1.073 [1.055, 1.092]: slower | 1.067 [1.025, 1.110]: slower |
| `suffix-sum` | 1.0000020 [1.0000015, 1.0000026]: equivalent | 0.992 [0.962, 1.022]: inconclusive | 0.977 [0.880, 1.084]: inconclusive |

What it shows:

- **`fib` and `suffix-sum`** execute the same instructions as the
  hand-written Rust to within 0.0003%. The run-to-run spread of that small
  excess is nearly zero, so it is a fixed cost. That fits the one-time
  startup call that installs GoPatina's Go-style panic hook, the only
  difference the machine-code comparison found; their hot loops are
  identical. Cycles and wall time are inconclusive at ±1%. On a machine this
  busy their spread is far wider than the margin.
- **`sieve`** executes 1.302 times the instructions, and is about 7% slower
  in cycles and in wall time, at 5σ on all three metrics. The cause is the
  extra bounds check in the generated inner loop, which keeps Go's
  wrap-around and index semantics (Go's `int` becomes `i64`, indexed as
  `usize`). Closing that gap without changing Go's semantics is being worked
  on.

**Against ±0.5%.** The verdicts do not change. The `fib` and `suffix-sum`
instruction intervals lie well inside ±0.5%, so they are equivalent at the
stricter margin too. All three `sieve` intervals lie wholly above +0.5%, so
it is slower. The cycle and wall-time intervals of `fib` and `suffix-sum` are
far wider than ±0.5%, so they stay inconclusive. Deciding those needs a quiet
machine and enough runs (bench/README.md, "How many runs").
