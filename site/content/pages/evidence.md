# The evidence: GoPatina's speed and correctness, as a study

GoPatina makes two measurable claims, and this page reports them the way a scientific paper would: what is tested, how, on which machines, with which statistics, what has been measured so far, and what could make the results wrong. **The speed bar is met on its primary metric, user instructions, at 5σ and at 6σ (test server, 2026-11-22); on run time it is not yet decided, and the correctness bar is pending.** Nothing here claims more than was measured. Summaries are on the [benchmarks](/benchmarks) and [correctness](/correctness) pages.

## 1. Hypotheses {#hypotheses}

**Speed.** For each benchmark, let μ be the mean of the paired log ratios ln(T<sub>GoPatina</sub> / T<sub>Rust</sub>), and δ = ln 1.005 ≈ 0.0049875, a margin of about ±0.5% on the ratio. H₀: |μ| ≥ δ, the two differ by at least the margin. H₁: |μ| < δ, they are equivalent. "Identical speed" means H₀ is rejected at 5σ.

**Correctness.** Let p be the probability that a test is a defect, and p₀ = 3.4 × 10⁻⁶, the Six Sigma rate of 3.4 defects per million opportunities (DPMO). H₀: p ≥ p₀. H₁: p < p₀, shown at 6σ confidence.

## 2. Method {#method}

- **Speed programs.** A benchmark suite (`fib`, `sieve`, `suffix-sum`), each written in Go and as idiomatic hand-written Rust that is not tuned to flatter GoPatina. All three builds must print the same expected output before any timing.
- **Correctness programs.** Random programs from GoPatina's generator (`internal/progen`): valid Go inside the phase 1 subset, with wrapping integer arithmetic, shifts, slices, `append`, `switch`, formatted printing, and the loop nests RFC 0006 specializes, some at exactly their thresholds. Each batch ends with a panic case: an index out of range, a division by zero, a `make` out of range, an explicit `panic`, or a nest one past its threshold.
- **The differential contract.** A test passes when the `-patina` binary's standard output, first line of standard error and exit status equal those of the same program built by Go's reference build, `go build -gcflags=all=-N -l` (optimization and inlining off, the closest the gc compiler comes to the language specification). A difference, or a failure to build, is a defect. Each program is also built with plain `go build`; where that optimized build disagrees with the reference build, the program is recorded as a Go compiler disagreement, not a GoPatina defect.
- **Metrics.** Wall time per run by default, and user instructions and cycles from the CPU's counters where the machine allows them. Also the machine code of each benchmark's hot functions (`bench/asm-check.sh`), which no timing noise affects.
- **Standing checks.** 98 hand-written golden programs and 26 rejection and semantics fixtures run in CI on every change, with 100% unit-test coverage. Every change, including the maintainer's own, goes through a pull request that the project lead approves.

## 3. Apparatus {#apparatus}

| Aspect | Local runs (not official) | Test server (official) |
| --- | --- | --- |
| Machine | Intel Core i7-8650U, 8 logical CPUs, 60 GiB RAM (a laptop) | Intel Xeon Gold 6152 at 2.10 GHz, 7 vCPUs, 7 GB RAM (a rented virtual server) |
| Operating system | Linux 7.0.0-43 x86_64 | Ubuntu 24.04, Linux 6.8.0 |
| Toolchains | Go 1.27.1; rustc 1.93.0 (latest stable on the machine, and the version emitted crates pin) | Go 1.27.1; Rust 1.100.0 stable, and 1.93.0 for emitted crates |
| Load | in normal use: load average 2 to 37 on 8 CPUs | the pilot: load average 0.2 to 2.3; the counter runs: 7.6 to 9.8, beside the long correctness run, which leaves instruction counts unchanged but disturbs cycles and wall time |
| Recorded with each result | commit, versions, date, load | commit, versions, CPU model, date, load |

## 4. Statistical analysis {#statistics}

**Speed: two one-sided tests (TOST).** The two binaries run alternately in ABBA order after warm-up runs, so slow drift affects both alike. From the N paired log ratios, with mean μ̂ and standard deviation s, the (1 − 2α) confidence interval is μ̂ ± t<sub>1−α, N−1</sub> · s / √N, with α = Φ(−5) ≈ 2.87 × 10⁻⁷ for each one-sided test. The benchmark is *equivalent* when the whole interval lies inside (−δ, δ), *slower* or *faster* when it lies wholly beyond, and *inconclusive* otherwise. N is chosen from a 100-run pilot as N ≈ (5 · s / (δ − |μ̂|))², so that the interval can fit inside the margin. Across the three benchmarks, the chance of any false "equivalent" is at most 3α ≈ 8.6 × 10⁻⁷ (Bonferroni).

