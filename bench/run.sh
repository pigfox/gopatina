#!/usr/bin/env bash
# run.sh: the benchmark harness (bench/README.md).
#
# usage: bench/run.sh [--out results.json] [--runs N]
#        bench/run.sh --equiv [--out results.json] [--runs N] [--margin M]
#                     [--sigma S] [--metric wall|perf]
#
# For each case under bench/cases it builds three binaries: the Go program
# with gopatina build -patina (the generated crate, built by rustc with its
# pinned toolchain), the hand-written Rust crate with cargo build --release
# (the latest stable Rust on the machine), and the Go program with plain go
# build. It checks that all three print the case's expected output, runs the
# three N times, interleaved (after one warm-up run each), and reports the
# ratios of the medians: GoPatina and plain Go to hand-written Rust, and
# GoPatina to plain Go.
#
# Only ratios are written: shared CI runners are too noisy for absolute
# numbers, which come from the maintainer's dedicated machine (policy in
# docs/QUALITY.md).
#
# With --equiv, it instead runs cmd/gopatina-bench on each case: GoPatina
# against hand-written Rust, N runs each (default 400) in ABBA order, and a
# TOST equivalence test at the given margin (default 0.01, ±1%) and sigma
# (default 5) on wall time. --metric perf also counts user instructions (then
# the primary metric) and user cycles where perf can read the CPU's counters,
# each with its own test; it is off by default. The report names the primary
# metric.
set -euo pipefail

out=bench-results.json
runs=""
equiv=false
margin=0.01
sigma=5
metric=wall
while [ "$#" -gt 0 ]; do
  case "$1" in
    --out) out="$2"; shift 2 ;;
    --runs) runs="$2"; shift 2 ;;
    --equiv) equiv=true; shift ;;
    --margin) margin="$2"; shift 2 ;;
    --sigma) sigma="$2"; shift 2 ;;
    --metric) metric="$2"; shift 2 ;;
    *) echo "usage: $0 [--equiv] [--out file] [--runs N] [--margin M] [--sigma S] [--metric wall|perf]" >&2; exit 2 ;;
  esac
done
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

# median: the median of the numbers on standard input.
median() { sort -n | sed -n "$(((runs + 1) / 2))p"; }

ratio() { awk -v a="$1" -v b="$2" 'BEGIN { printf "%.3f", a / b }'; }

results=()
for dir in "$root"/bench/cases/*/; do
  name="$(basename "$dir")"
  expected="$(cat "$dir/expected")"
  "$work/gopatina" build -patina -o "$work/$name-patina" "$dir/go"
  go build -o "$work/$name-go" "$dir/go"
  (cd "$dir/rust" && cargo build --release --locked --quiet --target-dir "$work/target-$name")
  rust="$work/target-$name/release/$name"
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
      -metric "$metric" -case "$name" "$work/$name-patina" "$rust")"
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
  echo "$name: gopatina/rust $r_patina, go/rust $r_go, gopatina/go $r_pg (medians of $runs runs)"
  results+=("{\"case\": \"$name\", \"gopatina_over_rust\": $r_patina, \"go_over_rust\": $r_go, \"gopatina_over_go\": $r_pg}")
done

commit="$(git -C "$root" rev-parse HEAD 2>/dev/null || echo unknown)"
{
  echo "{"
  if $equiv; then
    echo "  \"note\": \"TOST equivalence of GoPatina against hand-written Rust (cmd/gopatina-bench); ratios and intervals only\","
  else
    echo "  \"note\": \"ratios only (median wall time over hand-written Rust); no absolute numbers come from shared runners\","
  fi
  echo "  \"runs\": $runs,"
  echo "  \"gopatina_commit\": \"$commit\","
  echo "  \"go\": \"$(go version)\","
  echo "  \"rust_stable\": \"$(rustc --version)\","
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
