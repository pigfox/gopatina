# RFC 0009: Programs of many packages, compiled from source

- **Status:** Draft
- **Author(s):** GoPatina contributors
- **Created:** 2026-11-22
- **Discussion:** the pull request that adds this file
- **Supersedes / superseded by:** none

## Summary

Let a patina program use other packages: the packages of its own module,
packages of other modules, and standard-library packages. Each Go package
becomes a Rust module of the one crate, translated from its Go source by
the same pipeline as `package main`, so Go's semantics carry over by
construction. Only what `main` can reach is translated, and a program is
accepted when everything it reaches is inside the subset. A small table of
**intrinsics** replaces standard-library functions that gc itself replaces
with machine instructions (`math/bits.Add64`, `Mul64`, `RotateLeft64`, ...)
with Rust that gives gc's results, including outside their documented
domain. The coverage measurement gets the same cross-package rule.

## Motivation

The coverage measurement at fb0f721 ranks "types from other packages" first
on the roadmap. Its diagnostics, split by where the type is declared
(`foreign_types_by_origin`, 2026-11-22):

| Origin | Diagnostics |
| --- | --- |
| standard library | 21,118 |
| the measured package's own module | 18,905 |
| other modules | 2,777 |

Nearly half are types of the program's own other packages: real programs
are many packages. And the most frequent standard-library calls are pure
functions (`math/bits.Add64`, `Mul64`, `Sub64`, `RotateLeft32/64`, about
4,200 diagnostics). (The shares themselves are reviewed before anything is
published, so only these counts of diagnostics are quoted.)

## Guide-level explanation

```
example.com/calc/
  go.mod                 module example.com/calc
  main.go                package main; imports example.com/calc/num
  num/num.go             package num
```

```go
// num/num.go
package num

import (
	"fmt"
	"math/bits"
)

type Wide struct{ Hi, Lo uint64 }

func Mul(x, y uint64) Wide {
	hi, lo := bits.Mul64(x, y)
	return Wide{Hi: hi, Lo: lo}
}

func (w Wide) String() string { return fmt.Sprintf("%d:%d", w.Hi, w.Lo) }
```

```go
// main.go
package main

import (
	"fmt"

	"example.com/calc/num"
)

func main() { fmt.Println(num.Mul(1<<63, 6)) }
```

It prints `3:0` (2^63 × 6 is 3 × 2^64), run with Go 1.27.1.
`gopatina build -patina` translates both packages into one crate:

```
src/main.rs            the main package, with mod num_0;
src/num_0.rs           package example.com/calc/num
src/rt.rs
```

and `num.Mul(...)` becomes `crate::num_0::mul(...)`, with `bits.Mul64`
written as Rust's widening multiply.

## Reference-level explanation

### Which packages

- **The build list.** `go list -deps -json` from the main package gives
  every package the program imports, directly or not, with its files for the
  target `GOOS`/`GOARCH` and build tags, exactly the files `go build` would
  compile. GoPatina loads and type-checks each one from source with the
  importer the subset checker already uses.
- **Bridged packages.** The packages phase 1 maps straight to Rust (`fmt`,
  the supported functions of `strings`, `errors`, `os`, `io`, `slices`,
  `bytes`) stay as they are: their calls are written as Rust, not
  translated.
- **Every other package**, of the main module, another module or the
  standard library, is translated from source.

### What is translated: reachability

Starting from `main` (and the `init` functions of every package), GoPatina
follows the static references of the program: calls, types, constants,
methods of the types used. Interfaces and function values are outside the
subset, so every call has one known target. The program is accepted when
every reachable declaration passes the subset checker and the borrow
checker; one that does not is reported where it is, in its own package
(`net/http/server.go:...: a map type is not supported...`), together with
the chain of uses from `main` that reaches it. Declarations nothing reaches
are not translated, so importing a large package is fine when the program
uses a small, supported part of it.

### Rust layout

- Each translated package is one Rust module, `src/<name>_<n>.rs`, where
  `<name>` is the package name (snake case) and `<n>` a number that keeps
  equal names apart; `mod` lines in `main.rs` declare them all.
- Every item is `pub(crate)`: Go's exported/unexported rule has already been
  enforced by `go/types`, and the crate is one program.
- References across packages are paths: `crate::num_0::mul`,
  `crate::num_0::Wide`. Names inside a module follow the existing rules, per
  module.
- **Initialization order** is Go's: packages initialize in dependency
  order, imported before importer, and among those ready, by import path
  (the Go 1.21 rule); within a package, its `init` functions in file and
  source order. `main` calls them in that order before its own body.
  (Package-level variables are outside the subset until their RFC; when they
  arrive, their initialization joins this order as Go defines it.)

### Intrinsics

gc replaces some standard-library functions with machine instructions, and
for inputs outside the documented domain the instruction's result is not
what the Go source computes. Measured with Go 1.27.1 on amd64, in both the
optimized and the reference (`-gcflags=all=-N -l`) build:
`bits.Add64(1<<64-1, 1, 2)` returns `(1, 1)`, while the same function
compiled from `math/bits`' source returns `(2, 1)`. The oracle is gc, so
for these functions GoPatina writes Rust that gives gc's result, not the
source's:

