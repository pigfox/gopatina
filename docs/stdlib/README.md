# The standard library against TinyGo

"Okay, so this is the minimum target of GoPatina's coverage. Anything below
TinyGo's coverage is unacceptable." (the project lead, 2026-11-23)

TinyGo's table of the Go standard library (TinyGo 0.44.0, Go 1.27.3,
linux/amd64, [tinygo.org/lang-support/stdlib](https://tinygo.org/lang-support/stdlib/),
read 2026-11-23, copied in [tinygo-0.44.0.txt](tinygo-0.44.0.txt)) is the
minimum target. The method, and how GoPatina's test differs from TinyGo's,
is in [COVERAGE.md](../COVERAGE.md#the-standard-library-against-tinygo).

## First measurement, 2026-11-23

`gopatina-coverage stdlib` on a workstation (Go 1.27.1, linux/amd64,
rustc 1.93.0 as GoPatina's crates pin it), at the commit of the pull
request that adds this file. That commit includes two fixes the
measurement needed: blank imports, and initializing package-level
variables that nothing uses. Raw results:
[results/2026-11-23-workstation.json](results/2026-11-23-workstation.json).

| Of 172 standard library packages | GoPatina | TinyGo |
| --- | --- | --- |
| Importable | **37** (22%) | 163 (95%) |
| Every function accepted (strictest reading of importable) | **1** (`structs`, which has none) | (TinyGo's table has no such column) |
| Passes its own tests | **0**: no patina test mode yet | 101 (59%) |

Below the minimum target on both counts.

- **Importable (37):** bufio, bytes, cmp, compress/lzw, container/heap,
  container/list, container/ring, crypto/subtle, encoding,
  encoding/ascii85, encoding/hex, errors, fmt, go/version, hash,
  hash/adler32, hash/fnv, hash/maphash, io, math, math/bits, math/cmplx,
  mime/quotedprintable, os, os/user, path, plugin, slices, strconv,
  strings, structs, text/tabwriter, time, unicode/utf16, unicode/utf8,
  unique, weak.
- **On TinyGo's list too:** 35 of those 37. GoPatina's 37 include
  hash/maphash and plugin, which TinyGo cannot import: their
  initialization translates, though most of what they do does not.
- **Importable is not usable.** Of the 10,638 functions in the 172
  packages, patina mode accepts 1,527 (14.4%). A blank import translates
  only a package's initialization.
- **Passing their own tests:** none. GoPatina has no patina test mode
  (`gopatina test -patina`), and patina mode does not accept the `testing`
  package.

## What keeps the other 135 out

The first thing each package's blank import is refused for. Fixing one may
show the next, so these count where work starts, not how much each fix
gains.

| First blocker | Packages |
| --- | --- |
| `io.Reader` (as a type, field or parameter) | 20 |
| A function type | 18 |
| An array type | 11 |
| An indexed slice literal element (`[]T{2: x}`) | 10 |
| A pointer to a type that is not the program's own, converted to an interface | 8 |
| A struct field of type `io.Writer` | 7 |
| `time.Time` | 7 |
| A package-level variable holding pointers in a struct or slice (`unicode`'s range tables, which the other 6 import) | 7 |
| An interface method with an error result (`io/fs.FS`, under `path/filepath` and others) | 6 |
| A map type | 5 |
| `internal/godebug.New` (provided package, not yet provided) | 4 |
| A generic constraint (an interface with a type set) | 4 |
| A pointer where RFC 0013 does not support one yet | 4 |
| A channel type | 3 |
| Converting untyped `nil` to a pointer type (`*go/scanner.Error`) | 3 |
| Others: 8 provided packages measured on their own and not yet provided (`sync`, `sync/atomic`, `sort`, `syscall`, `os/exec`, `os/signal`, `math/rand`, `math/rand/v2`), `unsafe`, `copy` (2), and one each of a changed array variable, interface methods with pointer or slice parameters, `internal/bytealg`, `slices.Grow`, an instantiated generic function and an assembly function | 18 |

Until the loader fix (2026-11-23, after the first measurement), 40 of these
packages were first refused by an internal type-checking error instead: a
package from export data brought its own copy of a package that was also
read from source (`os.DirEntry` against `fs.DirEntry`, `iter.Seq` in
`slices.Sorted`). Each now stops at the first feature it needs, as above;
the counts of importable packages and accepted functions did not change.

## The order of work

To raise both counts fastest, after RFC 0016's owned pointers (step 1b):

1. **The loader's type identity: done** (2026-11-23). A package that
   imports a translated package, directly or not, is type-checked from
   source, so `io/fs` and `iter` arrive once. It changed no count, but it
   turned 40 internal errors into the features each package needs, and
   made `io.Reader` the largest single first blocker.
2. **Function types and function values** (18 first blockers). They need
   a design for how closures own what they capture, so an RFC comes first.
3. **Arrays and indexed slice literals** (21 together). Arrays map to Rust
   arrays; small and mechanical.
4. **`io.Reader` and `io.Writer` fields** (27), then the provided
   `internal/godebug` and `time.Time` (11): RFC 0011's provided-package work.
5. **A patina test mode.** `gopatina test -patina`: the `testing` package
   provided in Rust, `_test.go` files translated with the package, and a
   generated test main. Until it exists the second count stays at 0.
   Packages that become importable through steps 1 to 4 are where tests
   can start to pass.

Each step is measured again with `gopatina-coverage stdlib`, and the
results are published here next to TinyGo's, whatever they show. The
real-world function share ([COVERAGE.md](../COVERAGE.md)) stays the other
number.
