# GoPatina benchmarks: Go compiled through Rust vs hand-written Rust

How fast is Go when it is compiled through Rust? GoPatina's goal is identical speed to equivalent hand-written Rust on every benchmark, built with the latest stable Rust, where identical means equivalent within ±0.5% at 5σ. Each benchmark is also built with plain `go build`, so every result shows Go too. **On 2026-11-23 the test server measured all five benchmarks three ways.** On three, GoPatina executes the same user instructions as hand-written Rust. On the two pointer benchmarks it is slower than both Rust and Go, and plain Go is the fastest of the three. The statistics behind every verdict are in [the evidence](/evidence#statistics).

## The suite {#suite}

| Benchmark | What it measures against plain Go |
| --- | --- |
| `fib` | `fib(42)` by plain recursion: allocates nothing, so it compares code generation (Go's compiler against LLVM) on calls and integer arithmetic |
| `sieve` | one large slice, then a loop of element writes: bounds checks and loop code, not the collector |
| `suffix-sum` | one allocation, then reads through `s[i:]` views: bounds checks and slicing |
| `binary-trees` | millions of small trees allocated and freed through pointers: Go's collector against GoPatina's reference counts and the hand-written Rust's `Box`; memory management decides |
| `cycles` | doubly linked lists built and dropped: Go's collector reclaims their cycles, GoPatina's reference counts do not, so its memory is the point; the hand-written Rust uses `Weak` back pointers |

Each benchmark is written in Go, built with `gopatina build -patina` and with `go build`, and written as idiomatic hand-written Rust that is not tuned to flatter GoPatina. All three builds must print the same output before any timing. The cases are in [bench/cases](https://github.com/pigfox/gopatina/tree/main/bench/cases). Why `fib(42)` and not `fib(1000)`: plain recursion makes about 10^209 calls for `fib(1000)`, which no computer could finish, and its value has 209 digits, too many for an `int64`.

## Go vs GoPatina vs Rust: the 2026-11-23 results {#results}

The test server (an Intel Xeon Gold 6152 virtual server, 7 vCPUs), GoPatina commit `d207c32`, rustc 1.100.0 for both Rust binaries, Go 1.27.1, 100 rounds of the three binaries interleaved, with the correctness run paused. Each value is the geometric mean with its 5σ interval.

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

Instruction intervals not shown are narrower than the digits given. Binary sizes are each tool's default: GoPatina strips its binaries, and the hand-written Rust and Go builds are not stripped, so the sizes compare defaults, not code.

The ratios, each with its 5σ interval and the verdict of a two one-sided tests (TOST) equivalence test at ±0.5%: "equivalent" only if the whole interval lies inside the margin, "faster" or "slower" if it lies wholly outside, "inconclusive" otherwise.

| Benchmark | GoPatina / Rust, wall | GoPatina / Go, wall | Go / Rust, wall | GoPatina / Rust, instructions |
| --- | --- | --- | --- | --- |
| `fib` | 0.904 [0.868, 0.941] faster | 0.526 [0.498, 0.554] faster | 1.720 [1.648, 1.796] slower | 1.00000 equivalent |
| `sieve` | 0.984 [0.931, 1.039] inconclusive | 0.968 [0.909, 1.031] inconclusive | 1.016 [0.961, 1.075] inconclusive | 1.00001 equivalent |
| `suffix-sum` | 1.006 [0.960, 1.054] inconclusive | 0.971 [0.920, 1.025] inconclusive | 1.036 [0.991, 1.082] inconclusive | 1.00001 equivalent |
| `binary-trees` | 1.496 [1.422, 1.574] slower | 1.591 [1.523, 1.662] slower | 0.940 [0.903, 0.979] faster | 1.372 slower |
| `cycles` | 1.999 [1.890, 2.114] slower | 2.701 [2.532, 2.882] slower | 0.740 [0.698, 0.784] faster | 0.983 faster |

What each result shows:

- **`fib`**: GoPatina executes the same instructions as the hand-written Rust, and Go 1.48 times as many, so GoPatina takes about half Go's time. Its 10% fewer cycles than Rust with the same instructions appeared on 2026-11-22 too; the cause is not established, and it is not taken as GoPatina being faster than Rust.
- **`sieve` and `suffix-sum`**: GoPatina and Rust execute the same instructions; on wall time no pair can be told apart at this precision.
- **`binary-trees`**: plain Go is the fastest, its collector working concurrently with the program. GoPatina represents a shared pointer as a reference-counted cell (`Rc<RefCell<T>>`) where the hand-written Rust uses a `Box`, and takes 1.50 times Rust's time with 1.47 times its peak memory.
- **`cycles`**: plain Go is the fastest. GoPatina executes about as many instructions as Rust but never frees the lists, the known limitation of [RFC 0013](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0013-pointers.md): 185 MiB at peak against Rust's 3 MiB, and twice Rust's time spent on that fresh memory.

The causes of both pointer results are measured in [RFC 0016](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0016-owned-pointers-and-cycle-collection.md), which the project lead accepted on 2026-11-23: a `Box` for every pointer that is never shared, then collection of cycles. Every table, every pair and the raw JSON are in the [run notes](https://github.com/pigfox/gopatina/blob/main/bench/results/2026-11-23-server-d207c32-three-arm.md).

## Earlier results {#earlier}

| Measured | Result (GoPatina / hand-written Rust) | When and where |
| --- | --- | --- |
| Test server, user instructions, one rustc (1.100.0) for both | `fib` 1.0000082, `sieve` 1.0000062, `suffix-sum` 1.0000071: all equivalent at 6σ | 2026-11-22, test server, CPU counters, 100 runs per binary |
| Test-server pilot, wall time | `fib` 0.984, `sieve` 0.975, `suffix-sum` 1.007: all inconclusive, with 5σ intervals 4% to 21% wide | 2026-11-22, test server, 100 runs per binary |
| Machine code after RFC 0006 | the hot loops of `fib`, `sieve` and `suffix-sum` are the same instructions as the hand-written Rust's | 2026-11-21, rustc 1.93.0 |
| `sieve` before RFC 0006 | 1.302 times the instructions, about 7% slower: one extra bounds check, since removed | 2026-11-21, laptop, CPU counters, 300 runs per binary |

## Where the numbers come from {#method}

`bench/run.sh --equiv` runs GoPatina's binary, the hand-written Rust and plain Go in interleaved ABCCBA order and tests each pair's equivalence with two one-sided tests; `bench/asm-check.sh` compares the machine code. Every result is published with its commit, toolchains, machine, date and raw JSON in [BENCHMARKS.md](https://github.com/pigfox/gopatina/blob/main/BENCHMARKS.md). See also [correctness](/correctness): speed only counts if the program does what the Go program does.
