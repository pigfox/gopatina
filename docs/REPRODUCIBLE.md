# Reproducible, checksum-verified builds

GoPatina's promise of "Rust underneath" is meant to be checked, not trusted:

1. **Deterministic emission.** The same Go input, GoPatina version and
   options always produce byte-identical Rust source, listed with sha256 in
   `gopatina-manifest.sha256`.
2. **No hidden compiler.** The binary from `gopatina build -patina` has the
   same sha256 as the one you get by running plain
   `cargo build --release --locked` yourself on the crate `-emit-rust`
   writes, with the same pinned toolchain and target.

Both are implemented for phase 1. The design, including every setting and
what can break identity, is in
[RFC 0001, "Verified: Rust underneath, byte for byte"](rfcs/0001-gopatina.md#verified-rust-underneath-byte-for-byte).

## gopatina verify

```console
$ gopatina verify ./examples/hello
emission:  deterministic (manifest sha256 87299988e121159fca6e8d88a46cf233c22caa0520ec42350e18db66d987fcc1, 6 files)
toolchain: Rust 1.93.0 (rust-toolchain.toml), target x86_64-unknown-linux-gnu
-patina:   96af9d4017b79bf6f35da51a892f4f631ad887aafb7e771364bd704dea495e85
cargo:     96af9d4017b79bf6f35da51a892f4f631ad887aafb7e771364bd704dea495e85  (cargo build --release --locked)
verified:  the -patina binary is byte-identical to the plain cargo build
```

It emits the package twice, into different directories, and requires the
two manifests to be identical; then it builds the crate through the
`-patina` path and with plain `cargo build --release --locked` in another
directory, and requires the two binaries to have the same sha256. CI runs it
on every program under `testdata/golden` (the `toolchains` workflow) and on
`examples/hello` (the `reproducible` workflow). The output above is from the
maintainer's workstation (Ubuntu 26.04.1, x86_64), on 2026-11-16.

To check it yourself:

```sh
gopatina build -emit-rust -o crate ./examples/hello
(cd crate && sha256sum -c gopatina-manifest.sha256 && cargo build --release --locked)
sha256sum crate/target/x86_64-unknown-linux-gnu/release/hello
```

## The example crates

`examples/hello/rust` is the crate gopatina emits for `examples/hello`
(a test keeps it up to date); `examples/counter/rust` is still the
hand-written design for goroutines, which are planned (RFC 0002).

Each crate carries the settings:

- `rust-toolchain.toml` pinning Rust 1.93.0 and the
  `x86_64-unknown-linux-gnu` target (the emitted crate also names it in
  `.cargo/config.toml`);
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
| `examples/hello/rust` (now emitted by gopatina) | `96af9d4017b79bf6f35da51a892f4f631ad887aafb7e771364bd704dea495e85` |
| `examples/counter/rust` | `d26e71aefe07da850ffa5a74821095f50053bbd8957182585745d96e5b436ef3` |

Toolchain: `rustc 1.93.0 (254b59607 2026-01-19)`, target
`x86_64-unknown-linux-gnu`, `SOURCE_DATE_EPOCH=1700000000`. Date:
2026-11-16.

The counter hash was produced, identically, in two places: the
maintainer's workstation (Ubuntu 26.04.1 LTS, x86_64, system `cc` GCC 15.2.0)
and GitHub Actions (`ubuntu-26.04` runner image), both builds each. The hello
hash is from the maintainer's workstation, where `scripts/repro-check.sh` and
`gopatina verify` gave the same value; the `reproducible` workflow prints
both on every run.
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
