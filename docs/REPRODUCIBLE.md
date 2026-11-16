# Reproducible, checksum-verified builds

GoPatina's promise of "Rust underneath" is meant to be checked, not trusted:

1. **Deterministic emission.** The same Go input, GoPatina version and
   options always produce byte-identical Rust source, listed with sha256 in
   `gopatina-manifest.sha256`.
2. **No hidden compiler.** The binary from `gopatina build -patina` has the
   same sha256 as the one you get by running plain
   `cargo build --release --locked` yourself on the crate `-emit-rust`
   writes, with the same pinned toolchain and target.

Both are **planned**: GoPatina's emitter and the `gopatina verify` command do
not exist yet. The design, including every setting and what can break
identity, is in
[RFC 0001, "Verified: Rust underneath, byte for byte"](rfcs/0001-gopatina.md#verified-rust-underneath-byte-for-byte).

## What is verified today

The reproducible-build **setup** that GoPatina will put in every emitted
crate is checked today on the two hand-written example crates, which show
the Rust GoPatina is designed to generate:

- [`examples/hello/rust`](../examples/hello/rust)
- [`examples/counter/rust`](../examples/counter/rust) (with Tokio, pinned)

> This demonstrates the reproducible-build settings on hand-written Rust. It
> does **not** test GoPatina's emitter, which does not exist yet.

Each crate carries the settings:

- `rust-toolchain.toml` pinning Rust 1.93.0 and the
  `x86_64-unknown-linux-gnu` target;
- a committed `Cargo.lock`, built with `--locked`;
- a release profile with `codegen-units = 1`, `debug = false`,
  `strip = true` and `incremental = false`.

[`scripts/repro-check.sh`](../scripts/repro-check.sh) builds each crate
**twice**: in two different directories with paths of different lengths,
and with two different `CARGO_HOME`s (so even the dependency sources live
at different host paths). Each build uses a fixed `SOURCE_DATE_EPOCH`,
`--remap-path-prefix` for its directory and its `CARGO_HOME`, `--locked`
and an explicit `--target`. The script fails if the two release binaries'
sha256 differ. The [`reproducible`](../.github/workflows/reproducible.yml)
CI workflow runs it on every pull request and every push to `main`.

```sh
scripts/repro-check.sh examples/hello/rust examples/counter/rust
```

### Results

| Crate | sha256 of the release binary |
| --- | --- |
| `examples/hello/rust` | `88b84bc668a94fd6da64013263f1b1c7afd2822cb087b503e7d749beff763710` |
| `examples/counter/rust` | `d26e71aefe07da850ffa5a74821095f50053bbd8957182585745d96e5b436ef3` |

Toolchain: `rustc 1.93.0 (254b59607 2026-01-19)`, target
`x86_64-unknown-linux-gnu`, `SOURCE_DATE_EPOCH=1700000000`. Date:
2026-11-16.

These hashes were produced, identically, in two places:

- the maintainer's workstation (Ubuntu 26.04.1 LTS, x86_64, system `cc`
  GCC 15.2.0), both builds of each crate;
- GitHub Actions (`ubuntu-26.04` runner image), both builds of each crate,
  in the first run of the `reproducible` workflow.

So for these crates the binaries were byte-identical across build
directories, across `CARGO_HOME`s and across two different Linux machines.
This target uses the system linker, so identity on other distributions or
linker versions is not claimed (see RFC 0001 for how the planned default
musl target avoids the host's C toolchain).

### A control: what the settings prevent

Building `examples/counter/rust` twice **without** `--remap-path-prefix`,
with different `CARGO_HOME`s, gave two different binaries (sha256 prefixes
`44b25dd6cf05ab6b` and `445183f7ad3fbf48`). The second binary contained 29
strings with its `CARGO_HOME` path, from Tokio's panic locations. With the
remapping, both builds are identical.

## How to check a hash yourself

```sh
git clone https://github.com/pigfox/gopatina && cd gopatina
scripts/repro-check.sh examples/hello/rust examples/counter/rust
```

With rustup installed, the pinned toolchain is installed automatically from
`rust-toolchain.toml`. Compare the printed hashes with the table above.
