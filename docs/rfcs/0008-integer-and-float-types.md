# RFC 0008: The remaining integer types and float32

- **Status:** Approved by the coordinator on the project lead's authority
  (2026-11-22, merged in #57); implemented 2026-11-22 (see "Implementation
  notes")
- **Author(s):** GoPatina contributors
- **Created:** 2026-11-22
- **Discussion:** the pull request that adds this file
- **Supersedes / superseded by:** none

## Summary

Accept, in patina mode, Go's remaining numeric types: `int8`, `int16`,
`uint`, `uint16`, `uint32`, `uint64`, `uintptr` and `float32`, with Go's
exact arithmetic, conversions, constants and printing. Each maps to the Rust
primitive of the same width. Where Go and Rust differ (wrapping, division,
shifts, float-to-integer conversion, float formatting), the emitter uses the
rules phase 1 already applies to `int`, `int32`, `uint8` and `float64`,
extended to every width, and every rule is checked against Go's compiler.

## Motivation

The goal is 100% of real Go functions other than those using `reflect` or
`unsafe` (the project lead, 2026-11-21). In the coverage measurement's
greedy roadmap, "other int and float types" is one of the next steps after
named types and types from other packages. Fixed-width integers are
everywhere in real Go: hashes and checksums (`uint32`, `uint64`), byte and
bit manipulation (`uint16`, `uint64`), sizes (`uint`), protocols (`int16`,
`int8`), graphics and numerics (`float32`).

## Guide-level explanation

```go
func fnv32(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

func main() {
	var m int8 = -128
	var a float32 = 0.1
	fmt.Println(fnv32("gopatina"), m/-1, -m, a*3, float64(a))
	f, g := 70000.5, -2.5
	fmt.Println(uint64(1<<64-1)/3, uint16(f), uint64(g))
}
```

prints, as Go prints it,

```
1195377846 -128 -128 0.3 0.10000000149011612
6148914691236517205 4464 18446744073709551614
```

and the Rust is ordinary fixed-width arithmetic, roughly (the exact output
will be in the golden programs):

```rust
fn fnv32(s: &str) -> u32 {
    let mut h: u32 = 2166136261;
    let mut i: i64 = 0;
    while i < s.len() as i64 {
        h ^= s.as_bytes()[i as usize] as u32;
        h *= 16777619;
        i += 1;
    }
    h
}
```

## Reference-level explanation

### Types

| Go | Rust |
| --- | --- |
| `int8`, `int16` | `i8`, `i16` |
| `uint16`, `uint32`, `uint64` | `u16`, `u32`, `u64` |
| `uint`, `uintptr` | `u64` |
| `float32` | `f32` |

`int` is already `i64`, so `uint` and `uintptr` are `u64`: every target
GoPatina builds for (`internal/patina`) is 64-bit. `uintptr` is an ordinary
unsigned integer here; nothing converts it to or from a pointer, since
`unsafe` is out of scope. Slices, struct fields, named types (RFC 0007) and
constants of these types follow from the existing rules.

### Arithmetic

The rules phase 1 applies today, for every width:

- Overflow wraps: emitted crates turn Rust's overflow checks off in every
  profile, so `+`, `-`, `*`, unary `-` (`wrapping_neg` for unsigned types),
  `++`, `--` and the `op=` forms wrap as in Go.
- A signed division or remainder by a non-constant divisor uses
  `wrapping_div` and `wrapping_rem`, so `int8(-128) / -1` is `-128` and
  `% -1` is `0`. Division by zero panics with Go's message, for every
  width.
- Shifts by a constant count use Rust's operator, and a count at least the
  width gives Go's 0 or −1 (`wideShift`, extended to 8, 16, 32 and 64
  bits). Variable shift counts stay outside the subset (a later RFC).
- `float32` arithmetic is IEEE single precision in both languages: each
  operation rounds to `float32`. On amd64 gc does not fuse `x*y + z` into a
  fused multiply-add, and neither does Rust. **Known gap, not part of this
  RFC:** gc does fuse on arm64 (and ppc64le, s390x, riscv64), which the Go
  specification permits; GoPatina never fuses, so `float32` and `float64`
  results can differ from gc's on arm64. This is untested here (no arm64
  machine) and is recorded in `docs/go-rust-disconnects.md`.
- `min` and `max` accept every integer type, as now; float arguments stay
  rejected.

### Conversions

Measured against gc 1.27.1 on amd64; the reference and optimized builds
agree on every case below.

- **Integer to integer**: Rust's `as` truncates and sign- or zero-extends
  exactly as Go does.
- **Integer to float**: Rust's `as` rounds to nearest, ties to even, once,
  as gc does (including `uint64` values above 2^63, measured).
- **`float64` to `float32`**: `as`, round to nearest even. **`float32` to
  `float64`**: exact.
