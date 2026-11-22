# GoPatina correctness: differential tests against Go

A program compiled through Rust is only useful if it does exactly what the Go program does. GoPatina checks that by differential testing: the same program is built with Go's reference build (`go build -gcflags=all=-N -l`, optimization and inlining off) and with `gopatina build -patina`, and both must print the same standard output, the same first line of standard error, and exit with the same status.

## The Six Sigma bar {#bar}

The bar is at most 3.4 defects per million tests, the Six Sigma rate, shown at 6σ confidence: 0 defects in at least 6,099,050 generated tests. On the way there, 0 defects in 881,098 tests show the same rate at 95% confidence, a milestone. **Both runs are pending on the test server.** The hypotheses and formulas are in [the evidence](/evidence#statistics).

## How programs are checked {#checks}

- **Golden programs.** 106 hand-written Go programs, each with the exact Rust it must produce, are built both ways in CI on every change, and must match Go. 25 rejection fixtures and 3 Go-only semantics counterexamples pin down what must be refused.
- **Generated programs.** `cmd/gopatina-difftest` builds random programs from GoPatina's generator, inside the phase 1 subset: wrapping integer arithmetic, shifts, slices, `append`, `switch`, formatted printing, named types of every integer width with their methods, `float32` arithmetic and conversions, shifts by run-time counts, a second package of the same program, package-level variables read and changed in the same statements as calls that change them, float-to-integer conversions (including NaN, the infinities and values far out of range), fmt widths and flags on slices, integers and strings, `strings` functions on text chosen where Go's Unicode case rules differ from Rust's, and the loop nests that RFC 0006 specializes, some at exactly their thresholds.
- **Panic cases.** Each batch of generated programs ends with one that panics at run time (an index out of range, a division by zero, a `make` out of range, an explicit `panic`), and GoPatina must panic with Go's message and Go's exit status.
- **Reproducible builds.** Emitted Rust is byte-identical for the same input, and `gopatina verify` checks that a `-patina` binary has the same sha256 as a plain `cargo build --release --locked` of the emitted crate.
- **Unit tests.** Every Go package has 100% statement coverage, enforced in CI.

## Defects found so far {#defects}

| Found | Defect | Fixed |
| --- | --- | --- |
| 2026-11-21, by hand | `make` with a negative length known only at run time panicked with Rust's message instead of Go's | with five golden programs |
| 2026-11-22, by the generated tests on the test server | an expression such as `a*b << 27` whose left side ended in a conversion did not compile as Rust | with a golden program and a generator case |
| 2026-11-22, by new generator cases, before they were merged | a shift by a constant at least the operand's width (`x << 70`, or `b >> 8` for a byte) gave Rust's result instead of Go's 0 or −1 | with a golden program and generator cases |
| 2026-11-22, by hand, while planning RFC 0008 | a float-to-integer conversion gave Rust's saturated result where Go's compiler gives another: NaN and out-of-range values, and `uint8(-2.5)`, which is 254 in Go | with a golden program and generator cases |
| 2026-11-22, by those generator cases, before they were merged | a float whose shortest digits have two equally close candidates printed the upper one (`1.2848823997124483e+15`) where Go prints the even one (`...482e+15`) | with a golden program |
| 2026-11-22, by hand, while adding fmt flags | a width on a slice (`%6v` of `[]int`) padded the whole slice, where Go pads each element (`[     1    -20]`) | with a golden program and generator cases |
| 2026-11-22, by hand, while adding RFC 0008's types | a slice literal printed or measured without a type from its context (`fmt.Println([]int{1 << 40})`) did not compile: Rust took its constants as 32-bit | with a golden program |

## When Go itself is wrong {#go-disagreements}

On 2026-11-22 the generated tests reported two programs where GoPatina's output differed from `go build`'s. Both times GoPatina was right: the optimized gc build on amd64 (one program from Go 1.22.0, the other from Go 1.26.0, both up to 1.27.1, the newest tested) printed a result that contradicts the language specification and a hand calculation, while gc with optimization and inlining off agreed with GoPatina. A test whose oracle can be wrong measures the oracle too, so the oracle is now Go's reference build, `go build -gcflags=all=-N -l`. Every batch is still built optimized as well, and a program whose two Go builds disagree is recorded and kept for review as a Go compiler disagreement, never counted as a GoPatina defect and never silently passed. Both programs, reduced, are golden tests.

A defect found is a defect the bar's run would also have counted, so the long run starts again from the beginning with each fixed compiler. Development smoke runs (1,318 generated tests on a laptop, 0 defects) are far below the bar and are not evidence for it.

See [the benchmarks](/benchmarks) for speed, and the [feature matrix](https://github.com/pigfox/gopatina/blob/main/docs/compat.md) for what the subset covers.
