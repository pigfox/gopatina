#!/usr/bin/env bash
# edition-canary.sh: compile every committed GoPatina crate with a Rust
# toolchain's next edition, so a change it makes to the language is known
# before the edition is stable (RFC 0003).
#
# usage: scripts/edition-canary.sh [<toolchain>] [<edition>]
#
# <toolchain> defaults to nightly, <edition> to future (rustc's name for
# the edition after the newest stable one; 2027 once it has a number).
# Each crate without dependencies has its entry file (src/main.rs or
# src/lib.rs) compiled by rustc directly, with that edition and with future-incompatible,
# edition-future-compatibility and every warning denied. It prints one
# line per crate that fails and exits 1 if any does.
set -euo pipefail

toolchain="${1:-nightly}"
edition="${2:-future}"
cd "$(dirname "$0")/.."
out="$(mktemp -d)"
trap 'rm -rf "$out"' EXIT

rustc +"$toolchain" --version
ok=0
failed=0
for crate in testdata/golden/*/rust testdata/golden-lib/*/rust examples/*/rust examples/rust-interop/rules; do
  if [ -f "$crate/src/main.rs" ]; then
    entry="$crate/src/main.rs" type=bin
  elif [ -f "$crate/src/lib.rs" ]; then
    entry="$crate/src/lib.rs" type=lib
  else
    continue
  fi
  if [ "$(grep -c '^\[\[package\]\]' "$crate/Cargo.lock" 2>/dev/null || echo 0)" -gt 1 ]; then
    # A crate with dependencies (examples/counter, Tokio) needs cargo;
    # GoPatina's own code in it is the same runtime the others compile.
    echo "skip $crate: it has dependencies"
    continue
  fi
  name="$(printf '%s' "$crate" | tr '/' '_')"
  if rustc +"$toolchain" -Zunstable-options --edition "$edition" -O --crate-type "$type" \
    -D future-incompatible -D edition-future-compatibility -D warnings \
    --crate-name canary --out-dir "$out/$name" "$entry" >"$out/$name.log" 2>&1; then
    ok=$((ok + 1))
  else
    failed=$((failed + 1))
    echo "FAIL $crate: $(grep -m1 '^error' "$out/$name.log" || echo 'see the log')"
    sed -n '1,30p' "$out/$name.log"
  fi
done
echo "edition $edition with $toolchain: $ok crates compile, $failed fail"
[ "$failed" -eq 0 ]
