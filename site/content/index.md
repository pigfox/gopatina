# GoPatina

> A patina is the thin layer that forms on metal: Go on the surface, Rust underneath.

**GoPatina is an open-source Go toolchain front end in which ordinary Go code can opt into running the way Rust code does: no garbage collector, ownership and borrowing, and Rust-grade memory safety.** Without the flag it is plain Go, and every existing Go program builds exactly as before.

**Mission: Go developers get Rust's performance and safety without the painful Rust learning curve.**

**Status: early. Phase 1 works for a defined subset of Go; real programs need features still in progress.** Default mode works for all Go. For the subset, `-emit-rust`, `-patina`, `-patina=check`, `-explain` and `gopatina verify` are implemented and tested, including read-only `s[i:]` suffix arguments; everything else is rejected with a clear error. No official performance or defect-rate figures yet; the evidence section below reports the method and every result so far.

Last updated 2026-11-21. Canonical page: https://gopatina.org/

## Why GoPatina

The language you write in matters more to you, day to day, than what runs underneath it. Go is small, readable and quick to learn, with fast builds and excellent tooling. Rust gives deterministic memory management with no garbage collector, and catches memory errors and data races at compile time, but it is powerful and painful to learn: ownership, borrowing, lifetimes and `async` are a steep climb.

