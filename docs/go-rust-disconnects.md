# Where Go and Rust do not line up

This document lists every known place where Go's semantics and Rust's do
not match, and what GoPatina does or will do about each. It is the working
list behind the goal of patina mode: **the whole Go language and the whole
Go standard library, with Go's observable behaviour**. Nothing here is
implemented unless [compat.md](compat.md) says so; this page is about the
gaps, not about what works.

## Scope: which mode this is about

- **Default mode has none of these problems.** `gopatina build`, `run`,
  `test` and every other command without `-patina` or `-emit-rust` run the
  installed `go` command with the same arguments, streams and exit status.
  The program is compiled by Go, linked with Go's runtime and garbage
  collector, and is 100% compatible with Go by construction, because it *is*
  Go.
- **Patina mode (`-patina`, `-emit-rust`, `-patina=check`) is not
  compatible with Go yet.** It accepts a defined subset
  ([patina-subset.md](patina-subset.md)) and rejects everything else with a
  Go-style error naming the construct. Its status, feature by feature, is in
  [compat.md](compat.md); a feature counts as supported only when it has
  golden, differential, rejection and fuzz tests. Until compat.md shows a
  row as supported, assume it is not.

## The rule for every entry

When patina mode cannot match Go exactly, it **rejects the program with a
precise error**; it never translates approximately. Each entry below gives:

- **Go**: the exact behaviour a Go program can observe.
- **Rust**: why the straightforward Rust translation differs.
- **Options** considered.
- **Recommendation**: what GoPatina should do.
- **Blocks 100%?** *No* (solvable, the work is known), *Partly* (most
  programs can be matched; some observable behaviour cannot, and those
  programs are rejected), or *Yes* (cannot be matched without Go's runtime;
  such programs stay default-mode only).

The recommendations here are proposals. Each large one (reflection,
interfaces, the standard library strategy, networking) gets its own RFC
before it is implemented.

## Summary

