#!/usr/bin/env bash
# asm-check.sh: compare the machine code of GoPatina's generated Rust with
# the hand-written Rust, case by case (bench/README.md).
#
# usage: bench/asm-check.sh [--out dir] [--toolchain TOOLCHAIN]
#
# For each case under bench/cases it emits the Go program's crate
# (gopatina build -emit-rust), compiles the emitted src/main.rs and the
# hand-written src/main.rs with the same rustc and the same release flags,
# disassembles the functions listed in the case's `hot` file (one Rust path
# per line, such as `fib` or `main`; functions the compiler inlines are
# compared through their caller), masks addresses, and reports for each one
# the instruction counts and the number of differing lines. The listings and
# their diffs are kept in the output directory.
#
# Both crates are compiled by the same rustc: the default one, or the rustup
# toolchain --toolchain names (such as 1.93.0, the version emitted crates
# pin, or stable).
#
# This is a report, not a gate: a difference is evidence to read, and the
# exit status is 0 unless a build fails. Wall-clock noise does not affect it.
set -euo pipefail

out=""
tc=()
while [ "$#" -gt 0 ]; do
  case "$1" in
    --out) out="$2"; shift 2 ;;
    --toolchain) tc=("+$2"); shift 2 ;;
    *) echo "usage: $0 [--out dir] [--toolchain TOOLCHAIN]" >&2; exit 2 ;;
  esac
done

root="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
if [ -z "$out" ]; then out="$work/asm"; fi
mkdir -p "$out"

go build -o "$work/gopatina" "$root/cmd/gopatina"
export GOPATINA_CACHE="$work/cache"
flags=(-C opt-level=3 -C codegen-units=1 -C overflow-checks=off
  -C debug-assertions=off --edition 2024)
echo "rustc: $(rustc "${tc[@]}" --version)"

# listing <binary> <function>: the function's instructions, addresses and
# symbol names masked so two binaries compare by code alone.
listing() {
  objdump -d --no-show-raw-insn -C "$1" | awk -v f="<main::$2>:" '
    index($0, f) { p = 1; next }
    p && /^$/ { p = 0 }
    p { sub(/^ *[0-9a-f]+:\t/, ""); gsub(/[0-9a-f]{4,}/, "A"); gsub(/<[^>]*>/, ""); print }'
}

for dir in "$root"/bench/cases/*/; do
  name="$(basename "$dir")"
  [ -f "$dir/hot" ] || continue
  "$work/gopatina" build -emit-rust -o "$work/$name-crate" "$dir/go" >/dev/null
  rustc "${tc[@]}" "${flags[@]}" -o "$work/$name-emit" "$work/$name-crate/src/main.rs"
  rustc "${tc[@]}" "${flags[@]}" -o "$work/$name-hand" "$dir/rust/src/main.rs"
  while read -r fn; do
    [ -n "$fn" ] || continue
    listing "$work/$name-emit" "$fn" >"$out/$name.$fn.gopatina.s"
    listing "$work/$name-hand" "$fn" >"$out/$name.$fn.rust.s"
    e="$(wc -l <"$out/$name.$fn.gopatina.s")"
    h="$(wc -l <"$out/$name.$fn.rust.s")"
    diff "$out/$name.$fn.gopatina.s" "$out/$name.$fn.rust.s" >"$out/$name.$fn.diff" || true
    d="$(grep -c '^[<>]' "$out/$name.$fn.diff" || true)"
    if [ "$d" -eq 0 ]; then verdict=identical; else verdict="differs ($d lines)"; fi
    echo "$name main::$fn: gopatina $e instructions, rust $h instructions, $verdict"
  done <"$dir/hot"
done
echo "listings and diffs in $out" >&2
