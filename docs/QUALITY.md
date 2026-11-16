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
- **Target.** Within 10% of hand-written Rust on each benchmark, tracked over
  time. This is a goal that guides the work, **not a claim**. A pull request
  that makes a ratio worse beyond run-to-run noise must explain why in its
  description.

### Status

**No numbers exist yet.** Neither patina mode generates code today, so there
is nothing to measure. The harness and the suite are described in
[`bench/README.md`](../bench/README.md), and the
[`bench` workflow](../.github/workflows/bench.yml) (the per-pull-request
ratio run) is in place but skips its measurement steps until there is
generated code. The test-server runner does not exist yet either. No
performance figure will be published without the raw results and the
versions that produced it.
