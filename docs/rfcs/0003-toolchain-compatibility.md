# RFC 0003: Toolchain compatibility and versioning

- **Status:** Draft. The CI described below runs: generated crates are built
  and run with Rust stable and the MSRV, and checked with the pinned
  toolchain.
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

**Status:** the Go and Rust jobs run on every pull request: the
`toolchains` workflow builds and runs every golden program's crate with Rust
stable and the MSRV and compares it with the Go build, checks the committed
crates with the pinned toolchain (no warnings, rustfmt-clean,
`gopatina verify`), and runs Go tip and Rust beta and nightly as canaries.

**The pin.** `toolchain.json` is the one place the toolchain versions are
written: the Rust release emitted crates pin, the MSRV, the edition and
go.mod's `go` version. Everything else that names them is a copy, and
`TestToolchainDrift` (internal/emit) fails when one disagrees: the
`Toolchain` and `MSRV` constants, every committed crate's
`rust-toolchain.toml` and `Cargo.toml`, the workflows (which read the file
and name no version of their own), and the README and patina-subset lines
that state the pin. `scripts/toolchain-bump.sh <toolchain>` rewrites every
copy at once. Weekly, the `toolchain bump` workflow runs it for the newest
Rust stable and beta on a branch, runs every gate there (the golden crates
and Rust tests, float bit for bit on arm64 and on amd64 with and without
FMA, the toolchain matrix and clippy, the difftest on amd64 and arm64, and
the benchmarks), and, for stable, opens a pull request when they pass. A
failed gate opens an issue.

**The next edition.** Emitted crates use the 2024 edition. The next one,
Rust 2027, has no number yet: on 2026-11-27, neither stable 1.100.1 nor
nightly 1.102.0 accepts `--edition 2027`, and nightly calls it `future`.
A canary in the `toolchains` workflow, `scripts/edition-canary.sh`,
compiles every committed crate without dependencies (189 of 190) with
nightly's next edition, denying `future-incompatible`,
`edition-future-compatibility` and every warning. On a scheduled failure
it opens an issue. When 2027 is stable, the crates move to it with the
toolchain bump, as for a new pinned release.

**Go's pin.** CI's Go is pinned too (toolchain.json `go.pinned`, and
`.go-version`, which actions/setup-go reads), except in the Go stable and
oldstable matrix and the canaries. It matters because the sites where gc
fuses a multiply-add depend on the release: the `fma-sites.txt` records
are gc's, from the pinned release, so TestFMASites compares them exactly.
`scripts/go-bump.sh <release>` rewrites the pin and the records. Daily,
the bump workflow's `go-stable` channel tries the newest Go release through
every gate and opens a pull request when they pass. Its `go-rc` channel
tries each new release candidate (go1.28rc1 first) and files an issue with
the result either way, so a change in gc is known before the release.

## Drawbacks

- Two release cadences to follow; the 4-week Go target is a commitment of
  maintainer time.
- Pinning runtime crates means GoPatina, not the user, decides when they
  move.

## Unresolved questions

- The initial MSRV is 1.85, the first release with the 2024 edition: it is
  in every emitted `Cargo.toml` and tested in CI. Whether it stays there for
  1.0 is open.
- Emitted crates pin an exact toolchain (`toolchain.json`) in
  `rust-toolchain.toml`, as reproducible builds need. The weekly bump
  proposes each new stable once every gate passes on it; whether such a
  pull request is merged at once or held for a benchmark review is open.
- How long a Go version stays supported in patina mode after it drops out of
  Go's own support window.