**Correctness: a zero-defect bound at 6σ.** With 0 defects in N independent tests, the exact one-sided upper bound on p at confidence 1 − α is 1 − α<sup>1/N</sup>, which is below −ln(α) / N. H₀ is rejected at level α when N ≥ −ln(α) / p₀. The bar uses α = Φ(−6) ≈ 9.87 × 10⁻¹⁰, 6σ confidence, so it needs 0 defects in at least **6,099,050** tests. On the way there, 0 defects in 881,098 tests show the same rate at 95% confidence (α = 0.05, where −ln(α) / N ≈ 3 / N is the rule of three); that is reported as a milestone, not as the bar. The 3.4 per million comes from the Six Sigma convention: a mean 6σ from its limit, less the conventional 1.5σ long-term drift, leaves 4.5σ, and P(Z > 4.5) ≈ 3.4 × 10⁻⁶.

## 5. Results {#results}

| Measured | Result | When and where | Standing |
| --- | --- | --- | --- |
| `fib` speed | user instructions 1.0000024 [1.0000018, 1.0000029]: equivalent; cycles 1.069 [1.001, 1.140] and wall time 1.043 [0.977, 1.114]: inconclusive | 2026-11-21, laptop, commit `6cb858f`, CPU counters, 300 runs per binary, 5σ, tested at ±1% (equivalent at ±0.5% too) | not official |
| `sieve` speed | instructions 1.302; cycles 1.073 [1.055, 1.092]; wall time 1.067 [1.025, 1.110]: slower on all three | same run | not official; the cause has since been removed (next row) |
| `suffix-sum` speed | instructions 1.0000020 [1.0000015, 1.0000026]: equivalent; cycles 0.992 [0.962, 1.022] and wall time 0.977 [0.880, 1.084]: inconclusive | same run | not official |
| Machine code after RFC 0006 | the hot loops of all three benchmarks are the same instructions as the hand-written Rust's; what remains is a 3-instruction panic-hook setup at startup (and one alignment no-op in `sieve`) | 2026-11-21, rustc 1.93.0 | compiles only; not a speed measurement |
| Test-server pilot, wall time | `fib` 0.984 [0.948, 1.021], `sieve` 0.975 [0.874, 1.087], `suffix-sum` 1.007 [0.961, 1.056]: all inconclusive; the standard deviation of the log ratios was 7% to 20% | 2026-11-22, test server, commit `0af9da8`, 100 runs per binary, 5σ | the noise is far wider than ±0.5%: wall time on this machine cannot decide the bar |
| Test server, user instructions (the primary metric), one rustc (1.100.0) for both | `fib` 1.0000082 [1.0000074, 1.0000090], `sieve` 1.0000062 [1.0000056, 1.0000069], `suffix-sum` 1.0000071 [1.0000061, 1.0000082] at 6σ: all equivalent; the 5σ run agrees | 2026-11-22, test server, commit `bfed1e1`, CPU counters, 100 runs per binary | the speed bar met on its primary metric |
| Test server, cycles and wall time, the same runs | all inconclusive, except `fib`'s cycles at 0.903 [0.881, 0.926], although both binaries execute the same instructions | the same runs, under load | not taken as a result: these metrics need a quiet machine |
| Machine code, rustc 1.100.0 | `fib`'s hot function identical; each `main` differs only by the startup panic-hook setup | 2026-11-22, test server | compiles only; not a speed measurement |
| **The speed bar** (±0.5%, at 5σ and 6σ) | met on user instructions; on run time not yet decided | test server | **met on the primary metric**; run time pending a quiet run |
| Golden programs | 98 of 98 match the Go reference build's output, first stderr line and exit status | every change, in CI | passing |
| Development smoke runs | 1,318 generated tests, 0 defects | 2026-11-21, laptop, three generator versions | far below 881,098: not evidence for the bar |
| Defects found by the generated tests | 2: a shift after a type conversion did not compile; a shift by a constant at least the operand's width gave Rust's result instead of Go's (both fixed, with golden programs) | 2026-11-22: test server, programs 150 to 299; laptop, new generator cases before merging | the run restarts with the fixed compiler |
| Go compiler disagreements found by the generated tests | 2: optimized gc builds on amd64 (one from Go 1.22.0, the other from 1.26.0, both up to 1.27.1) printed results that contradict the specification and a hand calculation; the reference build and GoPatina agreed with both | 2026-11-22, test server, programs 169026 and 174986 | not GoPatina defects; the Go oracle is now the reference build, and both programs are golden tests |
| Correctness milestone (≤ 3.4 DPMO at 95% confidence) | pending: 0 defects needed in at least 881,098 tests | test server | pending |
| **The correctness bar** (≤ 3.4 DPMO at 6σ confidence) | pending: 0 defects needed in at least 6,099,050 tests | test server | pending |

