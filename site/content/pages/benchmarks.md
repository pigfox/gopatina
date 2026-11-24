# GoPatina benchmarks: Go compiled through Rust vs hand-written Rust

How fast is Go when it is compiled through Rust? GoPatina's goal is identical speed to equivalent hand-written Rust on every benchmark, built with the latest stable Rust, where identical means equivalent within ±0.5% at 5σ. Each benchmark is also built with plain `go build`, so every result shows Go too. **On 2026-11-24 the test server measured all five benchmarks three ways, after GoPatina's one owner per pointer.** On `fib`, `sieve`, `suffix-sum` and now `binary-trees`, GoPatina executes the same user instructions as hand-written Rust, and on `binary-trees` its wall time and peak memory can no longer be told apart from Rust's. On `cycles` it is faster than Rust. Plain Go is still the fastest on `binary-trees`. The statistics behind every verdict are in [the evidence](/evidence#statistics).

## What is compared {#compared}

Both sides are Rust. GoPatina translates the Go program into a Rust crate, and rustc builds it; the other side is Rust written by hand for the same task, built by the same rustc (1.100.0 in the latest run). So a difference between the two is a difference in the Rust code, not in the compiler. Where GoPatina's generated Rust does what a person would write, the two execute the same instructions, as on `fib`, `sieve`, `suffix-sum` and now `binary-trees`. Where it does something a person would not, it is slower: until RFC 0017, `binary-trees` executed 19% more instructions than the hand-written Rust, because GoPatina wrapped each owned pointer in its own runtime type. Now each is a plain `Option<Box<T>>`, as the hand-written Rust has, and the instructions are the same. Plain Go, built by `go build`, is the third column, for reference.

## The suite {#suite}