| # | Area | Blocks 100%? | Recommendation in one line |
| --- | --- | --- | --- |
| 1 | [Standard library strategy](#1-standard-library-strategy) | No | Translate Go's own standard library source through patina mode; native Rust only under it |
| 2 | [reflect](#2-reflect) | Partly | Generated type descriptors (a runtime type table) for every type, giving reflect its data without a GC |
| 3 | [fmt verbs that use reflection](#3-fmt-v-v-v-t-p) | Partly | Generated per-type formatting from the type table; `%p` and pointer values printed only where Go's output is stable |
| 4 | [encoding/json, xml, gob; text/template, html/template](#4-encodingjson-xml-gob-and-texttemplate-htmltemplate) | No | Built on the reflect of entry 2; struct tags preserved in the type table |
| 5 | [unsafe](#5-unsafe) | Partly | `unsafe.Sizeof`/`Alignof`/`Offsetof` with Go's layout; `unsafe.Pointer` arithmetic rejected |
| 6 | [cgo](#6-cgo) | Partly | Rust FFI to the same C code through a generated `extern "C"` block and the `cc` crate |
| 7 | [runtime](#7-runtime) | Partly | Each function mapped, emulated or rejected by name (table below) |
| 8 | [plugin](#8-plugin) | Yes | Rejected |
| 9 | [syscall, os, os/exec, os/signal](#9-syscall-os-osexec-ossignal) | Partly | Port Go's os layer onto Rust std and `libc`; `syscall` per platform, as Go does |
| 10 | [net, net/http](#10-net-and-nethttp) | No | Translate Go's net/http source onto a Tokio network poller; not hyper |
| 11 | [crypto](#11-crypto) | No | Translate Go's crypto source (constant-time code kept); RustCrypto or ring only behind exact differential tests |
| 12 | [time](#12-time) | No | Go's tz database lookup and monotonic clock, translated; timers on Tokio |
| 13 | [Map iteration order](#13-map-iteration-order) | No | Randomised per iteration, as Go does; tests may not depend on order |
| 14 | [select fairness](#14-select-fairness) | No | GoPatina's own select: uniform random choice among ready cases |
| 15 | [Scheduling and preemption](#15-goroutine-scheduling-and-preemption) | Partly | Yield points inserted in loops; `spawn_blocking` for detected CPU-bound goroutines |
| 16 | [Memory model and data races](#16-the-go-memory-model-and-data-races) | No | Shared variables become `Arc<Mutex<T>>` per access; atomics map to `SeqCst` |
| 17 | [Goroutine counts and stack growth](#17-unbounded-goroutine-counts-and-stack-growth) | Partly | Stackless tasks for goroutines; deep recursion on a grown stack or rejected |
| 18 | [nil](#18-nil-and-option) | No | Each nil-able type gets an explicit nil state with Go's panics |
| 19 | [Zero values](#19-zero-values) | No | Every type implements Go's zero value (`Default`) |
| 20 | [Interface comparison and panics](#20-interface-comparability-and-panics) | No | Dynamic equality with Go's run-time panic for uncomparable types |
| 21 | [Integer and float semantics and formatting](#21-integer-and-float-semantics-and-formatting) | No | Wrapping arithmetic, Go's panics, Go's float formatting (done for phase 1 types) |
| 22 | [Strings and []byte](#22-strings-and-byte) | No | Go strings as byte strings, not Rust `String` |
| 23 | [Slices sharing backing arrays](#23-slices-sharing-backing-arrays) | No | A GoPatina slice type (shared buffer, offset, length, capacity) |
| 24 | [Cyclic data and pointers](#24-cyclic-data-and-pointers) | Partly | `Rc`/`Arc` with cycle collection, or reference-counted arenas |
| 25 | [Closures and loop variables](#25-closures-capturing-loop-variables) | No | Go 1.22 per-iteration variables; captured-and-changed variables become shared cells |
| 26 | [defer, panic, recover](#26-defer-panic-and-recover-including-across-async-code) | No | Defer stacks per function call; panics as unwinding; recover catches them |
| 27 | [embed](#27-embed) | No | `include_bytes!` and a generated `embed.FS` |
| 28 | [Directives and build tags](#28-gogenerate-build-tags-and-compiler-directives) | Partly | Build tags and `go:generate` honoured; `go:linkname` rejected |
| 29 | [testing and go test](#29-the-testing-package-and-go-test) | No | `gopatina test -patina` runs Go's test functions in the Rust build |
| 30 | [Package initialisation](#30-package-initialisation-and-package-level-variables) | No | Go's init order, computed at compile time |
| 31 | [Generics](#31-generics) | No | Monomorphisation, with Go's rules for type sets |
| 32 | [Panics, fatal errors and exit status](#32-panic-messages-fatal-errors-and-exit-status) | Partly | Same first line and exit status; goroutine traces not reproduced |
| 33 | [Algorithms whose output is observable](#33-algorithms-whose-output-is-observable) | No | Port Go's exact algorithms (sort, math/rand, hashing, strconv) |
| 34 | [Unicode tables](#34-unicode-tables) | No | Generate the tables from the Go release in use |

## 1. Standard library strategy

- **Go**: about 180 importable packages (`go list std`, without `internal`
  and `vendor`), most written in Go, some with assembly and runtime hooks.
- **Rust**: Rust's standard library has different APIs and, more
  importantly, different observable behaviour (formatting, sorting,
  parsing, error texts).
- **Options**:
  1. Map each Go function to the nearest Rust std or crate function. Fast
     to start, but every difference in output, error text or edge case is a
     compatibility bug, and there are thousands.
  2. Hand-write each Go package in Rust. Exact only as far as the porter is
     careful; large, and drifts from Go with every release.
  3. **Translate Go's own standard library source with patina mode**, so
     `strings`, `strconv`, `sort`, `fmt`, `encoding/*`, `net/http`, ... are
     Go's code, compiled through GoPatina. Only the layer Go implements in
     the runtime or assembly (memory, goroutines, the network poller,
     syscalls, `reflect`'s type data) is native Rust.
- **Recommendation**: option 3. It is the only way to match Go's behaviour
  in every edge case and to follow each Go release. It also makes the
  standard library the largest test of the compiler: every package that
  translates and passes Go's own tests (`gopatina test -patina std`) is
  supported, and compat.md can be generated from that result. Option 1 is
  acceptable only for small leaf functions with exact differential tests
  (phase 1 does this for `strings.TrimSpace` and a few others).
- **Blocks 100%?** No, but it requires most of the language first:
  interfaces, generics, `reflect`, `unsafe.Sizeof`, and the runtime layer.

## 2. reflect

- **Go**: `reflect` inspects and changes any value at run time: its type,
  fields, struct tags, methods; it can call functions and build new types
  (`reflect.StructOf`, `FuncOf`, `MakeFunc`).
- **Rust**: no run-time type information beyond `std::any::TypeId`; types
  are erased after compilation.
- **Options**: (a) reject `reflect`; (b) generate, at compile time, a type
  descriptor for every type in the program (kind, size, fields with names,
  offsets and tags, method table, element types), which is what Go's
  compiler itself emits for its runtime; (c) a derive-macro crate such as
  `bevy_reflect`.
- **Recommendation**: (b). GoPatina sees the whole program, so it can emit
  exactly the descriptors Go would, and `reflect` can be translated from
  Go's source on top of them. `reflect.StructOf`, `FuncOf`, `ArrayOf` and
  `MakeFunc` build types or code at run time; they need a generic
  representation and an interpreter-like call path, and are rejected until
  then. Reflection on unexported fields follows Go's rules (readable, not
  settable).
- **Blocks 100%?** Partly: the run-time type constructors are the hard part.

## 3. fmt %v, %+v, %#v, %T, %p

- **Go**: `%v` prints any value, recursively, using its `String()` or
  `Error()` method when it has one; `%+v` adds field names, `%#v` prints Go
  syntax, `%T` the type name, `%p` and `%v` of a pointer print an address
  (`0xc000012345`).
- **Rust**: `Display` and `Debug` are per type and do not know Go's rules;
  addresses differ from run to run in both languages.
- **Options**: per-type generated formatting; or fmt translated from Go's
  source over the reflect of entry 2.
- **Recommendation**: fmt translated from Go's source, over the type table,
  so every verb, flag and `Stringer`/`Formatter`/`GoStringer` rule is Go's.
  Phase 1 has hand-written formatting for basic types and slices, tested
  differentially. Addresses (`%p`, a pointer's `%v`) cannot be equal to Go's
  and are not equal between two Go runs either, so differential tests treat
  them as opaque; the format (`0x` and hex digits) is kept.
- **Blocks 100%?** Partly, only in that printed addresses are never the
  same, which no Go program can rely on.

## 4. encoding/json, xml, gob, and text/template, html/template

- **Go**: driven by `reflect` and struct tags; templates call methods by
  name at run time.
- **Rust**: serde derives at compile time and has different defaults
  (field naming, `omitempty`, map key order, HTML escaping, number
  formatting, error texts).
- **Options**: map to serde (fast, but different output); translate Go's
  packages over entry 2.
- **Recommendation**: translate Go's packages. Output must be byte-for-byte
  Go's, including `json.Marshal`'s sorted map keys, HTML-safe escaping and
  error messages. A later optimisation can generate specialised encoders
  for known types, as long as differential tests keep the bytes identical.
- **Blocks 100%?** No, once reflect exists.

## 5. unsafe

- **Go**: `unsafe.Pointer` converts between any pointer types and to
  `uintptr`; `unsafe.Sizeof`, `Alignof`, `Offsetof`, `unsafe.Slice`,
  `unsafe.String`, `unsafe.StringData`, `unsafe.SliceData`, `unsafe.Add`.
- **Rust**: raw pointers exist, but GoPatina's values are laid out by
  rustc, and Go's moving-free but escape-analysed heap is not there.
- **Options**: reject `unsafe`; support the size and layout functions with
  Go's numbers (GoPatina controls layout with `#[repr(C)]`-style
  generated layouts); support pointer conversions that preserve type
  identity (`[]byte` to `string` without copying, as `strings.Builder`
  does) as GoPatina primitives.
- **Recommendation**: support `Sizeof`, `Alignof`, `Offsetof` with Go's
  values for each target; support the conversion patterns the standard
  library uses (recognised and replaced by safe primitives); reject
  arbitrary pointer arithmetic and `uintptr` round trips by name.
- **Blocks 100%?** Partly: programs doing raw pointer arithmetic stay in
  default mode.

## 6. cgo

- **Go**: `import "C"` compiles C code with the package and calls it, with
  rules about passing Go pointers to C.
- **Rust**: has its own FFI; no C preamble in source files.
- **Options**: reject; or generate `extern "C"` declarations from the
  preamble (as `bindgen` does) and compile the C code with the `cc` crate.
- **Recommendation**: generate the FFI layer; keep Go's pointer-passing
  rules as compile-time errors where they can be checked. RFC 0001 already
  proposes Rust crate interop as the preferred way to reach native code.
- **Blocks 100%?** Partly: C code that calls back into Go with `//export`
  and expects Go's runtime is the hard case.

## 7. runtime

No garbage collector and Tokio instead of Go's scheduler. Each function:

| Function | Go | Patina mode |
| --- | --- | --- |
| `runtime.GC`, `debug.FreeOSMemory` | runs a collection | no-op is observably the same except through memory statistics; supported as a no-op, documented |
| `runtime.SetFinalizer` | runs a function some time after the object is unreachable, maybe never | with reference counting, it can run when the last reference is dropped, which is one of the schedules Go allows; supported only for values whose ownership GoPatina tracks, else rejected |
| `runtime.KeepAlive` | keeps an object reachable until this point | no-op (Rust drops at end of scope or at last use the checker decides) |
| `runtime.GOMAXPROCS` | sets the number of threads running Go code | sets Tokio's worker count before the runtime starts; changing it later is rejected |
| `runtime.Gosched` | yields | `tokio::task::yield_now` |
| `runtime.NumGoroutine` | live goroutines | exact, from GoPatina's scheduler bookkeeping |
| `runtime.Goexit` | ends the goroutine after running its defers | a special unwind that defers see and recover does not stop |
| `runtime.LockOSThread` | pins the goroutine to its thread | the goroutine runs on a dedicated thread |
| `runtime.Caller`, `Callers`, `Stack`, `debug.Stack`, `debug.PrintStack` | file, line and function names of the stack | a table mapping Rust frames back to Go file and line, generated with the crate; function names in Go form; exact text only where Go's is stable |
| `runtime.MemStats`, `runtime/metrics`, `debug.SetGCPercent`, `SetMemoryLimit` | GC statistics and tuning | rejected by name: there is no GC; programs that only log them could get a documented stub later |
| `runtime/pprof`, `runtime/trace` | profiles in Go's formats | CPU profiles possible through a sampling profiler in pprof format; heap profiles rejected; trace rejected |
| `debug.ReadBuildInfo` | module and build settings | generated at compile time with the same data |

- **Blocks 100%?** Partly: GC statistics and heap profiles cannot exist
  without a GC.

## 8. plugin

- **Go**: loads Go shared objects built with `-buildmode=plugin`.
- **Rust**: no stable ABI for this.
- **Recommendation**: rejected; Linux/macOS-only in Go, rarely used.
- **Blocks 100%?** Yes, for `plugin` only.

## 9. syscall, os, os/exec, os/signal

- **Go**: `os` is built on `syscall`, with Go's error values
  (`*os.PathError` with Go's message texts), file modes, `os.Exit` not
  running defers; `os/exec` with Go's `LookPath` rules; `os/signal`
  delivers signals on channels.
- **Rust**: `std::fs`, `std::process` have other error texts and
  semantics (for example `Command` searches `PATH` differently on Windows).
- **Options**: map to Rust std; translate Go's `os` over a native syscall
  layer.
- **Recommendation**: translate Go's `os`, `os/exec`, `io/fs`,
  `path/filepath` source over a small native layer (`libc` on Unix,
  `windows-sys` on Windows) that exposes Go's `syscall` API, so error
  messages and edge cases are Go's. `os/signal` maps to Tokio's signal
  handling feeding a Go channel. `syscall` itself is per-platform in Go as
  well; supported per target, rejected by name elsewhere.
- **Blocks 100%?** Partly: `syscall.ForkExec` details and raw `Syscall`
  numbers are platform ABI, possible but large.

## 10. net and net/http

- **Go**: a blocking API on goroutines, an integrated network poller,
  Go's DNS resolver (pure Go or cgo), `net/http` with HTTP/1.1 and HTTP/2
  and its exact header handling, timeouts and error texts.
- **Rust**: async I/O (Tokio); hyper has different behaviour (header
  normalisation, connection reuse, error kinds).
- **Options**: wrap hyper and reqwest; or translate Go's `net` and
  `net/http` source over a native poller built on Tokio's reactor (the
  part Go's runtime provides).
- **Recommendation**: translate Go's source onto a Tokio-based poller.
  Blocking calls are async under the hood, which the automatic async
  colouring of RFC 0002 already handles. hyper would be faster to reach
  but would never be Go's `net/http`.
- **Blocks 100%?** No, but it depends on interfaces, reflect-free parts of
  the library, `crypto/tls` (entry 11) and the poller.

## 11. crypto

- **Go**: `crypto/*` in Go and assembly, constant-time where it matters,
  `crypto/tls` with Go's own TLS stack, a FIPS 140-3 mode
  (`GOFIPS140`, `crypto/fips140`).
- **Rust**: ring, rustls, RustCrypto: different APIs, different error
  behaviour; their constant-time properties come from their own code.
- **Options**: map to RustCrypto/ring/rustls; translate Go's source.
- **Recommendation**: translate Go's source, keeping its constant-time code
  paths (the translation must not introduce data-dependent branches: the
  emitter keeps Go's arithmetic as written, and constant-time functions get
  tests that compare instruction traces or use known timing harnesses).
  Assembly fast paths are replaced by the Go fallback code Go itself ships,
  then by Rust intrinsics where they give identical results. FIPS mode is
  rejected: Go's validated module is a specific binary; a translated one
  would not carry the validation.
- **Blocks 100%?** No for behaviour; FIPS validation cannot carry over.

## 12. time

- **Go**: `time.Now` carries a monotonic clock reading, used by `Sub` and
  `Since` and stripped by `Round(0)`; time zones from the system or
  `time/tzdata`; Go's layout-based formatting (`2006-01-02`); timers and
  tickers on the runtime.
- **Rust**: `Instant` and `SystemTime` are separate types; no time zones in
  std; chrono's formatting is different.
- **Recommendation**: translate Go's `time` package (formatting, parsing,
  zones, `LoadLocation` with Go's search order: `ZONEINFO`, system files,
  embedded tzdata) over a native clock layer giving wall and monotonic
  readings. `Sleep`, `After`, `Timer`, `Ticker` map to Tokio timers; a
  goroutine sleeping is not "asleep" for deadlock detection, as in Go.
- **Blocks 100%?** No.

## 13. Map iteration order

- **Go**: iteration order is unspecified and deliberately randomised on
  every `range`; printing a map with fmt sorts the keys.
- **Rust**: `HashMap` order is arbitrary but fixed for a given map;
  `BTreeMap` is sorted.
- **Recommendation**: a GoPatina map type that starts each iteration at a
  random position, as Go's does, so programs that accidentally depend on
  order fail in patina mode as they would in Go. fmt prints sorted keys
  (Go's rule). Tests, including differential tests, must not depend on
  iteration order; golden programs that range over maps sort the keys or
  aggregate order-independently.
- **Blocks 100%?** No.

## 14. select fairness

- **Go**: when several cases are ready, one is chosen uniformly at random.
- **Rust**: `tokio::select!` is random by default but polls futures, which
  can consume a value from a channel in a case that is not chosen.
- **Recommendation**: GoPatina's own select (planned with goroutines): all cases
  are checked under one lock, in a random permutation, and only the chosen
  operation takes effect. Programs and tests may not depend on the choice.
- **Blocks 100%?** No.

## 15. Goroutine scheduling and preemption

- **Go**: goroutines are preempted asynchronously (since Go 1.14), so a
  tight loop cannot starve others.
- **Rust**: Tokio tasks are cooperative; a task that never awaits holds
  its worker thread.
- **Options**: insert yield points at loop back-edges in async code; run
  goroutines that never block on `spawn_blocking` threads; one OS thread
  per goroutine (correct but expensive).
- **Recommendation**: a cheap budget check at loop back-edges of goroutine
  code (a counter decremented per iteration, yielding when it reaches
  zero), plus `spawn_blocking` for goroutines detected as CPU-bound, both
  reported by `-explain`. The planned goroutine runtime starts by yielding
  in `select` with `default`, the common busy-polling pattern.
- **Blocks 100%?** Partly: until back-edge yields exist, a program whose
  progress needs preemption (more spinning goroutines than worker threads)
  can hang where Go would not. Such programs are rare and usually racy.

## 16. The Go memory model and data races

- **Go**: race-free programs are sequentially consistent; racy programs on
  word-sized values see some written value, and races on strings,
  interfaces and slices can corrupt memory. `sync/atomic` operations are
  sequentially consistent.
- **Rust**: data races are compile errors; shared mutation needs
  `Mutex`, `RwLock` or atomics with an explicit ordering.
- **Recommendation**: a variable that is shared with a goroutine and
  changed becomes `Arc<Mutex<T>>`, locked for each access (planned with
  goroutines, reported by `-explain`). A race-free Go program behaves exactly
  the same; a racy one gets one of the behaviours Go allows, never memory
  corruption. `sync/atomic` maps to Rust atomics with `SeqCst`, which is
  Go's guarantee. A strict mode (RFC 0001) can turn the automatic fix into
  an error.
- **Blocks 100%?** No.

## 17. Unbounded goroutine counts and stack growth

- **Go**: goroutines start with small stacks that grow; millions of
  goroutines are normal; deep recursion grows the stack up to 1 GB, then
  `fatal error: stack overflow`.
- **Rust**: async tasks are stackless state machines (cheap, so goroutine
  counts are fine); but deep recursion runs on the thread's fixed stack (8
  MB main, 2 MB Tokio workers by default) and aborts with a different
  message.
- **Options**: larger thread stacks; the `stacker` crate's segmented
  growth at recursion points; Go's limit enforced by a depth counter.
- **Recommendation**: grow the stack at recursive calls (`stacker`-style)
  and print Go's `fatal error: stack overflow` with exit status 2 at Go's
  limit. Recursive async functions are boxed already, so their depth lives
  on the heap.
- **Blocks 100%?** Partly, until stack growth is implemented: deep
  recursion that Go survives would overflow.

## 18. nil and Option

- **Go**: pointers, slices, maps, channels, functions and interfaces can
  be nil, each with its own behaviour: reading a nil map gives zero values,
  writing panics; a nil slice has length 0 and can be appended to; a nil
  channel blocks forever; calling a nil function or dereferencing a nil
  pointer panics with `invalid memory address or nil pointer dereference`;
  an interface holding a nil pointer is not nil.
- **Rust**: `Option<T>` is explicit; no implicit null.
- **Recommendation**: every nil-able Go type gets an explicit nil state in
  its GoPatina representation (`Chan` already has one), with Go's
  operations and panic messages. The borrow checker removes the nil state
  where it can prove a value is never nil. The typed-nil-in-interface case
  is represented exactly (an interface value holds a type and a value, and
  the value can be a nil pointer).
- **Blocks 100%?** No.

## 19. Zero values

- **Go**: every type has a zero value; variables, struct fields, map
  misses, closed-channel receives and `new(T)` produce it.
- **Rust**: no implicit initialisation.
- **Recommendation**: every emitted type implements `Default` as Go's zero
  value (done for phase 1 types; `Chan`'s default is a nil channel).
- **Blocks 100%?** No.

## 20. Interface comparability and panics

- **Go**: interface values compare equal if their dynamic types are
  identical and their values are equal; comparing two interfaces holding
  the same uncomparable type (a slice, map or function) panics at run time
  (`runtime error: comparing uncomparable type []int`), including as a map
  key.
- **Rust**: `dyn Trait` has no equality; `PartialEq` is static.
- **Recommendation**: interface values carry their type descriptor (entry
  2) and a comparison function or "uncomparable" marker, reproducing Go's
  result and panic text.
- **Blocks 100%?** No.

## 21. Integer and float semantics and formatting

- **Go**: integers wrap on overflow; division by zero panics
  (`runtime error: integer divide by zero`); `MinInt64 / -1` wraps; shifts
  by a count >= the width give 0 (or -1 for a negative signed value);
  shifting by a negative count panics; float-to-integer conversion of an
  out-of-range value is implementation-defined; NaN keys in maps are all
  distinct; `-0.0` prints as `-0`; `%v` of a float uses the shortest
  representation that reads back the same, with Go's exponent rules.
- **Rust**: overflow panics in debug builds; shifts by >= width are
  errors; `as` saturates; float `Display` differs from Go's.
- **Recommendation**: wrapping arithmetic and Go's panics (done in phase 1
  for `int`, `int64`, `int32`, `uint8`, `float64`); shifts by a variable
  count emitted with Go's rules (phase 1 accepts only constant counts so
  far); float-to-integer conversions follow what Go's compiler does on the
  same target (amd64 and arm64 differ in Go; GoPatina should match the
  target). Go's float formatting is in the runtime (`rt::Float`); full
  `strconv` will be translated from Go.
- **Blocks 100%?** No.

## 22. Strings and []byte

- **Go**: a string is an immutable sequence of bytes, not necessarily
  valid UTF-8; indexing gives bytes; `range` decodes UTF-8 and yields
  U+FFFD for invalid bytes; `string(b)` and `[]byte(s)` copy (the compiler
  avoids some copies when it can prove nothing changes).
- **Rust**: `String` must be valid UTF-8.
- **Recommendation**: a GoPatina string type that holds bytes (immutable,
  cheaply shared: an `Arc<[u8]>` or a small-string type), with Go's
  operations; `std::str` only where valid UTF-8 is proved. Phase 1 uses
  `String` and therefore rejects string values that may not be valid
  UTF-8 (byte-slice conversion, invalid literals); that restriction goes
  away with the byte-string type.
- **Blocks 100%?** No.

## 23. Slices sharing backing arrays

- **Go**: a slice is (pointer, length, capacity) into a shared array;
  `s[i:j]` and `s[i:j:k]` alias it; `append` writes in place when capacity
  allows (so it can overwrite another slice's elements) and reallocates
  otherwise, with Go's growth formula deciding which; `copy` handles
  overlap.
- **Rust**: `Vec<T>` owns its buffer; `&[T]` borrows are statically
  checked and cannot outlive or alias mutably.
- **Recommendation**: phase 1 uses `Vec<T>` and borrows where the borrow
  checker proves no aliasing is observable, and rejects the rest. Full
  support needs a GoPatina slice type: a shared buffer (`Rc`/`Arc` of a
  `RefCell`/lock or an unsafe-but-encapsulated cell), offset, length and
  capacity, with Go's growth formula copied exactly so `cap` and aliasing
  after `append` match. The borrow checker keeps using `Vec`/`&[T]` where
  it can prove the difference is not observable, so performance stays
  Rust's in the common case.
- **Blocks 100%?** No.

## 24. Cyclic data and pointers

- **Go**: pointers can form any graph, including cycles; the GC frees
  unreachable cycles.
- **Rust**: ownership is a tree; `Rc` cycles leak.
- **Options**: `Rc`/`Arc` plus `Weak` where a back edge is detectable;
  reference counting with a cycle collector (as CPython does); arena
  allocation per data structure; reject.
- **Recommendation**: the automatic fixes of RFC 0001 (`Rc`/`Arc` for
  shared ownership, `Weak` for detected parent pointers), with a cycle
  collector as the fallback for general graphs. Leaks are not observable
  through Go semantics, except through memory use and finalizers (entry 7).
- **Blocks 100%?** Partly: memory use of long-running programs with
  cyclic garbage depends on the collector being there.

## 25. Closures capturing loop variables

- **Go**: closures capture variables, not values. Since Go 1.22, each
  iteration of a `for` loop has its own loop variable; before, one
  variable was shared by all iterations (the module's `go` line decides).
- **Rust**: closures capture by reference or by move, checked statically.
- **Recommendation**: a captured variable that nothing changes afterwards
  is copied into the closure; a captured variable that is changed by the
  closure or after the capture becomes a shared cell (`Rc<RefCell<T>>`, or
  `Arc<Mutex<T>>` across goroutines). Loop variables follow the module's
  language version: per-iteration for go 1.22 and later, one shared cell
  before. Goroutine closures are the first planned use.
- **Blocks 100%?** No.

## 26. defer, panic and recover, including across async code

- **Go**: deferred calls run when the function returns or panics, in
  reverse order, with arguments evaluated at the `defer` statement;
  `recover` in a deferred call stops a panic and can change named results;
  a panic in any goroutine that is not recovered ends the program; `os.Exit`
  does not run defers.
- **Rust**: `Drop` runs at scope end; panics unwind and can be caught with
  `catch_unwind`; Tokio catches a panic in a task and keeps going.
- **Recommendation**: each function with `defer` keeps a defer stack (a
  guard that runs it on return and during unwinding); `recover` maps to
  catching the unwind in the deferring frame (`catch_unwind` around the
  body, with the panic value carried as a Go value); a panic that reaches a
  goroutine's top ends the process with Go's message and exit status 2
  (the panic hook already exits on any thread). Async functions keep their defer
  stack in the future's state, so it survives suspension; a guard holding
  only owned handles is `Send`. The first planned step is the restricted
  form `defer wg.Done()`, `defer mu.Unlock()` and similar at function
  level; general `defer` and `recover` follow.
- **Blocks 100%?** No.

## 27. embed

- **Go**: `//go:embed` fills a `string`, `[]byte` or `embed.FS` at build
  time.
- **Rust**: `include_bytes!` and `include_str!`.
- **Recommendation**: the emitter copies the embedded files into the crate
  (listed in the manifest) and generates `include_bytes!` and an
  `embed.FS` implementation with Go's `fs.FS` behaviour.
- **Blocks 100%?** No.

## 28. go:generate, build tags and compiler directives

- **Go**: build tags and `GOOS`/`GOARCH` file suffixes select files;
  `go:generate` runs commands on request; `//go:noinline`, `//go:nosplit`,
  `//go:linkname`, `//go:noescape` and others instruct the compiler.
- **Rust**: `cfg` attributes; no equivalent of `linkname`.
- **Recommendation**: build tags are honoured already (the `patina` tag,
  plus the target's `GOOS`/`GOARCH`); `go generate` is unchanged (default
  mode runs it); optimisation directives are accepted and ignored where
  they do not change behaviour; `//go:linkname` (reaching into another
  package's internals) is rejected by name, except the uses inside the
  standard library itself that the native layer replaces.
- **Blocks 100%?** Partly: third-party `linkname` into the runtime cannot
  work.

## 29. The testing package and go test

- **Go**: `go test` builds a test binary from `_test.go` files, runs
  `Test`, `Benchmark`, `Fuzz` and `Example` functions, with flags (`-run`,
  `-count`, `-race`, `-cover`, `-bench`), output format and caching.
- **Rust**: `cargo test` has different conventions and output.
- **Recommendation**: `gopatina test` (default mode) is `go test`.
  `gopatina test -patina` translates the package with its tests, generates
  a test main like Go's, and prints Go's output format, so the same tests
  check the translated program. `-race` maps to nothing needed (races are
  compile-time fixed); `-cover` needs coverage counters emitted into the
  crate; fuzzing maps onto Go's corpus format with a Rust fuzzing engine.
  Examples compare output exactly, as Go does.
- **Blocks 100%?** No.

## 30. Package initialisation and package-level variables

- **Go**: package-level variables are initialised in dependency order,
  then `init` functions in file order, packages in import order; they can
  be changed by any function, including goroutines.
- **Rust**: statics need constant initialisers or lazy initialisation;
  mutable statics need synchronisation.
- **Recommendation**: compute Go's initialisation order at compile time and
  run it at the start of `main` (before any goroutine exists), storing
  package-level variables in statics. Variables only read after
  initialisation become plain statics; changed ones get the same
  per-access protection as shared locals when goroutines exist.
- **Blocks 100%?** No.

## 31. Generics

- **Go**: type parameters with constraints (interfaces with type sets),
  implemented by dictionaries and GC shapes; type inference; methods
  cannot have their own type parameters.
- **Rust**: monomorphised generics with trait bounds; no type sets.
- **Recommendation**: monomorphise every instantiation GoPatina sees in the
  whole program (it compiles whole programs), so constraints only need
  checking, not expressing as Rust traits. Type-set operations (`+` on a
  `~int | ~float64` parameter) work because each instance is concrete.
- **Blocks 100%?** No.

## 32. Panic messages, fatal errors and exit status

- **Go**: an unrecovered panic prints `panic: <value>` (with Go's
  formatting of the value, `[recovered]` and wrapping rules), a blank line,
  then a trace of every goroutine, and exits with status 2; runtime errors
  print `panic: runtime error: ...`; fatal errors (`all goroutines are
  asleep - deadlock!`, `concurrent map writes`, `unlock of unlocked mutex`)
  print `fatal error: ...` and cannot be recovered.
- **Rust**: `panicked at ...` and exit status 101.
- **Recommendation**: the panic hook prints the panic message and exits with
  status 2 (done); fatal errors print Go's `fatal error:` line (planned with
  goroutines, for deadlock and the sync errors). Rust's standard
  divide-by-zero and index-out-of-bounds text differs from Go's first line,
  so the differential tests translate just those two standard messages before
  comparing; a program's own `panic("...")` payload is never rewritten. The
  goroutine traces after the first line contain addresses and goroutine
  numbers that differ between Go runs too, so they are not reproduced;
  differential tests compare standard output, the exit status and the first
  line of standard error.
- **Blocks 100%?** Partly: programs that parse their own crash traces.

## 33. Algorithms whose output is observable

- **Go**: `sort.Sort` and `slices.Sort` are not stable, so the order of
  equal elements is defined by Go's pdqsort implementation; `math/rand`
  with a fixed seed gives a fixed sequence (and `math/rand/v2`'s PCG and
  ChaCha8 too); `hash/fnv`, `hash/crc32` and `strconv` produce exact
  results; `hash/maphash` is seeded randomly.
- **Rust**: `sort_unstable` uses a different algorithm; `rand` crates give
  different sequences.
- **Recommendation**: translate Go's implementations (entry 1), never map
  to Rust's, so every observable result is bit-identical, with
  differential tests on large random inputs.
- **Blocks 100%?** No.

## 34. Unicode tables

- **Go**: `unicode` holds tables for one Unicode version per Go release
  (`unicode.Version`); `strings.ToUpper` and friends use them.
- **Rust**: `char` methods use Rust's Unicode version, which can differ.
- **Recommendation**: generate the tables from the Go release GoPatina
  targets, so results match that Go exactly. (Phase 1's `rt::trim_space`
  uses Rust's `is_whitespace` above U+00FF: the same Unicode property, but
  from Rust's Unicode version, so it is to be replaced by generated
  tables.)
- **Blocks 100%?** No.

## Keeping this list honest

- A new disconnect found while implementing goes here first, with its
  recommendation, before the code that handles it.
- When an entry is fully handled, it stays, with "done" and a link to the
  tests, so the reasoning remains visible.
- compat.md, not this page, is the record of what is supported.
