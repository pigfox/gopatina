#!/usr/bin/env bash
# run.sh: the benchmark harness (bench/README.md).
#
# usage: bench/run.sh [--out results.json] [--runs N] [--toolchain same|shipped]
#                     [--rust TOOLCHAIN]
#        bench/run.sh --equiv [--out results.json] [--runs N] [--margin M]
#                     [--sigma S] [--metric wall|perf] [--toolchain same|shipped]
#                     [--rust TOOLCHAIN]
#
# For each case under bench/cases it builds three binaries: the Go program
# with gopatina build -patina, the hand-written Rust crate with cargo build
# --release, and the Go program with plain go build.
#
# Which rustc builds the two Rust binaries (--toolchain):
#   same     (the default, and the official comparison) both are built by
#            the same rustc, the toolchain --rust names (default stable,
#            the latest stable Rust): GoPatina's crate through a
#            RUSTUP_TOOLCHAIN override of its pinned toolchain. This
#            compares GoPatina's Rust with hand-written Rust, and nothing
#            else: different rustc releases generate different machine code
#            for the same source.
#   shipped  GoPatina's crate with the toolchain it pins (what a user gets
#            by default), the hand-written crate with --rust. A secondary
#            comparison, reported as such.
# The rustc that built each binary is read from its .comment section and
# recorded; in same mode a mismatch stops the run.
# It checks that all three print the case's expected output, runs the
# three N times, interleaved (after one warm-up run each), and reports the
# ratios of the medians: GoPatina and plain Go to hand-written Rust, and
# GoPatina to plain Go; and, from one more run of each under GNU time, the
# ratios of their peak resident memory to the hand-written Rust's.
#
# Only ratios are written: shared CI runners are too noisy for absolute
# numbers, which come from the maintainer's dedicated machine (policy in
# docs/QUALITY.md).
#
# With --equiv, it instead runs cmd/gopatina-bench on each case: GoPatina,
# hand-written Rust and plain Go, N runs each (default 400) in ABCCBA
# order, and a TOST equivalence test for every pair at the given margin
# (default 0.005, ±0.5%) and sigma (default 5) on wall time, peak memory
# and each arm's own interval and binary size. --metric perf also counts user instructions (then
# the primary metric) and user cycles where perf can read the CPU's counters,
# each with its own test; it is off by default. The report names the primary
# metric.
set -euo pipefail

out=bench-results.json
runs=""
equiv=false
margin=0.005
sigma=5
metric=wall
toolchain=same
rust_tc=stable
while [ "$#" -gt 0 ]; do
  case "$1" in
    --out) out="$2"; shift 2 ;;
    --runs) runs="$2"; shift 2 ;;
    --equiv) equiv=true; shift ;;
    --margin) margin="$2"; shift 2 ;;
    --sigma) sigma="$2"; shift 2 ;;
    --metric) metric="$2"; shift 2 ;;
    --toolchain) toolchain="$2"; shift 2 ;;
    --rust) rust_tc="$2"; shift 2 ;;
    *) echo "usage: $0 [--equiv] [--out file] [--runs N] [--margin M] [--sigma S] [--metric wall|perf] [--toolchain same|shipped] [--rust TOOLCHAIN]" >&2; exit 2 ;;
  esac
done
case "$toolchain" in
  same) gopatina_tc="$rust_tc" ;;
  shipped) gopatina_tc="" ;; # the crate's own rust-toolchain.toml
  *) echo "$0: --toolchain must be same or shipped" >&2; exit 2 ;;
esac
if [ -z "$runs" ]; then
  if $equiv; then runs=400; else runs=11; fi
fi

root="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

go build -o "$work/gopatina" "$root/cmd/gopatina"
go build -o "$work/gopatina-bench" "$root/cmd/gopatina-bench"
export GOPATINA_CACHE="$work/cache"

# time_ns <binary>: the wall time of one run, in nanoseconds.
time_ns() {
  local start end
  start=$(date +%s%N)
  "$1" >/dev/null
  end=$(date +%s%N)
  echo $((end - start))
}

# peak_kb <binary>: the peak resident memory of one run, in kilobytes, or
# "" without GNU time.
peak_kb() {
  if ! [ -x /usr/bin/time ]; then echo ""; return; fi
  /usr/bin/time -f '%M' -o "$work/peak" "$1" >/dev/null && cat "$work/peak"
}

# mem_ratio <a> <b>: a/b, or null when either is unknown.
mem_ratio() { if [ -z "$1" ] || [ -z "$2" ]; then echo null; else ratio "$1" "$2"; fi; }

# median: the median of the numbers on standard input.
median() { sort -n | sed -n "$(((runs + 1) / 2))p"; }

ratio() { awk -v a="$1" -v b="$2" 'BEGIN { printf "%.3f", a / b }'; }

# built_by <binary>: the rustc release recorded in the binary's .comment
# section, or "unknown" without readelf.
built_by() {
  if ! command -v readelf >/dev/null; then echo unknown; return; fi
  readelf -p .comment "$1" 2>/dev/null | grep -o 'rustc version [0-9][0-9.]*' | sort -u | head -1 | sed 's/rustc version //'
}

