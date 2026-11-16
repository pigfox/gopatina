# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Until 1.0.0, minor versions may contain breaking changes.

## [Unreleased]

### Added

- Dual license (MIT OR Apache-2.0).
- Governance, contributing guide, code of conduct, security policy, issue
  templates and pull request template.
- Continuous integration: gofmt, go vet and go mod tidy checks, and
  go test -race on Ubuntu and macOS with the two supported Go releases.
- Pull request checks for a Conventional Commit title and DCO sign-off.
- Dependabot updates for Go modules and GitHub Actions.
- RFC 0001: the GoPatina design (three build modes: default, -patina and
  -emit-rust), and the RFC template.
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
- RFC 0001: Rust underneath for both patina flags, the GoPatina borrow
  checker (ownership and lifetime inference, automatic fixes, -explain,
  gopatina fix, Go-style errors), deployment and portability (static
  binaries, cross-compilation, -patina=check, toolchain provisioning).
- RFC 0002: concurrency in patina mode (goroutines on Tokio's multi-threaded
  runtime, automatic async colouring, data races as compile errors or
  automatic fixes).
- RFC 0003: toolchain compatibility and versioning; docs/compat.md, the Go
  feature-support matrix for patina mode; a supported-toolchains table in
  the README.

- Toolchains workflow: Go stable and oldstable with Rust stable and MSRV
  (required), Go tip and Rust beta and nightly canaries, weekly schedule;
  the Rust steps are placeholders until there is generated code.

### Changed

- CI uses actions/checkout v7 and actions/setup-go v7.
- RFC 0001: the -patina backend is decided (Rust source built by rustc);
  the roadmap is now four phases after Phase 0.

[Unreleased]: https://github.com/pigfox/gopatina/commits/main
