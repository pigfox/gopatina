#!/usr/bin/env bash
# shellcheck disable=SC2329 # the gate functions run through gate()
# gates.sh: run the local gates a change must pass before review, with the
# Go toolchain .go-version pins, the one CI uses. gc's float multiply-add
# fusion differs between Go releases (docs/QUALITY.md), so a gate run with
# another installed Go can pass what CI rejects. GOTOOLCHAIN fetches the
# pinned release from golang.org into the module cache once; the system Go
# is not changed.
#
# usage: scripts/gates.sh [--parity <out-dir>] [--no-rust] [--no-difftest]
#
#   --parity <out-dir>  also run gopatina-parity against
#                       testdata/parity/baseline.json, writing
#                       <out-dir>/parity.json and parity.md
#   --no-rust           skip the GOPATINA_RUST_TESTS suites (cargo, rustfmt,
#                       clippy), for a quick run
#   --no-difftest       skip the differential test of generated programs
#
# Each gate prints one line, "ok <gate>" or "FAIL <gate>"; the script exits
# 1 if any gate failed. Logs go to a temporary directory it names.
set -uo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root" || exit 2

parity="" rust=1 difftest=1
while [ "$#" -gt 0 ]; do
  case "$1" in
    --parity) parity="${2:?--parity needs a directory}"; shift 2 ;;
    --no-rust) rust=0; shift ;;
    --no-difftest) difftest=0; shift ;;
    *) echo "usage: $0 [--parity <out-dir>] [--no-rust] [--no-difftest]" >&2; exit 2 ;;
  esac
done

GOTOOLCHAIN="go$(cat .go-version)"
export GOTOOLCHAIN
logs="$(mktemp -d)"
echo "go: $(go version) (GOTOOLCHAIN=$GOTOOLCHAIN); logs in $logs"

failed=0
# gate <name> <command>...: run one gate. The functions below are run
# through it, which shellcheck does not see (SC2329).
gate() {
  local name="$1"
  shift
  if "$@" >"$logs/$name.log" 2>&1; then
    echo "ok $name"
  else
    echo "FAIL $name (see $logs/$name.log)"
    failed=1
  fi
}

gofmt_clean() {
  local files
  mapfile -t files < <(git ls-files '*.go' | grep -v '^testdata/')
  test -z "$(gofmt -l "${files[@]}")"
}
coverage() {
  go test -race -covermode=atomic -coverprofile="$logs/coverage.out" ./... || return 1
  local total
  total="$(go tool cover -func="$logs/coverage.out" | awk '/^total:/ {print $NF}')"
  echo "total coverage: $total"
  [ "$total" = "100.0%" ]
}

gate gofmt gofmt_clean
gate vet go vet ./...
gate test-race-coverage coverage
if [ "$rust" = 1 ]; then
  gate rust-patina env GOPATINA_RUST_TESTS=1 go test ./internal/patina -timeout 60m
  gate rust-examples env GOPATINA_RUST_TESTS=1 go test ./examples/... ./internal/emit
fi
if [ "$difftest" = 1 ]; then
  difftest() {
    go build -o "$logs/gopatina" ./cmd/gopatina &&
      go run ./cmd/gopatina-difftest -seed 1 -start 0 -programs 400 -per 200 -jobs 3 -gopatina "$logs/gopatina" >"$logs/difftest.json" &&
      sed -n '/^{/,$p' "$logs/difftest.json" | jq -e '.defects == 0 and .go_disagreements == 0 and .lint_failed == 0' >/dev/null
  }
  gate difftest difftest
fi
if [ -n "$parity" ]; then
  mkdir -p "$parity"
  parity_run() {
    go build -o "$logs/gopatina" ./cmd/gopatina &&
      go run ./cmd/gopatina-parity -gopatina "$logs/gopatina" -json "$parity/parity.json" -md "$parity/parity.md" -baseline testdata/parity/baseline.json
  }
  gate parity parity_run
fi
exit "$failed"
