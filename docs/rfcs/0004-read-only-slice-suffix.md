# RFC 0004: Read-only slice suffix arguments

- **Status:** Accepted and merged in PR #26 (squash commit `f6d0a50` on
  `main`, 2026-11-18); implemented in `-emit-rust` and `-patina`
- **Author(s):** GoPatina contributors
- **Created:** 2026-11-18
- **Discussion:** Scope and panic contract approved by the project lead;
  implementation and review in
  https://github.com/pigfox/gopatina/pull/26
- **Supersedes / superseded by:** none; answers one part of
  [RFC 0001's slice question](0001-gopatina.md#open-questions)

## Summary and motivation

Allow `read(s[i:])` as a standalone call to a read-only function. This
lets a caller skip a prefix without copying or changing the callee to take
a separate offset. It does not introduce general slice views or change
the current `Vec<T>` representation of owned slices.

## Guide-level explanation

```go
func show(s []int) { fmt.Println(s, len(s)) }

// In main:
s := []int{10, 20, 30}
i := 1
show(s[i:]) // [20 30] 2
s[1] = 99   // allowed: the borrow ended when show returned
show(s[i:]) // [99 30] 2
```

Default and patina modes accept this in the local implementation.
`-emit-rust` passes a borrowed `&[i64]` to `show`, and `-explain` reports
why the suffix can be shared.

## Reference-level explanation

All of the following must hold (ignore parentheses around expressions):

1. `s[i:]` is the sole argument of a standalone call statement. The
   callee is a directly named, package-local Go function with one
   parameter of type `[]int` and no results. Not a built-in, method,
   imported function, or function value.
2. `s` is an identifier naming an ordinary local variable or parameter of
   type `[]int`. `i` is an `int` local or parameter, or an integer literal
   or named integer constant representable as `int`. No calls, indexing,
   dereferences, or computed bounds. The low bound is required; the high
   and max bounds must be omitted.
3. The callee's slice parameter must be `Ref` after borrow inference has
   reached its fixed point: no writes through the view, no reassignment
   of this parameter, no escape or transfer of ownership, including
   through another callee. For this first increment, also check that all
   uses of that parameter are `len`, reading an indexed element, ranging,
   supported `fmt` operations that display the slice, or forwarding to a
   package-local function parameter that meets the same read-only checks.
   A suffix forwarded under this RFC is subject to the same checks. Any
   other or unclassified use rejects the view; do not clone or promote
   it to owned. Existing borrow conflicts for the base still apply.

In particular, reject `s[:]`, `s[i:j]` and `s[i:j:k]`; bases that are
strings, arrays, fields, or expressions other than a local/parameter;
other element types; saved, returned, nested, or chained views; writes,
owned or mutable callee parameters, multiple arguments, and side effects
in the bounds. Keep `cap`, `copy`, and comparisons of slices with `nil`
outside the subset. Diagnose rejected forms in Go terms before emission.

Go's omitted high bound is `len(s)`, not `cap(s)`. A valid low bound is
`0 <= i <= len(s)`, including equal-to-length and nil/empty at zero. The
view shares backing elements in Go; a shared Rust borrow sees those same
elements during this one call. The permitted element type holds no alias,
the callee may neither mutate nor retain the view, and neither it nor any
allowed operation can observe the suffix's capacity or nil identity.
There is no extra argument, effectful base or bound, or later operand to
interleave. Evaluate the base and low bound once at the call site and
check bounds before any callee code runs. An invalid low must panic even
if the callee would ignore the view or exit immediately. Preserve earlier
stdout, the Go-style first stderr line, and exit 2; Go stack frames are
not part of the comparison.

## Evidence and implementation

Run `go test ./internal/patina -run '^TestSliceGoSemantics$'` for the
[accepted goldens](../../testdata/golden) and the
[Go-only counterexamples](../../testdata/go_semantics). Set
`GOPATINA_RUST_TESTS=1` to compare the accepted fixtures with Rust. Output
below uses `\n` to show newlines; `before ` ends with a space and no newline.

| Fixture | Go stdout | Go exit / first stderr line | Patina result |
| --- | --- | --- | --- |
| `slice_suffix` | `[20 30] 2\n[] 0\n[99 30] 2\n[10 99 30]\n[] 0\n[] 0\n4\n` | `0` | Match |
| `slice_suffix_high` | `before ` | `2`: `panic: runtime error: slice bounds out of range [3:2]` | Match, even though `cap(s)` is 4 |
| `slice_suffix_negative` | `before\n` | `2`: `panic: runtime error: slice bounds out of range [-1:]` | Match |
| `slice_explicit_high` | `[7 0 0] 3\n` | `0` | Reject; an explicit high can reach beyond length |
| `slice_suffix_write` | `[10 99 30]\n` | `0` | Reject; a forwarded write aliases the base |
| `slice_suffix_effects` | `low\n[20]\n` | `0` | Reject; compute the bound in a separate statement |

1. **Loader:** `go/types` supplies the base and bound information and
   rejects invalid constant bounds before the subset gate. The loader
   configures `types.Sizes` from `GOARCH`, so a bound that fits a 64-bit
   `int` but overflows a 32-bit one is rejected for a 32-bit target.
2. **Subset:** Uses `go/types` object identity to recognize the exact call
   and view shape, leaving the general `ast.SliceExpr` rejection in place.
3. **Borrow:** Walks base and bound as reads throughout inference. After
   the fixed point, it validates the callee's `Ref` mode and audits its uses,
   following forwarded parameters (visit each once, including cycles).
   It applies existing conflicts to the base and records a per-call suffix
   decision and a source-positioned explanation. The existing-syntax
   [forwarded write regression](../../testdata/golden/slice_forward)
   checks that a wrapper around a writer is not mistaken for a reader.
4. **Emitter:** Lowers only approved views, in the shared argument path,
   through a checked `&[i64]` suffix helper. It checks the signed Go `int`
   bound before converting to `usize` and uses the existing panic hook for
   exit 2. No allocation, clone, `unsafe`, or Rust capacity access.
5. **Tests:** Rust goldens, `-explain` snapshots, exact rejection
   diagnostics, and Go/Rust comparisons cover the successful cases and
   the panic contract. The golden cases also seed the emitter fuzz test.
   See [QUALITY.md](../QUALITY.md) for the coverage, Rust, and benchmark
   checks required for codegen changes.

## Compatibility and drawbacks

Default mode and already-supported source are unchanged. This does not
accept Python-style clipping or Rust's default panic text. The restriction
to `[]int` and a one-argument, no-result call is deliberately narrow; real
code using `[]byte`, a returned scalar, or a reusable local view would
still need another spelling.

## Alternatives and follow-ups

Go's [slice rules](https://go.dev/ref/spec#Slice_expressions) permit an
explicit high bound up to capacity; Rust's `&[T]` is bounded by length.
Copying would add cost and hide the alias behavior of writers. A shared
backing store with explicit length and capacity could model more Go
semantics, but is unnecessary here. Defer mutable views, additional
element types, general bounds and contexts, and capacity-aware storage.
