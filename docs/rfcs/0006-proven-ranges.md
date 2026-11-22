# RFC 0006: Proven integer ranges and loop specialization

- **Status:** Accepted by the project lead on 2026-11-21 ("Approve RFC 0006
  with 60 and 1."); not implemented yet
- **Author(s):** GoPatina contributors
- **Created:** 2026-11-21
- **Discussion:** https://github.com/pigfox/gopatina/pull/37
- **Supersedes / superseded by:** none

## Summary

Teach the emitter to prove, from a loop's bounds, that the loop's integer
arithmetic cannot wrap and that its indexes are never negative. Where the
proof holds outright, the loop's induction variables are emitted as `usize`,
as a Rust programmer would write them. Where it holds only for some inputs,
the loop nest is emitted twice: a fast version for the proven case and the
current version for the rest, chosen by one comparison before the loop. Go
semantics are unchanged in both versions. The goal is to close the one known
gap between GoPatina's code and hand-written Rust in the benchmark suite,
`sieve`, and the class of loops it stands for.

## Motivation

The project's speed goal is identical speed to equivalent hand-written Rust,
meaning equivalent within ±0.5% at 5σ ([BENCHMARKS.md](../../BENCHMARKS.md)).
On 2026-11-21, `fib` and `suffix-sum` already compile to the same hot loops as
the hand-written Rust, and execute the same instructions to within 0.0003%.
`sieve` does not:

| Evidence (2026-11-21, laptop, not official) | GoPatina / hand-written Rust |
| --- | --- |
| User instructions (CPU counters, 300 runs, 5σ) | 1.302 |
| User cycles | 1.073 [1.055, 1.092] |
| Wall time | 1.067 [1.025, 1.110] |
| Machine code of `main` (`bench/asm-check.sh`) | 74 instructions against 56 |

The cause is in this loop:

```go
for i := 2; i < n; i++ {
	...
	for j := i * i; j < n; j += i {
		composite[j] = true
	}
}
```

Go's `int` becomes `i64`. The hand-written Rust uses `usize`, where the loop
condition `j < n` also proves `composite[j]` in bounds, so rustc drops the
bounds check. In the generated code, `j` is signed. Go's `i * i` and
`j += i` wrap on overflow, so `j` could in principle be negative. Then
`composite[j as usize]` must still panic the way Go's `composite[j]` would,
and rustc keeps a second compare in the inner loop.

The check is not waste in general. For `n` above about 3.04 × 10⁹, `i * i`
really does wrap, and Go then panics with an index error, while the
hand-written Rust silently leaves the loop. Removing the check is only
correct where GoPatina can prove the wrap cannot happen.

A prototype was written by hand to see whether the technique pays. It is the
generated function with one versioned loop nest: if `n <= 3_037_000_499`, the
same loops with `usize` induction variables; otherwise, the current code.
Compiled with the same rustc 1.93.0 and flags, its `main` was 56 instructions,
identical line for line (addresses masked) to the hand-written Rust's, and it
printed the same result. The prototype is evidence for the technique, not
generated output.

## Guide-level explanation

Nothing changes in the Go a programmer writes or in what a program does.
`-emit-rust` writes more Rust-like loops where it can prove them safe, and
`-explain` says so:

```text
./main.go:7:2: loop specialized for n <= 3037000499: i and j proven in [0, n); i*i and j+i cannot wrap
./main.go:7:2: loop kept general for n > 3037000499: i*i may wrap, so indexes keep Go's checks
```

The emitted Rust for `sieve` (abridged):

```rust
let mut composite: Vec<bool> = vec![false; rt::make_len(n, 1)];
let mut count: i64 = 0;
if n <= 3037000499 {
    let n_u = n as usize;
    for i in 2..n_u {
        if composite[i] {
            continue;
        }
        count += 1;
        let mut j = i * i;
        while j < n_u {
            composite[j] = true;
            j += i;
        }
    }
} else {
    // the loop exactly as GoPatina emits it today
}
```

## Reference-level explanation

### Facts

The analysis works on one function at a time and tracks, for each `int`
variable at each point, an interval `[lo, hi]` of mathematical integers,
where each bound is a constant or a loop-invariant variable plus a constant.
Facts come from:

1. **Constants and conversions** from smaller integer types (`int32`,
   `uint8`), whose ranges are their types' ranges.
2. **Counted loops.** In `for i := a; i < b; i++`, with `a` and `b`
   loop-invariant and `i` not assigned in the body, the body sees
   `i ∈ [a, b-1]`. `range` over a slice or an integer gives `[0, len-1]`.
3. **Monotone loops.** In `for j := e; j < b; j += d` (or the same as a
   condition loop with a single `j += d` at the end of the body), with `d`
   proven `>= 1` and loop-invariant, the body sees `j ∈ [lo(e), b-1]`.
4. **Allocation.** After `s := make([]T, n)` returns, `0 <= n` and
   `n * size(T) <= maxAlloc`. GoPatina's makeslice check (PR #36) panics
   otherwise, exactly as Go does. `len(s) == n` holds while `s` is not
   assigned, appended to or passed where it could be replaced.
5. **Guards.** Inside `if x < c { ... }` and similar, the comparison refines
   `x`.

An operation `x op y` with `op` in `+ - *` is **proven non-wrapping** when the
interval of its mathematical result lies inside `[MinInt64, MaxInt64]`. Then
Go's wrapping result equals the mathematical one, and so does Rust's.
Division, shifts and bitwise operators are not used as proofs in this RFC.

### When a loop nest is specialized

A loop nest (an outermost `for` and everything inside it) is specialized if
all of the following hold:

- Every slice index in the nest has a proven interval `[0, hi]` with `hi` a
  bound that also proves it below `len` of the slice (directly, or through
  fact 4), or the nest's other indexes are unaffected.
- Every `+`, `-` and `*` that feeds an index or an induction variable is
  proven non-wrapping, possibly under a **side condition**: one comparison
  of a loop-invariant variable against a constant, such as
  `n <= 3037000499`.
- Nothing in the nest takes the address of an induction variable, assigns
  it outside its increment, or calls a function that could change a
  variable a bound depends on.

With no side condition, the nest is emitted once, with the proven
induction variables as `usize`. With a side condition, it is emitted as
`if cond { fast } else { general }`. At most one side condition is used per
nest; a nest that would need two is not specialized. The fast copy is
emitted only if it is at most 60 lines, so code growth stays bounded; the
golden tests pin the limit. Both limits are the project lead's decision of
2026-11-21 ("Approve RFC 0006 with 60 and 1.").

### What the fast copy may change

- Proven induction variables and their bounds become `usize`. A use of one
  as a Go `int` value elsewhere (printing it, storing it, passing it) is
  `v as i64`, which is exact on `[0, isize::MAX]`.
- Indexing uses the `usize` directly. Rust's bounds check stays in the
  source; rustc removes it only where it can prove it, as in hand-written
  Rust. An out-of-range index, if one could still happen, panics with the
  same Go message as today.
- Nothing else changes: the same statements, in the same order, with the
  same side effects. The fast copy differs from the general copy only in
  types that are provably equal in value.

### Correctness argument

In the fast copy, every value that changed type lies in `[0, isize::MAX]`,
where `i64` and `usize` hold the same integers, and every arithmetic step on
those values is proven not to wrap. So each operation gives the same value in
both copies and in Go. Bounds checks are not removed by GoPatina, only made
provable for rustc. The side condition is evaluated once, before the nest,
on a loop-invariant value, so it cannot change while the nest runs.

### Tests

- **Golden tests** for each fact and each refusal: specialized with no side
  condition, specialized with one, refused for two side conditions, for an
  assigned induction variable, for an appended slice, and for the size
  limit.
- **Differential tests on both sides of each side condition.** Sieve's own
  threshold needs a 3 GB allocation, so the tests use shapes with small
  thresholds, such as `j := i * 1_000_000_000_000`, run with bounds just
  below and just above the threshold, where the general copy must reproduce
  Go's wrap-around and panics.
- **Generated programs.** `internal/progen` gains counted and monotone loops
  with random bounds around the thresholds. The 881,098-program defect-rate
  run (`cmd/gopatina-difftest`) is repeated with them before the change is
  accepted.
- **Machine code.** `bench/asm-check.sh` reports `sieve`'s hot loop
  identical to the hand-written Rust's. The case's `hot` file marks it as
  expected to match, so a regression is visible.
- **`-explain`** reports every specialization and every refusal with its
  reason, pinned by `explain.txt` golden files.

### Exit criteria

- On the test server, `sieve` is equivalent to the hand-written Rust within
  ±0.5% at 5σ (`bench/run.sh --equiv`), and `fib` and `suffix-sum` stay
  equivalent.
- 100% unit coverage, all golden, differential and fuzz layers green, and the
  defect-rate run with the new loop shapes finds no defect in 881,098
  programs.

## Compatibility

Default mode is untouched. In patina mode, no accepted program changes
behaviour; only the emitted Rust and `-explain` output change, and only for
loops the analysis proves. Emitted crates stay deterministic.

## Drawbacks

- A versioned nest appears twice in the emitted Rust, which is meant to be
  readable; the size limit and the `-explain` line keep that visible and
  bounded.
- The analysis is new code on the correctness-critical path. A wrong proof
  would turn a Go panic into silent wrong behaviour. This is why the tests
  above, especially both-sides differential tests and the defect-rate run,
  are exit criteria rather than extras.

## Rationale and alternatives

- **`get_unchecked` or `std::hint::assert_unchecked`.** Rejected. Both need
  `unsafe`, and they remove checks instead of letting rustc prove them; an
  analysis bug would become undefined behaviour instead of a wrong panic.
- **Changing the hand-written Rust to `i64`.** Rejected. The benchmark Rust
  is written the way a Rust programmer would write it, not tuned to flatter
  GoPatina ([bench/README.md](../../bench/README.md)).
- **`usize` for all loop variables.** Rejected. It changes Go semantics
  wherever a value can be negative or wrap.
- **Relying on LLVM.** rustc's `i64` arithmetic, with overflow checks off,
  is wrapping and carries no no-wrap flags, so LLVM cannot make the proof
  itself. That is the gap this RFC fills.
- **Constant bounds only, no versioning.** Simpler, but `sieve`'s `n` is a
  parameter, and most real loops are bounded by run-time values.
- **Doing nothing.** `sieve` stays about 7% slower and 30% more instructions,
  and the speed goal is not met for this class of loops.

## Prior art

- Go's own compiler has a `prove` pass that keeps a table of facts from
  branches and loop bounds to remove bounds checks, the same idea at the SSA
  level.
- LLVM's inductive range check elimination (IRCE) and loop versioning split
  a loop into a range where checks are provably redundant and a fallback.
- Java HotSpot's range check elimination does the same with pre- and
  post-loops.

## Decisions

- **Size limit:** a fast copy is at most 60 lines of emitted Rust.
- **Side conditions:** at most 1 per loop nest.

Both were decided by the project lead on 2026-11-21: "Approve RFC 0006 with
60 and 1."

## Unresolved questions

- 32-bit targets: `isize::MAX` and Go's `maxAlloc` are smaller there, so
  thresholds are computed per target from `go/types` sizes. Are 32-bit
  targets tested in CI?

## Future possibilities

- Facts from `len(s)` comparisons and from `min` and `max`.
- The same proofs for `int32` and `uint8` arithmetic.
- Feeding the facts to the borrow checker, for instance to prove two
  indexes distinct.