GoPatina keeps Go on top and puts Rust underneath. You keep writing Go; the compiler does the Rust part. **GoPatina is an experiment by [Pigfox](https://pigfox.com) to get Go developers past Rust's learning curve.** Without a garbage collector a few ownership rules remain, but GoPatina states and checks them in Go terms: no Rust syntax, no lifetime annotations, no `async` keywords to learn.

## Three modes, two flags

| Mode | Flag | Garbage collector | Works with all Go | Output | Status |
| --- | --- | --- | --- | --- | --- |
| Default | none | Yes (Go's) | Yes | A Go binary, built by your installed `go` | Works today |
| Emit Rust | `-emit-rust` | No | No: a defined subset | A readable Rust crate you can keep and build with cargo | Works for the phase 1 subset |
| Patina | `-patina` | No | No: the same subset | A native binary built by rustc from that crate | Works for the phase 1 subset |

A variant, `-patina=check`, runs the same checks and then builds with the Go compiler, for machines where Rust cannot be installed. It is never a silent fallback: you choose it, and the output says which compiler built the binary.

## What phase 1 supports

The patina modes accept a defined subset of Go, specified in [docs/patina-subset.md](https://github.com/pigfox/gopatina/blob/main/docs/patina-subset.md):

- `package main` in one directory, with functions, methods and named struct types
- `int`, `int64`, `int32` (`rune`), `byte`, `float64`, `bool`, `string`, and slices of supported types, including read-only `s[i:]` suffix arguments
- `if`, `for` (including `for range` over slices, strings and integers), `switch`, constants and `iota`
- `error` results with `errors.New` and `fmt.Errorf`, and pointers used as borrows
- `fmt` printing (`Println`, `Printf`, `Sprintf`, `Fprintf` and the rest) and a few functions from `strings`, `slices`, `bytes` and `os`

**Not supported yet:** goroutines and channels, maps, closures and function values, interfaces other than `error` and `io.Writer`, generics, `defer`, general slicing (`s[i:j]`), slices with an explicit high or max bound, saved slice views, and most of the standard library. A narrow read-only suffix view, `read(s[i:])`, is supported when passed to a proven local reader. Every unsupported construct is rejected with a Go-style error that names the construct; nothing is translated approximately. See the [feature matrix](https://github.com/pigfox/gopatina/blob/main/docs/compat.md) and [every known place where Go and Rust semantics differ](https://github.com/pigfox/gopatina/blob/main/docs/go-rust-disconnects.md).

**How it is checked.** Each of the 92 test programs in the project's golden suite is built both ways in CI, with `go build` and with `gopatina build -patina`, and both binaries must print the same output, the same first stderr line and exit with the same status; 24 rejection fixtures and 3 Go-only semantics counterexamples pin down what must be refused. Emitted Rust is byte-identical for the same input, and `gopatina verify` checks that a `-patina` binary has the same sha256 as a plain `cargo build --release --locked` of the emitted crate.

## How it works

### What you write on top

Ordinary Go, formatted by `gofmt` and type-checked by Go's own libraries. Patina code is always valid Go, so the plain Go toolchain can always build it too. A package can keep a patina version beside a plain one with the `//go:build patina` constraint.

### What happens underneath

Both patina flags translate the Go program into readable Rust. `-emit-rust` stops there and writes a cargo crate. `-patina` goes on to build that crate with rustc, so the result is a Rust program: no garbage collector and memory safe. Calling Rust crates directly is planned. rustc and LLVM do the optimising, and Rust's own borrow checker verifies every program GoPatina writes.

### The GoPatina borrow checker

Before any Rust is written, GoPatina's own borrow checker runs on the Go code. It decides who owns each value and which values are moved, copied or borrowed, and applies fixes that keep the program's meaning. Phase 1 implements moves, copies of strings and small structs, and inferred shared and exclusive borrows; shared ownership and mutexes for values shared across goroutines come with goroutines. Every decision is reported with `-explain`; `gopatina fix`, which will write fixes back into the Go source, is planned. When no safe fix exists, the error is in Go terms. This is real output for the project's ownership example:

```text
./examples/ownership/main.go:33:2: m.Body is changed after m was handed to out.Queue on line 32
        out.Queue(m)     // line 32: out.Queue keeps m
        m.Body[0] = 'D'  // line 33: this also changes the copy out.Queue kept, which shares m.Body's memory
    patina mode does not copy m on its own: a copy would not share that memory, so the program could behave differently
    if out.Queue should keep the original, hand it a copy: Message{To: m.To, Body: bytes.Clone(m.Body)}
    or finish using m before handing it over
```

### Goroutines

Goroutines are not supported yet; this part is designed, not built. In patina mode, goroutines are designed to run as tasks on Tokio's multi-threaded runtime, the closest match to Go's own scheduler. Channels map to Tokio channels and `select` to `tokio::select!`. The compiler decides which functions must be `async`, so Go code never says `async` or `await`. Data races, which Go finds only at run time with `-race`, are to become compile errors or be fixed automatically.

### Portability

A patina binary is native code, so the target machine needs neither Go nor Rust installed. `GOOS` and `GOARCH` choose the Rust target. Phase 1 builds for each platform's default C runtime (the `-gnu` targets on Linux); static musl binaries by default, and cross-compiling with a fetched target, are planned.

### Compatibility as Go and Rust evolve

Default mode always uses your installed Go, so new Go releases work there immediately. Patina modes support the two latest Go releases and refuse, by name, any Go feature they do not support yet; they never miscompile silently. Generated Rust uses stable Rust only, a declared edition (2024) and a minimum supported Rust version (1.85), and CI tests against Go and Rust release channels every week.

## Try it today

Requires Go 1.25 or later; `-patina` and `gopatina verify` also need a Rust toolchain. Phase 1 is not released yet, so install from `main`:

```sh
go install github.com/pigfox/gopatina/cmd/gopatina@main

gopatina build ./...                        # runs go build ./...
gopatina build -emit-rust -o crate .        # writes a cargo crate
gopatina build -patina -explain -o app .    # native binary, with every decision
gopatina run -patina .                      # build and run it
gopatina build -patina=check .              # patina checks, then the Go compiler
gopatina verify .                           # reproducible: -patina == plain cargo
```

The patina commands work on programs inside the phase 1 subset. See [the examples](https://github.com/pigfox/gopatina/blob/main/EXAMPLES.md) for the three modes on three small programs: real output for the two that fit the subset, and the goroutine example's planned output clearly labelled.

## Roadmap

| Phase | Scope | Status |
| --- | --- | --- |
| 0 | The RFCs; the `gopatina` command with default-mode pass-through | Done (v0.0.1) |
| 1 | `-emit-rust`, `-patina`, `-patina=check` and `gopatina verify` for the [phase 1 subset](https://github.com/pigfox/gopatina/blob/main/docs/patina-subset.md), with the borrow checker and the Rust code generator | Done (unreleased) |
| 2 | The rest of the borrow checker: maps, closures, `defer`, general slicing, `gopatina fix`, the strict mode | Read-only slice suffix arguments done; rest planned |
| 3 | Goroutines and channels on a multi-threaded async runtime, with shared-ownership and mutex fixes | Planned |
| 4 | A standard library subset and Rust crate interop | Planned |

## The evidence: a report

GoPatina makes two measurable claims, and this section reports them the way a scientific paper would: what is tested, how, on which machines, with which statistics, what has been measured so far, and what could make the results wrong. **No official test-server run has happened yet.** Both bars are targets until the test server's runs meet them, and both results are pending. Nothing here claims more than was measured.

### 1. Hypotheses

**Speed.** For each benchmark, let μ be the mean of the paired log ratios ln(T<sub>GoPatina</sub> / T<sub>Rust</sub>), and δ = ln 1.005 ≈ 0.0049875, a margin of about ±0.5% on the ratio. H₀: |μ| ≥ δ, the two differ by at least the margin. H₁: |μ| < δ, they are equivalent. "Identical speed" means H₀ is rejected at 5σ.

**Correctness.** Let p be the probability that a test is a defect, and p₀ = 3.4 × 10⁻⁶, the Six Sigma rate of 3.4 defects per million opportunities (DPMO). H₀: p ≥ p₀. H₁: p < p₀.

### 2. Method

- **Speed programs.** A benchmark suite (`fib`, `sieve`, `suffix-sum`), each written in Go and as idiomatic hand-written Rust that is not tuned to flatter GoPatina. All three builds must print the same expected output before any timing.
- **Correctness programs.** Random programs from GoPatina's generator (`internal/progen`): valid Go inside the phase 1 subset, with wrapping integer arithmetic, shifts, slices, `append`, `switch`, formatted printing, and the loop nests RFC 0006 specializes, some at exactly their thresholds. Each batch ends with a panic case: an index out of range, a division by zero, a `make` out of range, an explicit `panic`, or a nest one past its threshold.
- **The differential contract.** A test passes when the `-patina` binary's standard output, first line of standard error and exit status equal those of the same program built by `go build`. A difference, or a failure to build, is a defect.
- **Metrics.** Wall time per run by default, and user instructions and cycles from the CPU's counters where the machine allows them. Also the machine code of each benchmark's hot functions (`bench/asm-check.sh`), which no timing noise affects.
- **Standing checks.** 92 hand-written golden programs and 27 rejection and semantics fixtures run in CI on every change, with 100% unit-test coverage. Every change, including the maintainer's own, goes through a pull request that the project lead approves.

### 3. Apparatus

| Aspect | Local runs (not official) | Test server (official) |
| --- | --- | --- |
| Machine | Intel Core i7-8650U, 8 logical CPUs, 60 GiB RAM (a laptop) | dedicated box, 7 cores, 7 GB RAM |
| Operating system | Linux 7.0.0-43 x86_64 | Ubuntu 24.04 |
| Toolchains | Go 1.27.1; rustc 1.93.0 (latest stable on the machine, and the version emitted crates pin) | Go 1.27.1; latest stable Rust, and 1.93.0 for emitted crates |
| Load | in normal use: load average 2 to 37 on 8 CPUs | quiet: no other job runs during a timed run |
| Recorded with each result | commit, versions, date, load | commit, versions, CPU model, date, load |

### 4. Statistical analysis

**Speed: two one-sided tests (TOST).** The two binaries run alternately in ABBA order after warm-up runs, so slow drift affects both alike. From the N paired log ratios, with mean μ̂ and standard deviation s, the (1 − 2α) confidence interval is μ̂ ± t<sub>1−α, N−1</sub> · s / √N, with α = Φ(−5) ≈ 2.87 × 10⁻⁷ for each one-sided test. The benchmark is *equivalent* when the whole interval lies inside (−δ, δ), *slower* or *faster* when it lies wholly beyond, and *inconclusive* otherwise. N is chosen from a 100-run pilot as N ≈ (5 · s / (δ − |μ̂|))², so that the interval can fit inside the margin. Across the three benchmarks, the chance of any false "equivalent" is at most 3α ≈ 8.6 × 10⁻⁷ (Bonferroni).

**Correctness: a zero-defect bound.** With 0 defects in N independent tests, the exact one-sided 95% upper bound on p is 1 − 0.05<sup>1/N</sup>, which is below −ln(0.05) / N ≈ 3 / N, the rule of three. H₀ is rejected at the 5% level when N ≥ −ln(0.05) / p₀ = **881,098**. The 3.4 per million comes from the Six Sigma convention: a mean 6σ from its limit, less the conventional 1.5σ long-term drift, leaves 4.5σ, and P(Z > 4.5) ≈ 3.4 × 10⁻⁶. The bar is at 95% confidence, the usual reading of Six Sigma. The same rate at the speed bar's confidence would need N ≥ −ln(α) / p₀: 4,430,882 defect-free tests at 5σ, or 6,099,050 at 6σ.

### 5. Results

| Measured | Result | When and where | Standing |
| --- | --- | --- | --- |
| `fib` speed | user instructions 1.0000024 [1.0000018, 1.0000029]: equivalent; cycles 1.069 [1.001, 1.140] and wall time 1.043 [0.977, 1.114]: inconclusive | 2026-11-21, laptop, commit `6cb858f`, CPU counters, 300 runs per binary, 5σ, tested at ±1% (equivalent at ±0.5% too) | not official |
| `sieve` speed | instructions 1.302; cycles 1.073 [1.055, 1.092]; wall time 1.067 [1.025, 1.110]: slower on all three | same run | not official; the cause has since been removed (next row) |
| `suffix-sum` speed | instructions 1.0000020 [1.0000015, 1.0000026]: equivalent; cycles 0.992 [0.962, 1.022] and wall time 0.977 [0.880, 1.084]: inconclusive | same run | not official |
| Machine code after RFC 0006 | the hot loops of all three benchmarks are the same instructions as the hand-written Rust's; what remains is a 3-instruction panic-hook setup at startup (and one alignment no-op in `sieve`) | 2026-11-21, rustc 1.93.0 | compiles only; not a speed measurement |
| **The speed bar** (±0.5%, 5σ) | pending | test server | pending |
| Golden programs | 92 of 92 match Go's output, first stderr line and exit status | every change, in CI | passing |
| Development smoke runs | 1,318 generated tests, 0 defects | 2026-11-21, laptop, three generator versions | far below 881,098: not evidence for the bar |
| **The correctness bar** (≤ 3.4 DPMO) | pending: 0 defects needed in at least 881,098 tests | test server | pending |

Brackets are 5σ intervals of the ratio GoPatina / hand-written Rust; below 1 is faster. One defect was found during this work by hand, not by the generator: a `make` with a negative length known only at run time panicked with Rust's message instead of Go's. It was fixed, with five golden programs, before the generator began testing panics. Raw results and run notes are in [BENCHMARKS.md](https://github.com/pigfox/gopatina/blob/main/BENCHMARKS.md).

### 6. Threats to validity and limitations

- **Scope.** Everything here holds for the phase 1 subset only: no goroutines, maps, interfaces, generics or most of the standard library.
- **The benchmarks.** Three small programs, with one author's hand-written Rust. They may not represent real workloads.
- **Machines.** The local results come from a busy laptop; only the test server's are official, and it is one machine of one kind.
- **Measurement.** A wall-time run includes process start-up, which dilutes a difference in a short benchmark. Instruction counts are not time.
- **Toolchains.** Emitted crates pin rustc 1.93.0, while the baseline uses the latest stable Rust, so a compiler change can move a ratio either way.
- **The generator.** The correctness bound holds for programs the generator produces. A kind of bug the generator never triggers is invisible to it, and its tests are not independent draws from real programs. Only the first line of standard error is compared, not Go's stack trace.
- **Confidence.** The correctness bar is at 95%, not 5σ (see above).

### 7. Reproducibility

Every result names its commit, toolchains and machine, and its raw JSON is in [`bench/results`](https://github.com/pigfox/gopatina/tree/main/bench/results). From a checkout of that commit:

```sh
bench/run.sh --equiv --runs 100 --out pilot.json          # pilot: read sd_log_ratio
bench/run.sh --equiv --runs N --out equiv.json            # the TOST run, N from the pilot
bench/asm-check.sh                                        # the machine-code comparison
go build -o gopatina ./cmd/gopatina && go build -o gopatina-difftest ./cmd/gopatina-difftest
./gopatina-difftest -seed 1 -programs 882000 -per 150 -jobs 6 -gopatina ./gopatina >difftest.json
```

Emitted Rust is byte-identical for the same input, and `gopatina verify` checks that a `-patina` binary has the same sha256 as a plain `cargo build --release --locked` of the emitted crate.

### 8. References

- D. J. Schuirmann (1987). A comparison of the two one-sided tests procedure and the power approach for assessing the equivalence of average bioavailability. *Journal of Pharmacokinetics and Biopharmaceutics* 15(6), 657–680.
- J. A. Hanley and A. Lippman-Hand (1983). If nothing goes wrong, is everything all right? Interpreting zero numerators. *JAMA* 249(13), 1743–1745.
- M. J. Harry (1988). *The Nature of Six Sigma Quality*. Motorola University Press. The source of the 3.4 DPMO figure and the 1.5σ shift convention.

## Frequently asked questions

**What is GoPatina?**
GoPatina is an open-source Go toolchain front end by Pigfox. Without a flag it is plain Go. With an opt-in flag, a defined subset of Go compiles through Rust, with no garbage collector and Rust's memory safety.

**Does GoPatina work today?**
Partly. Default mode works for all Go: it passes every command through to the installed go tool. The Rust-based modes work for the phase 1 subset of Go: functions, structs and methods, slices, strings, errors and fmt printing. Goroutines, maps, closures, interfaces, generics and most of the standard library are not supported yet and are rejected with a clear error.

**Will my existing Go code still work?**
Yes. Without a patina flag, gopatina runs go with the same arguments, so every Go program builds exactly as before. Only code you opt into patina mode must follow its stricter rules.

**Do I need to learn Rust to use GoPatina?**
No. You write Go. Without a garbage collector a few ownership rules remain, but GoPatina's borrow checker infers ownership and borrows, fixes what it safely can, and explains the rest in Go terms.

**How fast is GoPatina?**
The bar is equivalence with equivalent hand-written Rust within ±0.5%, shown by a two one-sided tests (TOST) equivalence test at 5σ; the test-server run that would show it is pending. In local measurements, fib and suffix-sum execute the same instructions as the hand-written Rust to within 0.0003% and compile to the same hot-loop machine code. The third benchmark, sieve, measured 1.30 times the instructions and about 7% slower, because of one extra bounds check per inner-loop pass; RFC 0006's loop specialization has since removed that check, so its hot loop now compiles to the same machine code too. The evidence section of this page reports the method, the statistics and every result.

**How is GoPatina's correctness measured?**
By differential testing against Go: a program built with gopatina build -patina must print the same standard output, the same first line of standard error and exit with the same status as the same program built with go build. The bar is Six Sigma: at most 3.4 defects per million tests, shown by 0 defects in at least 881,098 generated tests (a 95% one-sided upper bound). That run is pending on the test server; the 92 golden programs pass in CI on every change.

**What does a machine need to run a patina binary?**
Nothing extra. A patina binary is native machine code, so the target needs neither Go nor Rust installed, just like a Go binary. Phase 1 links against the platform's default C runtime; static binaries are planned.

**Who makes GoPatina, and under what license?**
GoPatina was started by Peter Sjolin as an experiment by Pigfox. It is open source under MIT OR Apache-2.0, and every change goes through a reviewed pull request on GitHub.

## Links

- Source code and README: https://github.com/pigfox/gopatina
- Examples: https://github.com/pigfox/gopatina/blob/main/EXAMPLES.md
- The phase 1 subset: https://github.com/pigfox/gopatina/blob/main/docs/patina-subset.md
- Feature matrix: https://github.com/pigfox/gopatina/blob/main/docs/compat.md
- Where Go and Rust semantics differ: https://github.com/pigfox/gopatina/blob/main/docs/go-rust-disconnects.md
- RFC 0001, the design: https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0001-gopatina.md
- RFC 0002, concurrency: https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0002-concurrency.md
- RFC 0003, toolchain compatibility: https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0003-toolchain-compatibility.md
- RFC 0004, read-only slice suffix arguments: https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0004-read-only-slice-suffix.md
- RFC 0005, phase 1 semantic exactness fixes: https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0005-phase1-semantic-exactness.md
- RFC 0006, proven integer ranges and loop specialization: https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0006-proven-ranges.md
- Benchmarks, raw results and run notes: https://github.com/pigfox/gopatina/blob/main/BENCHMARKS.md
- The two bars, with their formulas: https://github.com/pigfox/gopatina/blob/main/docs/QUALITY.md#the-two-bars
- Pigfox: https://pigfox.com

This site's server is a Go program built with gopatina's default mode; it uses `net/http` and `embed`, which are outside the phase 1 subset.