results=()
for dir in "$root"/bench/cases/*/; do
  name="$(basename "$dir")"
  expected="$(cat "$dir/expected")"
  if [ -n "$gopatina_tc" ]; then
    RUSTUP_TOOLCHAIN="$gopatina_tc" "$work/gopatina" build -patina -o "$work/$name-patina" "$dir/go"
  else
    "$work/gopatina" build -patina -o "$work/$name-patina" "$dir/go"
  fi
  go build -o "$work/$name-go" "$dir/go"
  (cd "$dir/rust" && RUSTUP_TOOLCHAIN="$rust_tc" cargo build --release --locked --quiet --target-dir "$work/target-$name")
  rust="$work/target-$name/release/$name"
  rustc_gopatina="$(built_by "$work/$name-patina")"
  rustc_rust="$(built_by "$rust")"
  echo "$name: GoPatina's binary built by rustc $rustc_gopatina, the hand-written Rust by rustc $rustc_rust ($toolchain toolchain)" >&2
  if [ "$toolchain" = same ] && [ "$rustc_gopatina" != "$rustc_rust" ]; then
    echo "$name: --toolchain same, but the two binaries were built by different rustc releases" >&2
    exit 1
  fi
  for bin in "$work/$name-patina" "$work/$name-go" "$rust"; do
    if [ "$("$bin")" != "$expected" ]; then
      echo "$name: $bin does not print the expected output" >&2
      exit 1
    fi
  done
  if $equiv; then
    # gopatina-bench prints the JSON on stdout and one line per metric on
    # stderr.
    r="$("$work/gopatina-bench" -runs "$runs" -margin "$margin" -sigma "$sigma" \
      -metric "$metric" -case "$name" -go "$work/$name-go" "$work/$name-patina" "$rust")"
    results+=("$r")
    continue
  fi
  # Warm up, then run the three interleaved, so drift affects all alike.
  for bin in "$rust" "$work/$name-patina" "$work/$name-go"; do "$bin" >/dev/null; done
  : >"$work/rust.t"; : >"$work/patina.t"; : >"$work/go.t"
  for ((i = 0; i < runs; i++)); do
    time_ns "$rust" >>"$work/rust.t"
    time_ns "$work/$name-patina" >>"$work/patina.t"
    time_ns "$work/$name-go" >>"$work/go.t"
  done
  t_rust="$(median <"$work/rust.t")"
  t_patina="$(median <"$work/patina.t")"
  t_go="$(median <"$work/go.t")"
  r_patina="$(ratio "$t_patina" "$t_rust")"
  r_go="$(ratio "$t_go" "$t_rust")"
  r_pg="$(ratio "$t_patina" "$t_go")"
  # Peak memory: one more run of each, the ratios of peak resident memory.
  m_rust="$(peak_kb "$rust")"
  m_patina="$(peak_kb "$work/$name-patina")"
  m_go="$(peak_kb "$work/$name-go")"
  mr_patina="$(mem_ratio "$m_patina" "$m_rust")"
  mr_go="$(mem_ratio "$m_go" "$m_rust")"
  echo "$name: gopatina/rust $r_patina, go/rust $r_go, gopatina/go $r_pg (medians of $runs runs); peak memory gopatina/rust $mr_patina, go/rust $mr_go"
  results+=("{\"case\": \"$name\", \"gopatina_over_rust\": $r_patina, \"go_over_rust\": $r_go, \"gopatina_over_go\": $r_pg, \"memory_gopatina_over_rust\": $mr_patina, \"memory_go_over_rust\": $mr_go}")
done

commit="$(git -C "$root" rev-parse HEAD 2>/dev/null || echo unknown)"
{
  echo "{"
  if $equiv; then
    echo "  \"note\": \"TOST for every pair of GoPatina, hand-written Rust and plain Go (cmd/gopatina-bench), and each arm's interval\","
  else
    echo "  \"note\": \"ratios only (median wall time over hand-written Rust); no absolute numbers come from shared runners\","
  fi
  echo "  \"runs\": $runs,"
  echo "  \"gopatina_commit\": \"$commit\","
  echo "  \"go\": \"$(go version)\","
  echo "  \"toolchain\": \"$toolchain\","
  echo "  \"rustc_gopatina\": \"$rustc_gopatina\","
  echo "  \"rustc_rust\": \"$rustc_rust\","
  echo "  \"rust_stable\": \"$(RUSTUP_TOOLCHAIN="$rust_tc" rustc --version)\","
  echo "  \"rust_pinned_for_gopatina\": \"$(cd "$root/testdata/golden/hello/rust" && rustc --version)\","
  echo "  \"os\": \"$(uname -srm)\","
  echo "  \"results\": ["
  for i in "${!results[@]}"; do
    sep=","
    if [ "$i" -eq $((${#results[@]} - 1)) ]; then sep=""; fi
    echo "    ${results[$i]}$sep"
  done
  echo "  ]"
  echo "}"
} >"$out"
echo "wrote $out" >&2