| Benchmark | What it measures against plain Go |
| --- | --- |
| `fib` | `fib(42)` by plain recursion: allocates nothing, so it compares code generation (Go's compiler against LLVM) on calls and integer arithmetic |
| `sieve` | one large slice, then a loop of element writes: bounds checks and loop code, not the collector |
| `suffix-sum` | one allocation, then reads through `s[i:]` views: bounds checks and slicing |
| `binary-trees` | millions of small trees allocated and freed through pointers: Go's collector against `Box`, GoPatina's and the hand-written Rust's; memory management decides |
| `cycles` | doubly linked lists built and dropped, their nodes in one slice linked by index, as Rust programmers write such a list: Go's collector against freeing the slice |

Each benchmark is written in Go, built with `gopatina build -patina` and with `go build`, and written as idiomatic hand-written Rust that is not tuned to flatter GoPatina. All three builds must print the same output before any timing. The cases are in [bench/cases](https://github.com/pigfox/gopatina/tree/main/bench/cases). Why `fib(42)` and not `fib(1000)`: plain recursion makes about 10^209 calls for `fib(1000)`, which no computer could finish, and its value has 209 digits, too many for an `int64`.

## Go vs GoPatina vs Rust: the 2026-11-24 results {#results}

The test server (an Intel Xeon Gold 6152 virtual server, Skylake-SP, 7 vCPUs), GoPatina commit `6e0d0ca`, with one owner per pointer, rustc 1.100.0 for both Rust binaries, Go 1.27.1, 100 rounds of the three binaries interleaved, with the correctness run paused. Each value is the geometric mean with its 5σ interval.

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

Instruction intervals not shown are narrower than the digits given. Binary sizes are each tool's default: GoPatina strips its binaries, and the hand-written Rust and Go builds are not stripped, so the sizes compare defaults, not code.

The ratios, each with its 5σ interval and the verdict of a two one-sided tests (TOST) equivalence test at ±0.5%: "equivalent" only if the whole interval lies inside the margin, "faster" or "slower" if it lies wholly outside, "inconclusive" otherwise.

| Benchmark | GoPatina / Rust, wall | GoPatina / Go, wall | Go / Rust, wall | GoPatina / Rust, instructions |
| --- | --- | --- | --- | --- |
| `fib` | 0.886 [0.842, 0.933] faster | 0.519 [0.490, 0.550] faster | 1.708 [1.616, 1.806] slower | 1.00000 equivalent |
| `sieve` | 1.028 [0.950, 1.113] inconclusive | 1.049 [0.956, 1.151] inconclusive | 0.980 [0.899, 1.068] inconclusive | 1.00001 equivalent |
| `suffix-sum` | 1.002 [0.943, 1.065] inconclusive | 0.972 [0.906, 1.044] inconclusive | 1.031 [0.972, 1.093] inconclusive | 1.00001 equivalent |
| `binary-trees` | 1.022 [0.966, 1.082] inconclusive | 1.133 [1.070, 1.199] slower | 0.902 [0.859, 0.948] faster | 0.99998 equivalent |
| `cycles` | 0.860 [0.800, 0.924] faster | 0.279 [0.257, 0.302] faster | 3.081 [2.858, 3.321] slower | 0.943 faster |

What one owner per pointer changed on `binary-trees`, against the [previous run](https://github.com/pigfox/gopatina/blob/main/bench/results/2026-11-23-server-477aaa3-three-arm.md) on the same server and toolchains:

| `binary-trees`, GoPatina / Rust | Before: `477aaa3`, owned pointers in `rt::Own` | After: `6e0d0ca`, plain `Option<Box<T>>` |
| --- | --- | --- |
| User instructions | 1.189 slower | 0.99998 equivalent |
| Wall time | 1.193 [1.136, 1.253] slower | 1.022 [0.966, 1.082] inconclusive |
| Peak memory | 1.001 [0.995, 1.006] inconclusive | 0.999 [0.993, 1.006] inconclusive |

What each result shows:

- **`binary-trees`**: GoPatina now writes each tree node with one owner as a plain `Option<Box<T>>`, as the hand-written Rust does, and executes the same instructions. Its wall time, cycles and peak memory (33.9 MiB each) cannot be told apart from Rust's at this precision. Plain Go is still the fastest on wall time, its collector working concurrently with the program.
- **`cycles`**: rewritten so that a list's nodes live in one slice and link by index, on the Go and the hand-written Rust side both, so it does not compare with earlier runs. GoPatina executes 6% fewer instructions than the hand-written Rust, for a cause not yet established, and is faster on wall time; Go takes 3.1 times Rust's time.
- **`fib`**: GoPatina executes the same instructions as the hand-written Rust, and Go 1.48 times as many, so GoPatina takes about half Go's time. Its 10% fewer cycles than Rust with the same instructions has appeared in every run; the cause is not established, and it is not taken as GoPatina being faster than Rust.
- **`suffix-sum`**: GoPatina and Rust execute the same instructions; on wall time no pair can be told apart at this precision.
- **`sieve`**: the same machine code as the hand-written Rust, instruction for instruction, yet a few percent more cycles in some runs (1.038 here, inconclusive). The cause is where the loop sits in the binary. In GoPatina's binary, as examined, the loop's closing jump (a `jb` at 0x156fe) ends exactly on the 32-byte boundary 0x15700. On this Skylake-SP CPU, Intel's microcode fix for the jump conditional code (JCC) erratum keeps such a jump out of the decoded micro-op cache, so the loop is decoded again on every pass. The hand-written Rust's loop sits elsewhere. An unrelated change to GoPatina, its runtime or the toolchain can move the loop and flip the effect either way, and another CPU can read differently. It is not a difference in the generated code, so it is reported, not tuned for.

How GoPatina gives each pointer one owner is [RFC 0017](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0017-one-owner.md), which the project lead accepted on 2026-11-23. Every table, every pair and the raw JSON are in the [run notes](https://github.com/pigfox/gopatina/blob/main/bench/results/2026-11-24-server-6e0d0ca-three-arm.md).

## Earlier results {#earlier}

| Measured | Result (GoPatina / hand-written Rust) | When and where |
| --- | --- | --- |
| Three-arm run after owned pointers (`rt::Own`) | `binary-trees` 1.193 times Rust's wall time, 1.189 times its instructions, 1.001 times its peak memory; `cycles` (shared lists, before #135) 2.033 times its wall time | 2026-11-23, test server, GoPatina `477aaa3`, [run notes](https://github.com/pigfox/gopatina/blob/main/bench/results/2026-11-23-server-477aaa3-three-arm.md) |
| First three-arm run, before owned pointers | `binary-trees` 1.496 times Rust's wall time, 1.473 times its peak memory; `cycles` 1.999 times its wall time | 2026-11-23, test server, GoPatina `d207c32`, [run notes](https://github.com/pigfox/gopatina/blob/main/bench/results/2026-11-23-server-d207c32-three-arm.md) |
| Test server, user instructions, one rustc (1.100.0) for both | `fib` 1.0000082, `sieve` 1.0000062, `suffix-sum` 1.0000071: all equivalent at 6σ | 2026-11-22, test server, CPU counters, 100 runs per binary |
| Test-server pilot, wall time | `fib` 0.984, `sieve` 0.975, `suffix-sum` 1.007: all inconclusive, with 5σ intervals 4% to 21% wide | 2026-11-22, test server, 100 runs per binary |
| Machine code after RFC 0006 | the hot loops of `fib`, `sieve` and `suffix-sum` are the same instructions as the hand-written Rust's | 2026-11-21, rustc 1.93.0 |
| `sieve` before RFC 0006 | 1.302 times the instructions, about 7% slower: one extra bounds check, since removed | 2026-11-21, laptop, CPU counters, 300 runs per binary |

## Where the numbers come from {#method}

`bench/run.sh --equiv` runs GoPatina's binary, the hand-written Rust and plain Go in interleaved ABCCBA order and tests each pair's equivalence with two one-sided tests; `bench/asm-check.sh` compares the machine code. Every result is published with its commit, toolchains, machine, date and raw JSON in [BENCHMARKS.md](https://github.com/pigfox/gopatina/blob/main/BENCHMARKS.md). See also [correctness](/correctness): speed only counts if the program does what the Go program does.
