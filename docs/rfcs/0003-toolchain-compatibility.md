# RFC 0003: Toolchain compatibility and versioning

- **Status:** Draft (design stage; the CI parts marked as placeholders below
  wait for generated code)
- **Author(s):** Peter Sjolin
- **Created:** 2026-11-16
- **Discussion:** this file's pull request, and issues labelled `rfc`
- **Depends on:** [RFC 0001](0001-gopatina.md), [RFC 0002](0002-concurrency.md)

## Summary

GoPatina sits between two toolchains that both release often: Go twice a
year (February and August), Rust stable every six weeks. This RFC states
what GoPatina supports, how it stays compatible as both move, how it is
versioned, and how breakage is detected before users see it.

## Motivation

A tool that depends on two compilers can break when either changes. Users
need to know which Go and Rust versions a GoPatina release works with, that
a new Go feature never silently miscompiles, and that a crate emitted today
still builds next year.

## The Go side

- **Default mode always works with new Go.** It delegates to the installed
  `go` command, so a new Go release works there the day it ships. GoPatina
  adds nothing that could break.
- **Patina modes parse Go with Go's own libraries**, `go/parser` and
  `go/types` (and Go's SSA form for analysis), and respect the `go`
  directive in `go.mod`, so the language version is the module's, as with
  `go build`.
- **Support policy mirrors Go's:** the two latest Go releases (`stable` and
  `oldstable`) are supported and tested.
- **Unsupported features fail loudly.** A Go language feature that patina
  mode does not support yet is rejected with a message naming the feature
  and the Go version that introduced it, for example:

  ```text
  ./iter.go:8:2: range-over-func (Go 1.23) is not supported in patina mode yet
      see docs/compat.md
  ```

  Never a silent miscompile.
- **A feature-support matrix**, [`docs/compat.md`](../compat.md), lists each
  Go language feature and its patina status.

## The Rust side

- **Stable Rust only.** Generated code uses no nightly features and no rustc
  internals. This is one more reason RFC 0001 set aside lowering to rustc's
  MIR, whose interfaces are unstable.
- **A declared edition and MSRV.** Emitted crates target the Rust 2024
  edition and a declared minimum supported Rust version (MSRV), recorded in
  `Cargo.toml` (`edition`, `rust-version`). The MSRV is raised only in a
  minor GoPatina release, and the release notes say so.
- **Reproducible crates.** `-emit-rust` writes `rust-toolchain.toml` and
  `Cargo.lock` with the crate, so an emitted crate keeps building the same
  way later, whatever toolchain the machine has by default.
- **Rust's stability promise makes this safe.** Code that compiles on a
  stable Rust release keeps compiling on later ones, and editions are opt-in:
  a new edition never changes the meaning of a crate that declares an older
  one.
- **Runtime crates are pinned** (Tokio, per RFC 0002, and any support
  crates). They are updated through Dependabot pull requests that must pass
  the full test suite and the benchmarks before merging.

## Versioning

- GoPatina follows [Semantic Versioning](https://semver.org/). Before 1.0,
  minor releases may contain breaking changes, and say so.
- **Every release states its supported ranges** in its release notes and in
  a table in the README:

  | GoPatina | Go | Rust stable | Rust MSRV | Rust edition |
  | --- | --- | --- | --- | --- |
  | (example) 0.2.x | 1.27, 1.26 | latest at release | 1.xx | 2024 |

- **New Go releases:** a GoPatina release supporting a new Go version ships
  within 4 weeks of Go's February and August releases. Until then, patina
  modes refuse language features newer than they support, as above; default
  mode is unaffected.
- **New Rust stable releases (every 6 weeks):** a scheduled CI run tests
  against the new stable, and the benchmarks are rerun, since the
  performance policy always measures against the latest stable Rust
  ([`docs/QUALITY.md`](../QUALITY.md)).

## Detection

- **Golden tests** of emitted Rust catch changes in GoPatina's own output.
- **Compile-and-run tests** build every golden crate and run it, so a change
  in either toolchain that breaks generated code fails CI.
- **Release channels:** CI runs the suite against Go `stable` and
  `oldstable` and Rust `stable` and the MSRV (required), and against Go's
  next release candidate and Rust beta and nightly as allowed-to-fail
  canaries, on every pull request and weekly on a schedule.
- **Early warning:** a canary failure on a scheduled run opens an issue
  automatically, so a break is known before that toolchain release ships.

**Status:** the Go jobs run today. Until there is generated code (Phase 1),
the Rust jobs only install the toolchain and record its version; the workflow
and this RFC say so.

## Drawbacks

- Two release cadences to follow; the 4-week Go target is a commitment of
  maintainer time.
- Pinning runtime crates means GoPatina, not the user, decides when they
  move.

## Unresolved questions

- The initial MSRV. The toolchains workflow uses 1.85, the first release
  with the 2024 edition, as a placeholder.
- Whether emitted crates pin an exact toolchain in `rust-toolchain.toml` or
  only a channel with a minimum.
- How long a Go version stays supported in patina mode after it drops out of
  Go's own support window.
