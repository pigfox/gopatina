# RFC 0011: Provided packages: where translation stops

- **Status:** Draft
- **Author(s):** GoPatina contributors
- **Created:** 2026-11-22
- **Discussion:** the pull request that adds this file
- **Supersedes / superseded by:** amends RFC 0009 ("Which packages")

## Summary

Define the boundary between the standard library GoPatina **translates
from source** (RFC 0009) and the standard library it **provides**: packages
implemented in Rust, with Go's observable semantics, whose API a program's
calls reach and whose implementation is never translated. Translation stops
at a provided package's API. The provided packages are the ones whose Go
implementation rests on the runtime, assembly, `unsafe` or `reflect`; every
other package stays translated. The coverage measurement applies the same
boundary.

## Motivation

RFC 0009 translates every package a program reaches, down to whatever its
functions reach. The coverage measurement at 2410684 (cov10, 2026-11-22)
shows where that leads: the functions without a Go body that the most
rejected functions reach are the Go runtime's own:

| Function without a body | Rejected functions reaching it | Would pass with only it |
| --- | --- | --- |
| `sync/atomic.AddInt32` | 10,741 | 2 |
| `sync/atomic.CompareAndSwapInt32` | 10,393 | 2 |
| `internal/sync` semaphores, spinning, `nanotime`, `throw` (7 functions) | about 10,362 each | 1 |
| `internal/bytealg.IndexByteString` | 8,067 | 5 |
| `internal/runtime/syscall/linux.Syscall6` | 7,667 | 8 |
| `internal/runtime/atomic` (about 25 functions) | about 7,385 each | 2–5 |
| `internal/chacha8rand.block` | 7,393 | 1 |
| `runtime` (`nanotime1`, `usleep`, `osyield`, ...), `internal/abi`, `internal/runtime/sys` | about 7,383 each | 1 |

A `sync.Mutex` leads to `internal/sync` and its semaphores; anything that
allocates, reads the clock or draws a random number leads into `runtime`.
Each such function alone unlocks almost nothing, because behind it stand
twenty more: per-function intrinsics would mean reimplementing the runtime
one assembly routine at a time. And 16,684 functions are rejected for
reaching `reflect` or `unsafe` inside a dependency, which is the same
boundary seen from the other side: `strings.Builder` uses `unsafe` in package
`strings` itself, `errors` and `sort` use `internal/reflectlite`.

What a Go program can observe of these packages is their API, not their
implementation. Rust has its own runtime, clock, atomics and allocator;
GoPatina should provide Go's API on top of them, exactly where Go's
behaviour is observable, and translate nothing below it.

## Guide-level explanation

```go
package main

import (
	"fmt"
	"strings"
	"sync"
)

var mu sync.Mutex

func main() {
	var b strings.Builder
	mu.Lock()
	b.WriteString("go")
	mu.Unlock()
	fmt.Println(b.String(), strings.ToUpper(b.String()))
}
```

`fmt`, `strings` and `sync` are provided packages: `strings.Builder`,
`strings.ToUpper` and `sync.Mutex` are Rust types and functions in the
crate's runtime module, written to Go's documented behaviour and tested
against Go. Nothing of Go's `strings`, `sync` or `runtime` is translated.

## Reference-level explanation

### The rule

- A package is **provided** when it is in the list below. Its supported API
  is listed with it, grows feature by feature, and is implemented in Rust
  (the runtime module `rt`, and modules beside it).
- A call, type, constant or variable of a provided package is accepted when
  it is in the supported API, and rejected otherwise with its own message
  ("`sync.Cond` is not provided yet").
- Translation (RFC 0009) never enters a provided package, and the
  reachability walk stops at its API: nothing a provided package uses
  internally is reached.
- Every other package, of the standard library or not, is translated.

### The provided packages

Each is listed with why it is provided rather than translated.

