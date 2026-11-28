#!/usr/bin/env bash
# go-bump.sh: move the Go release CI pins to a new one, rewriting
# toolchain.json and every copy of it at once.
#
# usage: scripts/go-bump.sh <go release>
#
# <go release> is a release (1.27.5) or a release candidate (1.28rc1),
# without the "go". It rewrites:
# - toolchain.json's go.pinned;
# - .go-version, which actions/setup-go reads (a release candidate as
#   1.28.0-rc.1, as setup-go spells it);
# - the fma-sites.txt records, from gc of that release (it is downloaded
#   through GOTOOLCHAIN when it is not the one installed). A change in where
#   gc fuses shows up here, and in the gates the bump runs.
# Then it runs TestToolchainDrift, which fails if any copy was missed.
# It changes nothing when the pin is already <go release>, and commits
# nothing.
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <go release>" >&2
  exit 2
fi
new="$1"
if ! printf '%s' "$new" | grep -Eq '^[0-9]+\.[0-9]+(\.[0-9]+|rc[0-9]+)$'; then
  echo "go-bump: not a Go release or release candidate: $new" >&2
  exit 2
fi
cd "$(dirname "$0")/.."

old="$(jq -r .go.pinned toolchain.json)"
if [ "$new" = "$old" ]; then
  echo "go-bump: already pinned to $old"
  exit 0
fi

tmp="$(mktemp)"
jq --arg v "$new" '.go.pinned = $v' toolchain.json >"$tmp"
mv "$tmp" toolchain.json
case "$new" in
  *rc*) printf '%s.0-rc.%s\n' "${new%rc*}" "${new#*rc}" >.go-version ;;
  *) printf '%s\n' "$new" >.go-version ;;
esac

GOTOOLCHAIN="go$new" go test ./internal/patina -run TestFMASites -update-fma-sites >/dev/null

go test ./internal/emit -run TestToolchainDrift
echo "go-bump: $old -> $new"