Brackets are 5σ intervals of the ratio GoPatina / hand-written Rust; below 1 is faster. Three defects have been found and fixed so far: by hand, a `make` with a negative length known only at run time panicked with Rust's message instead of Go's; by the generated tests, an expression such as `a*b << 27` whose left side ended in a conversion did not compile as Rust; and by new generator cases, a shift by a constant at least the operand's width gave Rust's result instead of Go's. Each fix came with golden programs and a generator case. Raw results and run notes are in [BENCHMARKS.md](https://github.com/pigfox/gopatina/blob/main/BENCHMARKS.md).

## 6. Threats to validity and limitations {#limitations}

- **Scope.** Everything here holds for the phase 1 subset only: no goroutines, maps, interfaces, generics or most of the standard library.
- **The benchmarks.** Three small programs, with one author's hand-written Rust. They may not represent real workloads.
- **Machines.** The local results come from a busy laptop. The test server is one rented virtual machine of one kind, whose wall-time noise (7% to 20% in the pilot) is far above the speed margin.
- **Measurement.** A wall-time run includes process start-up, which dilutes a difference in a short benchmark. Instruction counts are not time: the hypotheses are stated for run time, so the speed bar is met on its primary metric, not yet on time.
- **Toolchains.** Different rustc releases compile the same source differently, so the official comparison builds GoPatina's crate and the hand-written Rust with the same rustc, the latest stable; GoPatina's crates otherwise pin rustc 1.93.0. A comparison across releases measures the compilers, not GoPatina.
- **The generator.** The correctness bound holds for programs the generator produces. A kind of bug the generator never triggers is invisible to it, and its tests are not independent draws from real programs. Only the first line of standard error is compared, not Go's stack trace.

## 7. Reproducibility {#reproducibility}

Every result names its commit, toolchains and machine, and its raw JSON is in [`bench/results`](https://github.com/pigfox/gopatina/tree/main/bench/results). From a checkout of that commit:

```sh
bench/run.sh --equiv --runs 100 --out pilot.json          # pilot: read sd_log_ratio
bench/run.sh --equiv --runs N --out equiv.json            # the TOST run, N from the pilot
bench/asm-check.sh                                        # the machine-code comparison
go build -o gopatina ./cmd/gopatina && go build -o gopatina-difftest ./cmd/gopatina-difftest
./gopatina-difftest -seed 1 -programs 6076000 -per 150 -jobs 6 -gopatina ./gopatina >difftest.json   # 6,116,548 tests
```

Emitted Rust is byte-identical for the same input, and `gopatina verify` checks that a `-patina` binary has the same sha256 as a plain `cargo build --release --locked` of the emitted crate. The formulas are also in [docs/QUALITY.md](https://github.com/pigfox/gopatina/blob/main/docs/QUALITY.md#the-two-bars).

## 8. References {#references}

- D. J. Schuirmann (1987). A comparison of the two one-sided tests procedure and the power approach for assessing the equivalence of average bioavailability. *Journal of Pharmacokinetics and Biopharmaceutics* 15(6), 657–680.
- J. A. Hanley and A. Lippman-Hand (1983). If nothing goes wrong, is everything all right? Interpreting zero numerators. *JAMA* 249(13), 1743–1745.
- M. J. Harry (1988). *The Nature of Six Sigma Quality*. Motorola University Press. The source of the 3.4 DPMO figure and the 1.5σ shift convention.