| Package | Why provided | First API (in RFCs to come) |
| --- | --- | --- |
| `fmt`, `strings`, `bytes`, `errors`, `io`, `os` | already bridged; their implementations use `unsafe`, `reflect`/`reflectlite`, `sync` or the runtime | the phase 1 functions, then `strings.Builder`, `bytes.Buffer`, `strings.Split`, `ToUpper`, ... by measured use |
| `sync`, `sync/atomic` | the runtime's semaphores and the atomics are assembly (the top two rows above) | `Mutex`, `RWMutex`, `Once`, `WaitGroup`, the atomic functions and types, with RFC 0002's goroutines |
| `runtime` | the runtime itself | only what a program observes: `GOMAXPROCS`, `NumCPU`, `NumGoroutine`, `Gosched`, `GC`, `KeepAlive`, runtime errors through panics |
| `time` | the clock, timers and sleeping are the runtime's | `Duration` and its methods, `Now`, `Since`, `Sleep`, `Time` formatting; timers with RFC 0002 |
| `math/rand`, `math/rand/v2` | `internal/chacha8rand` is assembly; the sources' state lives in the runtime | the seeded sources (`rand.New(rand.NewSource(s))`, `rand.NewPCG`, `rand.NewChaCha8`) give Go's exact sequences; the unseeded top-level functions give random values, as in Go |
| `sort` | `sort.Slice` uses `internal/reflectlite` | `Ints`, `Strings`, `Search`, and `Slice` once closures are supported |
| `syscall`, `os/signal`, `os/exec` | system calls | as programs need them |
| `internal/bytealg` | assembly, reached by translated packages (`strconv`, `unicode/utf8`'s users) | as intrinsics: `IndexByte`, `IndexByteString`, `Count`, `Equal`, `Compare`, `Index` |
| `math` (its assembly functions) | `archSqrt`, `archFloor`, ... are assembly | as intrinsics with IEEE-exact Rust equivalents; the rest of `math` is translated |
| `runtime`'s internal packages (`internal/runtime/*`, `internal/abi`, `internal/sync`, `internal/chacha8rand`, `internal/cpu`, `internal/goarch`, `internal/godebug`, `internal/race`, `internal/poll`, `internal/syscall/*`, `internal/reflectlite`) | below the provided packages' APIs: never reached | none |

Everything else in the standard library (`strconv`, `unicode`,
`unicode/utf8`, `math/bits`, `slices`, `maps`, `container/*`, `encoding/*`,
...) is translated, using the provided packages and the intrinsics where its
source calls them.

### Semantics

A provided package's behaviour is Go's documented behaviour, and where the
documentation leaves a choice, gc's (as RFC 0008 and RFC 0010 do). Two kinds
of behaviour are tested differently:

- **Deterministic** (`strings`, `strconv` through translation, `sort`, the
  seeded random sources, `time.Duration` formatting, a `Mutex` locked twice
  by one goroutine, which Go reports as a deadlock): differentially against
  Go, as everything else.
- **Nondeterministic** (`time.Now`, unseeded random values, scheduling):
  tested by properties (monotonic `Since`, values in range, the documented
  distribution of `rand.IntN`), not by comparing outputs.

### The coverage measurement

The measurement applies the same boundary: a call into a provided package is
decided by that package's supported API, and nothing below the API is
reached. A use outside the supported API is classified by its package
("provided package: sync"), so the roadmap shows which provided package
unlocks how much.

## Compatibility

Default mode is unchanged. In patina mode, a program that reached a
provided package's internals through translation was rejected there; it is
now rejected at the API it calls, with a message naming that API.

## Testing

- For each provided API: golden programs differentially against Go, the
  generator once the API is in it, and property tests for the
  nondeterministic parts.
- Rejection fixtures: an API of a provided package not supported yet.

## Drawbacks

- Each provided package is a second implementation of Go behaviour, which
  is where differences can hide. The differential tests, and gc as the
  oracle, are how they are found; the boundary keeps the surface to the
  packages where translation is not possible anyway.

## Rationale and alternatives

- **Translating the runtime** (RFC 0009 taken all the way down): the
  measurement shows it needs hundreds of assembly routines, each unlocking
  almost nothing alone, and a garbage-collected runtime GoPatina exists to
  avoid.
- **Bridging only what is used** (per call, as phase 1 did): the same
  boundary, without a rule for what is translated; this RFC makes the rule.

## Prior art

- TinyGo replaces `runtime`, `sync`, `reflect` and parts of `os` and `time`
  with its own implementations and compiles the rest of the standard library
  from source: the same boundary, drawn for a different runtime.
- gccgo shares Go's library source but provides its own runtime.

## Unresolved questions

- Whether `os`'s file API is provided now or with its own RFC (it needs
  `io.Reader`, an interface).
- The exact first API of each provided package, set by the measurement once
  the boundary applies.

## Future possibilities

- `net`, `net/http`: provided on Rust's networking, with RFC 0002.
- `encoding/json`: translated if its `reflect` use can be replaced, else
  provided.
