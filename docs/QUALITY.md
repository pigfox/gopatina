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
| 1: `-emit-rust` | **Golden tests**: each Go input in `testdata/` has the exact Rust it must produce, reviewed in the pull request. **Compile tests**: every golden output builds with stable `rustc`. **Differential tests**: the Go program run with `go run` and the generated crate run with `cargo run` print the same output. |
| 2: `-patina` native | The phase 1 differential suite, run against `-patina` binaries as well. All three modes must agree. |
| 3: ownership inference | **Rejection tests**: programs that must not compile, each with the exact diagnostic. **Fuzz tests** (`go test -fuzz`) on the inference engine and the emitters, run in CI for a fixed time budget. |
| 4: concurrency | Race tests under `-race` for default mode, and tests that patina mode rejects programs with data races. |
| 5: stdlib and interop | Integration tests that build small real programs and call real Rust crates. |

## Performance: shadow Rust and keep up

### Policy

- **What is measured.** A benchmark suite of small, classic programs, each
  written twice: in Go (built with `gopatina build -patina`, and through
  `-emit-rust` plus `cargo build --release`) and as equivalent, idiomatic,
  hand-written Rust.
- **Against which Rust.** The latest stable Rust toolchain at the time of the
  run (`rustup toolchain install stable`). The exact `rustc --version` is
  recorded with every result, together with the Go version, the GoPatina
  commit, the machine and the operating system.
- **When.** For every release, and for every pull request that changes code
  generation (the emitters, the native backend, or the runtime).
- **What is reported.** For each benchmark, the ratio of GoPatina's time (and
  peak memory) to hand-written Rust's, with the spread over repeated runs.
- **Target.** Within 10% of hand-written Rust on each benchmark, tracked over
  time. This is a goal that guides the work, **not a claim**. A pull request
  that makes a ratio worse beyond run-to-run noise must explain why in its
  description.

### Status

**No numbers exist yet.** Neither patina mode generates code today, so there
is nothing to measure. The harness and the suite are described in
[`bench/README.md`](../bench/README.md), and the
[`bench` workflow](../.github/workflows/bench.yml) is in place but skips its
measurement steps until there is generated code. No performance figure will
be published without the raw results and the versions that produced it.
