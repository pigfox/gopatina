#!/usr/bin/env bash
# toolchain-bump.sh: move the Rust toolchain emitted crates pin to a new
# release, rewriting toolchain.json and every copy of it at once.
#
# usage: scripts/toolchain-bump.sh <toolchain>
#
# <toolchain> is a rustup channel name: a release (1.101.0) or a dated
# beta or nightly (beta-2026-12-01). It rewrites:
# - toolchain.json's rust.pinned;
# - internal/emit's Toolchain constant;
# - the golden crates and examples/hello/rust (go test ./internal/emit
#   -update), examples/rust-interop/rules (a fresh gopatina build
#   -emit-rust) and examples/counter/rust (written by hand);
# - the lines of README.md and docs/patina-subset.md that state the pin.
# Then it runs TestToolchainDrift, which fails if any copy was missed.
# It changes nothing when the pin is already <toolchain>, and commits
# nothing.
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <toolchain>" >&2
  exit 2
fi
new="$1"
if ! printf '%s' "$new" | grep -Eq '^([0-9]+\.[0-9]+\.[0-9]+|(beta|nightly)-[0-9]{4}-[0-9]{2}-[0-9]{2})$'; then
  echo "toolchain-bump: not a release or dated channel: $new" >&2
  exit 2
fi
cd "$(dirname "$0")/.."

old="$(jq -r .rust.pinned toolchain.json)"
if [ "$new" = "$old" ]; then
  echo "toolchain-bump: already pinned to $old"
  exit 0
fi
# $old as a sed pattern: its dots are literal.
pat="${old//./\\.}"

tmp="$(mktemp)"
jq --arg v "$new" '.rust.pinned = $v' toolchain.json >"$tmp"
mv "$tmp" toolchain.json
sed -i "s/^\(\tToolchain = \)\"$pat\"/\1\"$new\"/" internal/emit/crate.go
sed -i "s/^channel = \"$pat\"/channel = \"$new\"/" examples/counter/rust/rust-toolchain.toml
sed -i "s/emitted crates pin $pat;/emitted crates pin $new;/" README.md
sed -i "s/| \`rust-toolchain.toml\` | Rust $pat, minimal profile/| \`rust-toolchain.toml\` | Rust $new, minimal profile/" docs/patina-subset.md

# The drift test runs last, once every copy is rewritten.
go test ./internal/emit -skip TestToolchainDrift -update >/dev/null
rm -rf examples/rust-interop/rules.new
go run ./cmd/gopatina build -emit-rust -o examples/rust-interop/rules.new ./examples/rust-interop/go/rules >/dev/null
rm -rf examples/rust-interop/rules
mv examples/rust-interop/rules.new examples/rust-interop/rules

go test ./internal/emit -run TestToolchainDrift
echo "toolchain-bump: $old -> $new"
