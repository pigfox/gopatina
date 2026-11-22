# GoPatina frequently asked questions

Short answers about GoPatina, a Go toolchain front end that compiles Go through Rust with no garbage collector. Longer ones are on the pages linked below.

## Questions and answers {#questions}

### What is GoPatina?

GoPatina is an open-source Go toolchain front end by Pigfox. Without a flag it is plain Go. With an opt-in flag, a defined subset of Go compiles through Rust, with no garbage collector and Rust's memory safety.

### Does GoPatina work today?

Partly. Default mode works for all Go: it passes every command through to the installed go tool. The Rust-based modes work for the phase 1 subset of Go: functions, structs and methods, slices, strings, errors and fmt printing. Goroutines, maps, closures, interfaces, generics and most of the standard library are not supported yet and are rejected with a clear error.

### Can I compile Go to Rust with GoPatina?

Yes, for the phase 1 subset: gopatina build -emit-rust writes a readable Rust crate you can keep and build with cargo, and gopatina build -patina builds it into a native binary with rustc. Programs outside the subset are refused with an error that names the unsupported construct.

### Will my existing Go code still work?

Yes. Without a patina flag, gopatina runs go with the same arguments, so every Go program builds exactly as before. Only code you opt into patina mode must follow its stricter rules.

### Do I need to learn Rust to use GoPatina?

No. You write Go. Without a garbage collector a few ownership rules remain, but GoPatina's borrow checker infers ownership and borrows, fixes what it safely can, and explains the rest in Go terms.

### How fast is GoPatina?

The bar is equivalence with equivalent hand-written Rust within ±0.5%, shown by a two one-sided tests (TOST) equivalence test at 5σ; the test-server run that would show it is pending. In local measurements, fib and suffix-sum execute the same instructions as the hand-written Rust to within 0.0003%. The third benchmark, sieve, measured 1.30 times the instructions and about 7% slower, because of one extra bounds check per inner-loop pass; RFC 0006's loop specialization has since removed that check, so all three hot loops now compile to the same machine code as the hand-written Rust.

### How is GoPatina's correctness measured?

By differential testing against Go: a program built with gopatina build -patina must print the same standard output, the same first line of standard error and exit with the same status as the same program built with go build. The bar is Six Sigma: at most 3.4 defects per million tests, shown at 6σ confidence by 0 defects in at least 6,099,050 generated tests; the first milestone, 881,098 tests, shows the same rate at 95% confidence. Both runs are pending on the test server; the golden programs pass in CI on every change.

### What does a machine need to run a patina binary?

Nothing extra. A patina binary is native machine code, so the target needs neither Go nor Rust installed, just like a Go binary. Phase 1 links against the platform's default C runtime; static binaries are planned.

### Who makes GoPatina, and under what license?

GoPatina was started by Peter Sjolin as an experiment by Pigfox. It is open source under MIT OR Apache-2.0, and every change goes through a reviewed pull request on GitHub.

## More {#more}

- [How it works](/how-it-works), [get started](/get-started) and the [roadmap](/roadmap).
- [The evidence](/evidence), the [benchmarks](/benchmarks) and [correctness](/correctness).
- The [source code and README on GitHub](https://github.com/pigfox/gopatina).
