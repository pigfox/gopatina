# Quality and performance policy

GoPatina's mission is that Go developers get Rust's performance and safety
without the painful Rust learning curve. Two standing rules follow from it:
everything is tested, and everything patina mode produces is measured against
Rust.

## Testing

### Unit tests: 100% coverage, enforced

- Every Go package in this repository has **100.0% statement coverage**
  from its unit tests.
- CI enforces it: the `test` job runs
  `go test -race -covermode=atomic -coverprofile=coverage.out ./...` and fails
  if `go tool cover -func=coverage.out` reports a total below `100.0%`.
- A pull request that lowers coverage does not merge. Code that cannot be
  reached from a test is a design smell; restructure it (for example, inject
  the dependency) rather than excluding it.
- Check locally before pushing:

  ```sh
  go test -race -coverprofile=coverage.out ./...
  go tool cover -func=coverage.out | tail -n 1   # must say 100.0%
  ```

Coverage is a floor, not the goal: tests must assert behaviour, not just
execute lines.

### Other test layers, by phase

Each phase of the roadmap in RFC 0001 adds a layer, and that layer is a
requirement for the phase to be complete, not an extra.

Phase 1 has its layers, and the borrow-checker layers for the phase 1
subset: golden crates in `testdata/golden/*/rust`
(`go test ./internal/emit -update` rewrites them), `-explain` reports and
exact rejection diagnostics in `testdata/*/*/explain.txt` and `want.txt`
(`go test ./internal/borrow -update`), the emitter's own `-explain` lines for
loop specialization in `testdata/golden/*/emit-explain.txt`
(`go test ./internal/emit -update`), the differential, compile, clippy (`-D warnings`, with a documented allow
list for Go-faithful patterns), rustfmt
and `gopatina verify` tests in `internal/patina/rust_test.go` (run with
`GOPATINA_RUST_TESTS=1` by the `toolchains` workflow, with Rust stable and the
MSRV), and `FuzzCheck`, `FuzzBorrow` and `FuzzEmit`, run by CI for 60 seconds
each. The differential tests build the Go program with `go build` and compare
standard output, the first stderr line (Go's stack trace after a panic is
ignored) and exit status; the emitted binary prints Go's panic wording itself,
so no translation happens in the test. `gopatina fix` and its tests are
planned.

### Defect rate: generated differential tests

The golden programs are written by hand, so they test what their authors
thought of. `cmd/gopatina-difftest` measures a defect rate instead. It builds
random programs inside the phase 1 subset, made by `internal/progen`, with
`go build` and with `gopatina build -patina`, runs both, and counts every
program whose output differs, or that patina mode fails to build, as a
defect. The programs exercise wrapping integer arithmetic (including
`MinInt64 / -1`), shifts, bitwise operators, `min` and `max`, slices and
`append`, `switch`, helper calls, string building and formatted printing;
each is valid Go that terminates without a panic. They also contain the
loop nests RFC 0006 specializes (counted, sieve-like and cube nests over a
slice made with a variable length), now and then at exactly a
specialization threshold. Program k depends only on
the seed and k, so a failure is reproducible and a run can be split across
machines with `-start`.

Programs are built many to a batch, one Go package and one crate, and
compared program by program through marker lines. Each batch ends with one
more program, its panic case. In most batches it panics at run time: an
index out of range (of a slice or a string, positive or negative), an
integer division or remainder by zero, a `make` length or capacity out of
range, an explicit `panic`, or a cube nest one past its RFC 0006
threshold, where the general copy must reproduce Go's wrap-around and index
panic. It must match Go's output, the first line of
Go's standard error and Go's exit status. Programs and panic cases together
are the tests. With no defect among N tests, the 95% upper bound on the
defect rate is -ln(0.05) / N; 881,098 defect-free tests bound it at 3.4
defects per million, the Six Sigma level. A long run belongs on the test server, not a workstation:

```sh
go build -o gopatina ./cmd/gopatina && go build -o gopatina-difftest ./cmd/gopatina-difftest
./gopatina-difftest -seed 1 -programs 882000 -per 150 -jobs 6 \
  -gopatina ./gopatina -failures difftest-failures >difftest.json 2>difftest.log
```

The report (`difftest.json`) gives programs, panic cases, tests, defects,
defects per million tests and the bound; `difftest.log` lists each batch as it finishes, and failing
batches are kept with both outputs under `difftest-failures`.