- **Float to integer**: Go leaves an out-of-range result to the
  implementation, and the oracle is gc. gc on amd64:
  - `int8`, `int16`, `int32`, `uint8`, `uint16`: through `int32`
    (CVTTSD2SL: `MinInt32` for NaN and for a value whose truncation does not
    fit), then truncated: `uint16(-70000.5)` is 61072, `int8(70000.5)` is
    112.
  - `int`, `int64`, `uint32`: through `int64` (CVTTSD2SQ: `MinInt64` for
    NaN and non-fitting values), then truncated: `uint32(4294967301)` is 5.
  - `uint`, `uint64`, `uintptr`: below 2^63, through `int64`, so negative
    values wrap (`uint64(-2.5)` is 18446744073709551614); otherwise the
    `int64` conversion of `x − 2^63` with the top bit set, which gives 2^63
    for NaN, the infinities and values of 2^64 and above.
  - A `float32` operand behaves exactly as its (exact) `float64` widening.

  These are `rt::f64_to_i32`, `rt::f64_to_i64` (both since the fix of
  2026-11-22) and a new `rt::f64_to_u64`; on other architectures they are
  Rust's saturating conversion, gc's arm64 lowering, untested on arm64.
- **String conversions**: `string(r)` for an integer `r` stays rejected, as
  now.

### Constants

`go/types` gives every typed constant its exact value; the emitter writes
it as a literal of the Rust type (`200_u8`, `65535_u16`). A `float32`
constant is first rounded to `float32` (as Go does when the constant is
typed) and written with the shortest digits that read back as the same
`f32`.

### Printing

- Integers of every width print with `%v`, `%d`, `%x`, `%X`, `%c` and the
  flags phase 1 supports, as now: Rust's formatting of `i8` to `u64` is
  Go's for these verbs, `%x` of a negative signed value goes through
  `rt::hex` (Go prints `-ff`, Rust two's complement), and `%c` through
  `rt::rune`.
- `float32` prints with `%v` as Go does: the shortest digits that read back
  as the same `float32` (Go's `strconv.FormatFloat(x, 'g', -1, 32)`), with
  Go's exponent layout and an exact tie broken to even, as `rt::Float` does
  for `float64` since 2026-11-22: `0.1`, `0.033333335`, `1.6777216e+07`.
  With a precision (`%.3f`), the digits are those of the exact value, the
  same in both languages.
- **Panic messages** with an index of an unsigned 64-bit type: Go prints
  the index as an unsigned number (`index out of range
  [18446744073709551615] with length 3`); the runtime's translation of
  Rust's message reads it as signed today and will print it by the index's
  Go type.

## Compatibility

Default mode is unchanged. Patina mode only accepts programs it rejected
before; every program it accepts today emits the same Rust.

## Testing

- **Golden programs**: arithmetic of each type at its edges (MinInt8 / −1,
  `uint16` wrap, `uint64` near 2^64, shifts at and past each width);
  conversions between every pair of integer widths; the float-to-integer
  table above for every target type, with NaN and the infinities; `float32`
  arithmetic, constants, conversions and printing (including ties); `%x`,
  `%X`, `%c` and `%d` of each type; an index of each unsigned type out of
  range.
- **Rejections**: variable shift counts and `string(r)`, still rejected,
  with their messages.
- **Generator** (`internal/progen`): the named-type part draws its
  underlying type from every integer width, and the float part adds
  `float32` arithmetic, conversions to every integer type and printing,
  each from its own random stream, so existing programs stay as they are.
- **Differential run**: a part on the test server with the new generator,
  0 defects required; the 6σ run then continues on the compiler with this
  RFC.

## Drawbacks

- More runtime helpers (`rt::f64_to_u64`, `float32` printing). Each is
  small and only written into crates that use it.
- The float-to-integer rules are target-specific in Go, so GoPatina's
  output for out-of-range values is too; it matches gc on the same target,
  verified on amd64 only.

## Rationale and alternatives

- **Saturating conversions everywhere** (Rust's `as`) would be simpler, and
  equal to gc on arm64, but differ from gc on amd64 for NaN and every
  out-of-range value. The tests hold GoPatina to gc on the target.
- **Rejecting `uintptr`**: it is used in real code as a plain integer
  (hashes, sizes); without `unsafe` it is one.

## Prior art

- gc's own lowering (`cmd/compile/internal/ssagen`, the amd64 rules) is the
  specification of the conversions above; GoPatina reproduces its results.
- TinyGo compiles through LLVM, whose `fptosi` is undefined (poison) for
  out-of-range values; it does not promise gc's results.

## Implementation notes

- As specified, with each rule measured against gc 1.27.1 on amd64 and
  tested by the golden program `numeric_types` and the generator.
- **Found while implementing (pre-existing):** a slice literal printed
  without a type from its context (`fmt.Println([]int{1 << 40})`,
  `len([]int{-1 << 63})`) was written as `vec![...]` with untyped
  constants, which Rust takes as `i32`, so it did not compile; a `float32`
  slice literal would have been `f64`. The first constant element now
  carries its type when Rust's default would be wrong.
- Printing a slice of the new integer types or of `float32` uses separate
  runtime pieces (`show_ints`, `show_f32`), written only into crates that
  need them.

## Unresolved questions

- The arm64 fused multiply-add gap above: whether to reproduce gc's fusion
  rules (`mul_add` where gc fuses) needs arm64 hardware to verify, and is
  left to a later change.

## Future possibilities

- Variable shift counts, with Go's panic for a negative count.
- `complex64` and `complex128`.
