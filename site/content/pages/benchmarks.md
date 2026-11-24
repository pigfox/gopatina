# GoPatina benchmarks: Go compiled through Rust vs hand-written Rust

How fast is Go when it is compiled through Rust? GoPatina's goal is identical speed to equivalent hand-written Rust on every benchmark, built with the latest stable Rust, where identical means equivalent within ±0.5% at 5σ. Each benchmark is also built with plain `go build`, so every result shows Go too. **On 2026-11-23 the test server measured all five benchmarks three ways, after GoPatina's owned pointers.** On three, GoPatina executes the same user instructions as hand-written Rust. On `binary-trees` its peak memory now matches Rust's, and its wall time fell from 1.50 to 1.19 times Rust's. On the two pointer benchmarks it is still slower than both Rust and Go, and plain Go is the fastest of the three. The statistics behind every verdict are in [the evidence](/evidence#statistics).

## The suite {#suite}

| Benchmark | What it measures against plain Go |
| --- | --- |
| `fib` | `fib(42)` by plain recursion: allocates nothing, so it compares code generation (Go's compiler against LLVM) on calls and integer arithmetic |
| `sieve` | one large slice, then a loop of element writes: bounds checks and loop code, not the collector |
| `suffix-sum` | one allocation, then reads through `s[i:]` views: bounds checks and slicing |
| `binary-trees` | millions of small trees allocated and freed through pointers: Go's collector against `Box`, GoPatina's (for pointers with one owner) and the hand-written Rust's; memory management decides |
| `cycles` | doubly linked lists built and dropped: Go's collector reclaims their cycles, GoPatina's reference counts do not, so its memory is the point; the hand-written Rust uses `Weak` back pointers |

Each benchmark is written in Go, built with `gopatina build -patina` and with `go build`, and written as idiomatic hand-written Rust that is not tuned to flatter GoPatina. All three builds must print the same output before any timing. The cases are in [bench/cases](https://github.com/pigfox/gopatina/tree/main/bench/cases). Why `fib(42)` and not `fib(1000)`: plain recursion makes about 10^209 calls for `fib(1000)`, which no computer could finish, and its value has 209 digits, too many for an `int64`.

## Go vs GoPatina vs Rust: the 2026-11-23 results {#results}

The test server (an Intel Xeon Gold 6152 virtual server, 7 vCPUs), GoPatina commit `477aaa3`, with owned pointers, rustc 1.100.0 for both Rust binaries, Go 1.27.1, 100 rounds of the three binaries interleaved, with the correctness run paused. Each value is the geometric mean with its 5σ interval.

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

Instruction intervals not shown are narrower than the digits given. Binary sizes are each tool's default: GoPatina strips its binaries, and the hand-written Rust and Go builds are not stripped, so the sizes compare defaults, not code.

The ratios, each with its 5σ interval and the verdict of a two one-sided tests (TOST) equivalence test at ±0.5%: "equivalent" only if the whole interval lies inside the margin, "faster" or "slower" if it lies wholly outside, "inconclusive" otherwise.

| Benchmark | GoPatina / Rust, wall | GoPatina / Go, wall | Go / Rust, wall | GoPatina / Rust, instructions |
| --- | --- | --- | --- | --- |
| `fib` | 0.896 [0.851, 0.943] faster | 0.519 [0.491, 0.548] faster | 1.728 [1.643, 1.817] slower | 1.00000 equivalent |
| `sieve` | 1.007 [0.944, 1.076] inconclusive | 0.985 [0.910, 1.067] inconclusive | 1.023 [0.956, 1.093] inconclusive | 1.00001 equivalent |
| `suffix-sum` | 0.995 [0.950, 1.043] inconclusive | 0.945 [0.894, 0.998] inconclusive | 1.054 [1.004, 1.106] inconclusive | 1.00001 equivalent |
| `binary-trees` | 1.193 [1.136, 1.253] slower | 1.267 [1.208, 1.329] slower | 0.942 [0.905, 0.980] faster | 1.189 slower |
| `cycles` | 2.033 [1.926, 2.146] slower | 2.807 [2.648, 2.975] slower | 0.724 [0.683, 0.768] faster | 0.983 faster |

What owned pointers changed on `binary-trees`, against the [first three-arm run](https://github.com/pigfox/gopatina/blob/main/bench/results/2026-11-23-server-d207c32-three-arm.md) earlier the same day, on the same server and toolchains:

| `binary-trees`, GoPatina / Rust | Before: `d207c32`, shared cells | After: `477aaa3`, owned pointers |
| --- | --- | --- |
| Wall time | 1.496 [1.422, 1.574] slower | 1.193 [1.136, 1.253] slower |
| User instructions | 1.372 slower | 1.189 slower |
| Peak memory | 1.473 [1.467, 1.480] larger | 1.001 [0.995, 1.006] inconclusive |

What each result shows:

- **`fib`**: GoPatina executes the same instructions as the hand-written Rust, and Go 1.48 times as many, so GoPatina takes about half Go's time. Its 10% fewer cycles than Rust with the same instructions has appeared in every run; the cause is not established, and it is not taken as GoPatina being faster than Rust.
- **`sieve` and `suffix-sum`**: GoPatina and Rust execute the same instructions; on wall time no pair can be told apart at this precision.
- **`binary-trees`**: GoPatina now builds each tree node with one owner as a `Box`, as the hand-written Rust does, instead of a reference-counted cell (`Rc<RefCell<T>>`). Its peak memory is Rust's (34.0 MiB against 33.9 MiB), and it takes 1.19 times Rust's time and instructions, down from 1.50 and 1.37; the cause of the rest is not yet measured. Plain Go is still the fastest, its collector working concurrently with the program.
- **`cycles`**: unchanged, and plain Go is the fastest. Its lists are shared, so GoPatina keeps them as reference-counted cells and never frees them, the known limitation of [RFC 0013](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0013-pointers.md): 185 MiB at peak against Rust's 3 MiB, and twice Rust's time spent on that fresh memory.

The causes of both pointer results are measured in [RFC 0016](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0016-owned-pointers-and-cycle-collection.md), which the project lead accepted on 2026-11-23: a `Box` for every pointer that is never shared (done, measured above), then collection of cycles (next). Every table, every pair and the raw JSON are in the [run notes](https://github.com/pigfox/gopatina/blob/main/bench/results/2026-11-23-server-477aaa3-three-arm.md).

## Earlier results {#earlier}

| Measured | Result (GoPatina / hand-written Rust) | When and where |
| --- | --- | --- |
| First three-arm run, before owned pointers | `binary-trees` 1.496 times Rust's wall time, 1.473 times its peak memory; `cycles` 1.999 times its wall time | 2026-11-23, test server, GoPatina `d207c32`, [run notes](https://github.com/pigfox/gopatina/blob/main/bench/results/2026-11-23-server-d207c32-three-arm.md) |
| Test server, user instructions, one rustc (1.100.0) for both | `fib` 1.0000082, `sieve` 1.0000062, `suffix-sum` 1.0000071: all equivalent at 6σ | 2026-11-22, test server, CPU counters, 100 runs per binary |
| Test-server pilot, wall time | `fib` 0.984, `sieve` 0.975, `suffix-sum` 1.007: all inconclusive, with 5σ intervals 4% to 21% wide | 2026-11-22, test server, 100 runs per binary |
| Machine code after RFC 0006 | the hot loops of `fib`, `sieve` and `suffix-sum` are the same instructions as the hand-written Rust's | 2026-11-21, rustc 1.93.0 |
| `sieve` before RFC 0006 | 1.302 times the instructions, about 7% slower: one extra bounds check, since removed | 2026-11-21, laptop, CPU counters, 300 runs per binary |

## Where the numbers come from {#method}

`bench/run.sh --equiv` runs GoPatina's binary, the hand-written Rust and plain Go in interleaved ABCCBA order and tests each pair's equivalence with two one-sided tests; `bench/asm-check.sh` compares the machine code. Every result is published with its commit, toolchains, machine, date and raw JSON in [BENCHMARKS.md](https://github.com/pigfox/gopatina/blob/main/BENCHMARKS.md). See also [correctness](/correctness): speed only counts if the program does what the Go program does.