| Phase | Required test layers (in addition to 100% unit coverage) |
| --- | --- |
| 0: pass-through CLI | CLI tests that run a stand-in `go` command and check arguments, streams and exit status are passed through unchanged |
| 1: `-emit-rust` and `-patina` | **Golden tests**: each Go input in `testdata/` has the exact Rust it must produce, reviewed in the pull request. **Compile tests**: every golden output builds with stable `rustc`. **Differential tests**: the Go program run with `go run`, and the `-patina` binary, print the same output. |
| 2: borrow checker | **Rejection tests**: programs that must not compile, each with the exact Go-style diagnostic. **Fix tests**: each automatic fix, its `-explain` report and its `gopatina fix` rewrite. **Agreement tests**: nothing GoPatina accepts is rejected by rustc. **Fuzz tests** (`go test -fuzz`) on the checker and the emitter, run in CI for a fixed time budget. |
| 3: concurrency (RFC 0002) | Race tests under `-race` for default mode, and tests that patina mode rejects or fixes programs with data races. |
| 4: stdlib and interop | Integration tests that build small real programs and call real Rust crates. |

## Performance: shadow Rust and keep up

### Policy

- **What is measured.** A benchmark suite of small, classic programs, each
  written twice: in Go (built with `gopatina build -patina`, which compiles the
  generated crate with rustc in release mode) and as equivalent, idiomatic,
  hand-written Rust.
- **Against which Rust.** The latest stable Rust toolchain at the time of the
  run (`rustup toolchain install stable`). The exact `rustc --version` is
  recorded with every result, together with the Go version, the GoPatina
  commit, the machine and the operating system.
- **What is reported.** For each benchmark, the ratio of GoPatina's time (and
  peak memory) to hand-written Rust's and to plain Go's, with the spread over
  repeated runs.

### Where benchmarks run

There are two kinds of run, and they are kept apart:

- **Per pull request, on GitHub-hosted runners: ratios only.** For every pull
  request that changes code generation (the borrow checker's fixes, the Rust
  emitter, or the runtime support code), CI builds the GoPatina binary, the
  hand-written Rust and the plain Go program **in the same job**, runs each
  many times, and reports the **median** of each and the ratios between them.
  Shared runners are too noisy for absolute numbers, so none are reported;
  running all three side by side on one machine makes the ratios
  meaningful.
- **Official numbers, on the maintainer's own test server, outside GitHub.**
  Release numbers, and any figure published as GoPatina's performance, come
  only from a dedicated test server run by the maintainer. It benchmarks only
  code that is already merged into `main` with the maintainer's approval, and
  is **never triggered by a pull request**. It runs only when the server is
  quiet: before starting, the runner checks the machine's load and that no
  other CI job is running, and waits until both are clear. Results are
  committed to the repository with the machine description, the Go and Rust
  versions, the GoPatina commit and the date.

### Publishing official results

- **Every official run is the full suite**: every benchmark, never a subset.
- **Each run is published** as a results file,
  `bench/results/<date>-<gopatina version>.json`, plus a human-readable
  summary table, proposed in a pull request that the maintainer approves like
  any other change.
- **Once merged**, the numbers appear in [`BENCHMARKS.md`](../BENCHMARKS.md),
  the published home of the results (the latest run in full, every earlier
  run below it), in the README's Benchmarks section (the latest headline
  ratios), and in the GitHub release notes for that version. Each shows the
  machine, the Go and Rust versions, the commit and the date, and links to the
  raw results file.
- **Target.** Identical speed to hand-written Rust on each benchmark, tracked over
  time. "Identical" means equivalent within ±0.5% at 5σ: in a two one-sided tests (TOST) equivalence test on paired, interleaved runs, the whole 5σ interval of the ratio lies inside ±0.5%. (`bench/run.sh --equiv`, [bench/README.md](../bench/README.md)). This is a goal that guides the work, **not a claim**. A pull request
  that makes a ratio worse beyond run-to-run noise must explain why in its
  description.

### Status

**No official numbers exist yet.** The harness,
[`bench/run.sh`](../bench/run.sh), and three cases (`fib`, `sieve` and the
read-only `s[i:]` case `suffix-sum`) exist, and the
[`bench` workflow](../.github/workflows/bench.yml) (the per-pull-request
ratio run) runs them on pull requests that change code generation, reporting
ratios only. A local verification run on the maintainer's workstation has
been published in [`BENCHMARKS.md`](../BENCHMARKS.md) with its raw results
and environment, but it is **not** an official run: the dedicated,
quiet test server does not exist yet. No figure is published without the
raw results and the versions that produced it.
