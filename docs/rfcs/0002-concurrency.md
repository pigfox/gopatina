# RFC 0002: Concurrency in patina mode

- **Status:** Draft (design stage; nothing described here is implemented)
- **Author(s):** Peter Sjolin
- **Created:** 2026-11-16
- **Discussion:** this file's pull request, and issues labelled `rfc`
- **Depends on:** [RFC 0001](0001-gopatina.md) (the patina subset and the
  GoPatina borrow checker)

## Summary

In patina mode (`-patina` and `-emit-rust`), goroutines run as tasks on
Tokio's multi-threaded, work-stealing async runtime. `go f()` becomes
`tokio::spawn`, channels become Tokio channels, `select` becomes
`tokio::select!`. The compiler decides which functions need to be `async`
on its own, so Go code never says `async` or `await`. Data races, which Go
detects only at run time with `-race`, become compile errors, or are fixed
automatically by the GoPatina borrow checker, with Go-style messages.

## Motivation

Go already runs goroutines in parallel: its M:N scheduler multiplexes many
goroutines onto a pool of OS threads, one per CPU by default. Parallelism is
not what patina mode needs to add. The gap is **race safety**. In Go, two
goroutines can share a map or a slice and write to it at the same time; the
program compiles, and the race may be found by `go test -race`, or in
production. Rust rejects such a program at compile time (`Send`/`Sync` and
the borrow checker). Patina mode must keep Go's concurrency model, its
syntax and its performance characteristics, and add Rust's guarantee.

## Options considered

| Option | How goroutines run | For | Against |
| --- | --- | --- | --- |
| OS thread per goroutine (`std::thread`) | Each `go f()` starts a thread | Simplest mapping; no async colouring; blocking calls just work | Threads cost far more than goroutines (stack, creation, switching); programs with many goroutines, idiomatic in Go, do not scale |
| Single-threaded async runtime | Tasks on one thread | Cheap tasks; no `Send` requirements | Not parallel: a regression from Go, which uses every core |
| **Multi-threaded work-stealing async runtime (Tokio)** | Tasks on a thread pool, stolen by idle threads | Closest to Go's M:N scheduler; cheap tasks; parallel; mature, widely used, ecosystem of async crates | Requires async colouring (solved by inference, below); CPU-heavy tasks can starve others (solved by detection, below); a runtime dependency |

**Chosen:** Tokio's multi-threaded runtime. It is the closest match to how Go
already behaves, which keeps the mental model unchanged for Go developers.

## Mapping

| Go | Patina mode (generated Rust) |
| --- | --- |
| `go f(x)` | `tokio::spawn(f(x))`; `x` is moved into the task |
| `make(chan T)` (unbuffered) | a rendezvous channel (a Tokio channel of capacity 1 plus an acknowledgement, to keep Go's hand-off semantics; exact construction to be specified) |
| `make(chan T, n)` | `tokio::sync::mpsc::channel(n)`; multiple receivers use a shared receiver or `async-channel`-style MPMC (to be decided) |
| `ch <- v` / `v := <-ch` | `tx.send(v).await` / `rx.recv().await` |
| `close(ch)`, `v, ok := <-ch` | dropping the senders; `recv()` returning `None` |
| `select { ... }` | `tokio::select! { ... }` (Go chooses fairly at random among ready cases; `tokio::select!` is randomised by default, which matches) |
| `select` with `default` | `try_recv` / `try_send` checks, or `select!` with an immediately ready branch |
| `sync.WaitGroup` | `tokio::task::JoinSet` (or a counter where the tasks are not owned) |
| `sync.Mutex` / `sync.RWMutex` | `tokio::sync::Mutex` / `RwLock` when held across a blocking point, `std::sync::Mutex` otherwise |
| `sync.Once` | `std::sync::OnceLock` / `tokio::sync::OnceCell` |
| `context.Context` cancellation | a cancellation token passed with the task (to be specified) |
| `time.Sleep`, timers | `tokio::time::sleep`, `tokio::time::interval` |
| `func main()` | `#[tokio::main(flavor = "multi_thread")]` when the program uses goroutines |

### CPU-heavy goroutines

A Tokio task that computes for a long time without reaching an `.await`
holds its worker thread, which Go's preemptive scheduler would not allow.
Patina mode handles this in two ways:

- **Detection:** goroutines whose body has no blocking point and contains
  loops the compiler cannot bound are run with `tokio::task::spawn_blocking`
  or on a dedicated thread.
- **Annotation:** a directive (candidate: `//patina:cpu`) on a function marks
  it as CPU-bound explicitly.

This is reported by `-explain` like any other automatic decision.

## Automatic async colouring

Rust requires functions that can suspend to be declared `async` and their
callers to `.await` them. Go has no such distinction. Patina mode infers it:

1. Blocking points are the operations that can suspend: channel sends and
   receives, `select`, `sync` waits, sleeps and timers, and supported
   blocking I/O.
2. A function is async if it contains a blocking point or calls an async
   function. This is computed over the call graph, including through
   interface method calls (an interface method is async if any
   implementation in the program is).
3. The emitter makes those functions `async fn` and inserts `.await` at each
   call. Go source never changes; `-emit-rust` output shows the result.

Function values and interfaces whose implementations differ in colour are an
open question (see below).

## Data races

The GoPatina borrow checker (RFC 0001) treats a value captured by, or sent
to, another goroutine as moved into it. Then:

- A value moved into a goroutine and not used again by the sender: fine.
- A value read by several goroutines and never written: shared with
  `Arc<T>`, automatically.
- A value written by one goroutine and read or written by another:
  **automatically fixed** with `Arc<Mutex<T>>` when every access can be
  wrapped in a lock without changing the program's meaning, and reported by
  `-explain`; otherwise a **compile error** in Go terms, for example:

  ```text
  ./worker.go:22:3: map counts is written by the goroutine started on line 18
      and read by main on line 25 at the same time
      protect it with a sync.Mutex, or send the updates over a channel
  ```

`go test -race` in default mode is unchanged.

## Compatibility

Default mode is unaffected. In patina mode, programs that rely on data races
being benign, or on `runtime.Gosched`, `runtime.LockOSThread` and similar,
are rejected.

## Drawbacks

- Tokio becomes a runtime dependency of every concurrent patina program.
- Async colouring inferred across the whole program makes separate
  compilation of packages harder.
- Automatic `Arc<Mutex<T>>` can hide contention; `-explain` and the strict
  mode of RFC 0001 mitigate this.

## Prior art

- Go's own scheduler and race detector.
- Rust's `Send`/`Sync` traits and Tokio's runtime, whose design is adopted
  rather than reinvented.
- Languages that hide async colouring (for example Go itself, and
  runtimes with green threads); GoPatina does it by inference at compile
  time instead of with stackful coroutines.

## Unresolved questions

- Exact semantics of unbuffered channels on Tokio.
- MPMC channels: which implementation.
- Function values and interface methods with mixed async colouring.
- How cancellation (`context.Context`) maps onto task cancellation.
- Whether `spawn_blocking` detection is reliable enough to be on by default.

## Future possibilities

- A `gopatina vet -patina` check that lists every goroutine, its colour and
  every automatic synchronisation fix.
- A choice of runtime (for example a single-threaded flavour for embedded
  targets) behind a build flag.
