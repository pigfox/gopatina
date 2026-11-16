# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Until 1.0.0, minor versions may contain breaking changes.

## [Unreleased]

### Changed

- Every pull request, including the maintainer's own and automated ones, is
  merged only after the project lead's approval; CI for outside contributors
  runs after maintainer approval (GOVERNANCE.md, CONTRIBUTING.md).
- Benchmarks: pull requests get ratios only (GoPatina, Rust and Go in the
  same job, medians) on GitHub-hosted runners; official numbers come only
  from the maintainer's test server, on merged code, when the server is
  quiet (docs/QUALITY.md, bench/README.md).

### Added

- BENCHMARKS.md, the published home of official benchmark results, and a
  Benchmarks section in the README. Every official run is the full suite,
  published as bench/results/<date>-<version>.json with a summary through an
  approved pull request, then shown in BENCHMARKS.md, the README and the
  release notes. Nothing has been measured yet.
- EXAMPLES.md and runnable examples (examples/hello, examples/ownership,
  examples/counter) walking through default mode (real output), -emit-rust
  and -patina=check (planned output, labelled as design).
- Project website for gopatina.org (site/), published to GitHub Pages by the
  pages workflow on pushes to main.

## [0.0.1] - 2026-11-16

Phase 0: RFC and pass-through CLI. A pre-release; GoPatina is at the design
stage and is not a working compiler.

### Added

- Dual license (MIT OR Apache-2.0).
- Governance, contributing guide, code of conduct, security policy, issue
  templates and pull request template.
- Continuous integration: gofmt, go vet and go mod tidy checks, and
  go test -race on Ubuntu and macOS with the two supported Go releases.
- Pull request checks for a Conventional Commit title and DCO sign-off.
- Dependabot updates for Go modules and GitHub Actions.
- RFC 0001, the GoPatina design: three build modes (default, -emit-rust,
  -patina) with Rust underneath for both patina flags; the GoPatina borrow
  checker (ownership and lifetime inference, automatic fixes, -explain,
  gopatina fix, Go-style errors); deployment and portability (static
  binaries, cross-compilation, -patina=check, toolchain provisioning).
- The RFC template.
- gopatina CLI prototype: passes every command through to the go command
  unchanged; -patina and -emit-rust report that they are not implemented yet
  and exit 2.
- Quality policy (docs/QUALITY.md): 100% unit-test coverage, enforced in CI,
  and the test layers each phase requires.
- Performance policy: benchmarks against hand-written Rust on the latest
  stable toolchain, the planned suite and harness (bench/), and a bench
  workflow that skips measurement until there is generated code. No results
  exist yet.
- Full README: mission, status, the three build modes, roadmap, quality and
  performance policy, quick start, prior art.
- RFC 0002: concurrency in patina mode (goroutines on Tokio's multi-threaded
  runtime, automatic async colouring, data races as compile errors or
  automatic fixes).
- RFC 0003: toolchain compatibility and versioning; docs/compat.md, the Go
  feature-support matrix for patina mode; a supported-toolchains table in
  the README.
- Toolchains workflow: Go stable and oldstable with Rust stable and MSRV
  (required), Go tip and Rust beta and nightly canaries, weekly schedule;
  the Rust steps are placeholders until there is generated code.

[Unreleased]: https://github.com/pigfox/gopatina/compare/v0.0.1...HEAD
[0.0.1]: https://github.com/pigfox/gopatina/releases/tag/v0.0.1
