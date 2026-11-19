# RFC 0005: Phase 1 semantic exactness fixes

- **Status:** Proposed; implemented in
  https://github.com/pigfox/gopatina/pull/27, awaiting lead acceptance
- **Author(s):** GoPatina contributors
- **Created:** 2026-11-18
- **Discussion:** PR #27
- **Supersedes / superseded by:** none

## Summary and motivation

Phase 1 promises that patina mode refuses anything it cannot translate
exactly. Two accepted translations and one missing built-in violated or
limited that promise:

1. Rust's own divide-by-zero and index-out-of-bounds panic text differs from
   Go's first stderr line. The differential tests used to translate that text
   before comparing, which meant the emitted binary itself was not Go-exact.
2. `strings.TrimSpace` used Rust's `is_whitespace` above U+00FF. Rust and Go
   can pin different Unicode versions, so the result could differ silently.
3. Go 1.21's `min` and `max` were rejected even for integers, where Rust's
   `std::cmp` has exactly the same result.

This RFC records the decisions taken for these three cases in PR #27.

## Guide-level explanation

A patina-mode binary now prints Go's `runtime error:` wording for the two
runtime panics phase 1 can produce, and exits with status 2. A program's own
`panic("...")` payload is printed verbatim, even when that payload happens to
look like Rust's runtime text.

`strings.TrimSpace` trims exactly Go's `unicode.White_Space` set, written out
in the generated runtime so neither Rust's Unicode version nor Go's can
change the result at run time.

`min` and `max` work on integers, including calls with more than two
arguments. Calls with float arguments are rejected with a precise diagnostic
because Rust's NaN rules differ from Go's.

## Reference-level explanation

- `internal/emit/call.go` prefixes every panic originating in a Go
  `panic(...)` call with `gopatina-user-panic:`. The hook in
  `internal/emit/rt.go` strips that marker and prints the payload unchanged;
  unmarked Rust panics are translated only for the two known runtime messages.
- `internal/emit/rt.go` writes Go's `unicode.White_Space` code points as an
  explicit `matches!` range set in `trim_space`.
- `internal/subset/subset.go` accepts `min`/`max` with at least two integer
  arguments. `internal/emit/call.go` lowers them to `std::cmp::min`/`max`
  with the Go result type named explicitly, folded from the left for calls
  with more than two arguments.
- Differential tests compare the emitted binary's first stderr line directly;
  they no longer normalise panic text.

## Drawbacks

The panic hook still parses Rust's panic strings for the two runtime errors,
so a future Rust toolchain that changes that text would need a matching update.
Full Go panic traces and fatal errors remain out of scope for phase 1.

## Alternatives considered

Normalising in the differential test was the previous approach. It made the
test suite pass while the emitted binary printed Rust text, so it was rejected
in favour of fixing the emitted program. Using Rust's `is_whitespace` was
rejected because it trades a silent miscompile for a smaller runtime. Mapping
`min`/`max` to Rust's float methods was rejected for the same reason.

## Unresolved questions

Whether to keep the two message translations in the runtime or replace them
with explicit bounds/divide checks in the emitter, which would remove the
dependence on Rust's panic wording.
