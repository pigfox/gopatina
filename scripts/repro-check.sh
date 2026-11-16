#!/usr/bin/env bash
# repro-check.sh: build each Rust crate twice, in two different directories,
# with GoPatina's reproducible-build settings, and check that the two release
# binaries have the same sha256. See docs/REPRODUCIBLE.md.
#
# usage: scripts/repro-check.sh <crate-dir>...
#
# Prints one line per crate: "<sha256>  <crate-dir>" on success.
# Exits 1 if any crate's two builds differ.
set -euo pipefail

if [ "$#" -eq 0 ]; then
  echo "usage: $0 <crate-dir>..." >&2
  exit 2
fi

# A fixed timestamp for anything that would otherwise embed the build time.
export SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-1700000000}"
export CARGO_INCREMENTAL=0
TARGET="x86_64-unknown-linux-gnu"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# build <crate-src> <build-dir> <cargo-home>: copy the crate to <build-dir>
# and build it there with its own CARGO_HOME (so dependency sources live at a
# different host path), remapping every host path that could end up in the
# binary.
build() {
  local src="$1" dir="$2" home="$3"
  mkdir -p "$dir"
  cp -R "$src"/. "$dir"/
  rm -rf "$dir/target"
  (
    cd "$dir"
    CARGO_HOME="$home" \
      RUSTFLAGS="--remap-path-prefix=$dir=/build --remap-path-prefix=$home=/cargo" \
      cargo build --release --locked --quiet --target "$TARGET"
  )
}

status=0
for crate in "$@"; do
  name="$(basename "$(cd "$crate/.." && pwd)")" # examples/<name>/rust
  bin="$(sed -n 's/^name = "\(.*\)"/\1/p' "$crate/Cargo.toml" | head -n 1)"
  # Two different build directories and two different CARGO_HOMEs, with paths
  # of different lengths, so a leaked host path would change the output.
  a="$work/a/$name"
  b="$work/build-two-longer-path/$name"
  build "$crate" "$a" "${CARGO_HOME:-$HOME/.cargo}"
  build "$crate" "$b" "$work/cargo-home-two"
  sum_a="$(sha256sum "$a/target/$TARGET/release/$bin" | cut -d' ' -f1)"
  sum_b="$(sha256sum "$b/target/$TARGET/release/$bin" | cut -d' ' -f1)"
  if [ "$sum_a" = "$sum_b" ]; then
    echo "$sum_a  $crate"
  else
    echo "MISMATCH  $crate: $sum_a != $sum_b" >&2
    status=1
  fi
done

echo "toolchain: $(cd "$1" && rustc --version), target $TARGET, SOURCE_DATE_EPOCH=$SOURCE_DATE_EPOCH" >&2
exit "$status"