| Function | Rust | Outside the documented domain |
| --- | --- | --- |
| `bits.Add64(x, y, c)`, `Add32` | `overflowing_add` twice | a carry other than 0 or 1 counts as 1 (ADC reads the carry flag set from `c != 0`), measured |
| `bits.Sub64`, `Sub32` | `overflowing_sub` twice | likewise, as SBB |
| `bits.Mul64`, `Mul32` | widening multiply (`u128`, `u64`) | none |
| `bits.RotateLeft*` | `rotate_left` / `rotate_right` | a negative `k` rotates right, as documented |
| `bits.LeadingZeros*`, `TrailingZeros*`, `OnesCount*`, `Len*`, `Reverse*`, `ReverseBytes*` | `leading_zeros`, `trailing_zeros`, `count_ones`, `BITS - leading_zeros`, `reverse_bits`, `swap_bytes` | `TrailingZeros(0)` is the width, as both define it |

Each row is checked against gc, in and out of its domain, by a golden
program, and the table lists only functions gc intrinsifies on every target
GoPatina builds for; any other function is translated from source.

### The coverage measurement

The measurement now treats each package alone, as a program. It changes to
the same rule as builds: a function is accepted when it and everything it
uses, in any package of the corpus, is accepted (a fixed point over the whole
corpus, not each package), with the bridged packages and the intrinsics
accepted. Standard-library functions outside the corpus's measured packages
are not used, since the corpus includes all of `std`.

## Dependencies on later RFCs

This RFC is the mechanism; how much of a package it reaches depends on the
rest of the subset. Measured top types and calls that stay out until their
own features arrive:

- **Package-level variables** (`io.EOF`, `binary.BigEndian`, lookup tables in
  `math/bits.Len*`, `unicode`): the "package vars" roadmap step.
- **Interfaces**: `context.Context`, `io.Reader`, `net.Conn`,
  `protoreflect.Message`, `go/ast.Expr`.
- **Arrays**: `time.Duration.String` formats into a `[32]byte`.
- **Runtime-backed functions** (no Go body: `time.Now`, `sync` primitives,
  `syscall`): out of reach of translation; the ones programs need get
  bridges in later RFCs (time, concurrency in RFC 0002).

## Compatibility

Default mode is unchanged. A single-package program emits the same crate as
before, except for the new module lines when it imports a translated
package.

## Testing

- **Golden programs of several packages** (the golden harness learns
  directories with `go.mod` and subpackages): a program over packages of its
  own module, with types, methods, constants and functions across packages;
  name collisions between packages and with Rust names; `init` order across
  packages, including the import-path tie-break; a package reached only in
  part (an unsupported function nothing calls); the intrinsics table, in and
  out of domain.
- **Rejections**: an unsupported declaration reached from `main` through
  another package, reported in that package with the chain of uses; a
  package with cgo files; a reached function without a body.
- **Generator**: batches gain a second package of the same module, with
  named types, functions and methods the programs call, from a random stream
  of its own.
- **Differential run**: on the test server with the new generator, 0
  defects required; the 6σ run then continues on the compiler with this RFC.
- **Benchmarks**: unchanged (single-package); `bench/asm-check.sh`
  confirms it. A two-package benchmark is added to show that calls across
  modules of the crate inline as within one.

## Drawbacks

- Whole-program translation of large dependency graphs can be slow; the
  reachability walk keeps it to what is used.
- Error messages from inside dependencies may surprise a user; the chain of
  uses from `main` explains why each is reached.

## Rationale and alternatives

- **Hand-written Rust for standard-library packages** would be faster to
  start, but every function would be a second implementation of Go's
  semantics, each a chance to differ. Translating Go's own source gets the
  semantics by construction; only intrinsics, where gc itself does not run
  the source, are written by hand, and measured.
- **Separate crates per package** would mirror Go's packages more closely,
  but give nothing a program can observe, and cost compile time and
  cross-crate inlining.
- **Requiring every declaration of an imported package to be supported**
  would make most imports unusable. Reachability is what `go build`'s linker
  does too (dead-code elimination), so the program's behaviour is the same.

## Prior art

- TinyGo compiles the standard library from source with its own small
  runtime, replacing runtime-backed pieces; its experience shows how much of
  `std` is pure Go.
- gc's `cmd/compile/internal/ssagen` intrinsics table is the list this RFC's
  table follows.

## Unresolved questions

- The exact amd64 results of `Sub64`/`Sub32` for a borrow other than 0 or 1,
  and of the 32-bit forms, measured during implementation before the table is
  final.
- Whether the coverage measurement should report the chain-of-uses
  diagnostics per package or per function; decided with the tool change.

## Future possibilities

- Bridges for runtime-backed packages: `time` (clock), `sync`, `os` files.
- Caching translated packages between builds, keyed by their source hashes.
