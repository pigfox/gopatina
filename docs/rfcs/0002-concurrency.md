# RFC 0002: Concurrency in patina mode

- **Status:** Accepted (2026-11-28), with the recommended option on each
  of the seven decisions below and the three conditions under "Conditions
  of acceptance". The coordinator decided for the project lead, under his
  standing order of 2026-11-27: "Stop telling me what to do. Just get it
  done, okay?" The decisions: OS threads; rt::Chan on Mutex and Condvar
  with Go's exact semantics; values move into goroutines and through
  channels; select uniformly random among ready cases; a Mutex owns the
  fields it guards, accessed only between Lock and Unlock in one function;
  deadlock detected, with Go's fatal message and exit 2; a goroutine panic
  ends the program with exit 2. (The first draft, 2026-11-16, chose Tokio
  before RFCs 0017 to 0020; its analysis is kept under "Rationale and
  alternatives".)
- **Author(s):** Peter Sjolin; revised by GoPatina contributors
- **Created:** 2026-11-16
- **Discussion:** this file's pull request, and issues labelled `rfc`
- **Depends on:** RFC 0001 (the subset and the borrow checker), RFC 0017
  (one owner), RFC 0018 (closures), RFC 0020 (recover)

## Summary

Patina mode accepts goroutines, channels, `select`, `sync.WaitGroup` and,
in a later step, `sync.Mutex`. A goroutine is an OS thread (`std::thread`);
a channel is `rt::Chan<T>`, a small runtime type on `std::sync::Mutex` and
`Condvar` with Go's semantics exactly (unbuffered hand-off, buffering,
`close`, `v, ok := <-ch`, `range`, `len`, `cap`, many senders and receivers);
`select` is `rt::select`, choosing uniformly at random among ready cases, as
Go does. Emitted crates stay on the standard library alone. Under RFC 0017's
rule, a value a goroutine uses is moved into it, and a value sent on a
channel is moved into the receiver: the borrow checker turns Go's data races
into compile errors in Go's terms. The seven decisions below were made with
the recommended option on each (see Status).

## Motivation

The parity corpus (137 programs) refuses 16 for concurrency, the largest
cluster left: channels and the go statement (gobyexample channels,
channel-buffering, channel-directions, channel-synchronization,
range-over-channels, select, timeouts, timers, non-blocking-channel-
operations; Go Tour buffered-channels, range-and-close, select), and sync
(mutexes, atomic-counters, mutex-counter, exercise-web-crawler). Eight more
are skipped because their output depends on scheduling, in Go too.

## Guide-level explanation

```go
func worker(id int, jobs <-chan int, results chan<- int) {
	for j := range jobs {
		results <- j * 2
	}
}

func main() {
	jobs, results := make(chan int, 5), make(chan int, 5)
	for w := 1; w <= 3; w++ {
		go worker(w, jobs, results)
	}
	for j := 1; j <= 5; j++ {
		jobs <- j
	}
	close(jobs)
	sum := 0
	for a := 1; a <= 5; a++ {
		sum += <-results
	}
	fmt.Println(sum) // 30, in any order
}
```

In Rust:

```rust
fn worker(id: i64, jobs: rt::Chan<i64>, results: rt::Chan<i64>) {
    for j in jobs.iter() {
        results.send(j * 2);
    }
}

fn main() {
    let (jobs, results) = (rt::Chan::new(5), rt::Chan::new(5));
    for w in 1_i64..=3 {
        let (jobs, results) = (jobs.clone(), results.clone());
        rt::go(move || worker(w, jobs, results));
    }
    ...
}
```

A channel handle is cheap to copy, as Go's is: `rt::Chan` is an `Arc` of
its state, and each goroutine gets its own handle. Everything else a
goroutine uses moves into it.

## Reference-level explanation

| Go | Rust |
| --- | --- |
| `go f(x)`, `go func() {...}()` | `rt::go(move \|\| f(x))`: `std::thread::spawn`, counted for deadlock detection |
| `make(chan T)`, `make(chan T, n)` | `rt::Chan::<T>::new(0)`, `new(n)` |
| `ch <- v`, `<-ch`, `v, ok := <-ch` | `ch.send(v)`, `ch.recv()`, `ch.recv_ok()` |
| `close(ch)` | `ch.close()` (Go's panics: closing a closed or nil channel, sending on a closed one) |
| `for v := range ch` | `for v in ch.iter()` |
| `len(ch)`, `cap(ch)` | `ch.len()`, `ch.cap()` |
| `chan<- T`, `<-chan T` | the same `rt::Chan<T>`; the checker keeps Go's direction rules |
| `select {...}`, with `default` | `rt::select(&[...])`, random among ready cases; `default` when none is |
| `sync.WaitGroup` | `rt::WaitGroup` (`Add`, `Done`, `Wait`, `Go` of Go 1.25) |
| `time.Sleep(d)` | `std::thread::sleep` |
| `time.After(d)`, `time.Tick`, `time.NewTimer` | a channel fed by a timer thread (step 2) |
| `sync.Mutex` with the fields it guards | `std::sync::Mutex<Fields>` (step 3, decision 5) |
| `sync/atomic` | `std::sync::atomic` (step 3) |

**Sharing (decision 3).** A goroutine's function literal moves in what it
uses (as a returned closure does, RFC 0018 step 3): numbers and strings are
copied or moved, and using a moved variable afterwards in the starting
function is RFC 0017's use-after-move error, in Go's terms ("x was moved into
the goroutine started on line 12"). A value sent on a channel moves too, so a
slice sent and then changed by its sender, a race in Go, is refused. Channels
themselves, and `*sync.WaitGroup`, are the shared objects: their handles are
copied. What stays refused, with Go-style messages: two goroutines using the
same variable (other than a channel, a WaitGroup, or a Mutex of step 3), and
a pointer passed to a goroutine.

**Program end.** Go exits when `main` returns, whatever goroutines still
run, and so does a Rust process when `main` returns. A panic in a goroutine
ends the program with Go's message and status 2 (`rt::go` catches it and
exits, as `rt::run_main` does for `main`).

**Deadlock (decision 6).** When every goroutine is blocked on a channel or a
WaitGroup, Go prints "fatal error: all goroutines are asleep - deadlock!"
and exits with status 2. `rt::Chan` and `rt::WaitGroup` count the goroutines
that run and those that wait, and the last one to block prints Go's message
and exits.

**Steps.**

1. `go`, unbuffered and buffered channels, `close`, receive with ok,
   `range`, `len`, `cap`, directions, `sync.WaitGroup`, `time.Sleep`.
   Corpus: channels, channel-buffering, channel-directions,
   channel-synchronization, range-over-channels, buffered-channels,
   range-and-close.
2. `select` with and without `default`; `time.After`, timers and tickers.
   Corpus: select (both), timeouts, non-blocking-channel-operations, timers.
3. `sync.Mutex` and `RWMutex` guarding named fields, and `sync/atomic`.
   Corpus: mutexes, mutex-counter, atomic-counters; exercise-web-crawler
   also needs maps of pointers.

## Testing: never against Go's scheduling

As with map order (RFC 0019), no test may depend on which goroutine runs
first: the goldens print only results that are the same in any schedule
(sums, collected and sorted values, results received in a fixed order from
per-goroutine channels), progen writes no goroutines, and the parity corpus
keeps skipping the programs whose Go output varies between runs. Step 1
adds to the differential test that a golden which starts goroutines runs 20
times, in Go and in Rust (condition 3), so a schedule-dependent golden is
caught.

## Conditions of acceptance

Each is written into the implementation and tested.

1. **No new dependency.** Steps 1 and 2 add no crate dependency: the
   runtime is `std::thread`, `std::sync` and `rt::`. If one ever looks
   necessary, it goes back to the project lead first.
2. **Deadlock detection never fires falsely.** A goroutine blocked in
   `time.Sleep`, on a timer (`time.After`, a ticker), or on I/O is not
   asleep in Go's sense: it wakes on its own. Only goroutines blocked on a
   channel operation, a `select` without `default` whose channels are all
   blocked, or `WaitGroup.Wait` count as asleep; the message is printed only
   when every goroutine, `main` included, is in one of those. A golden proves
   each case that must not fire: a program that is merely slow (a long
   computation while another goroutine waits for it), one whose goroutines
   wait on `time.Sleep` and on a timer while the others block, and one
   blocked reading its standard input while the others wait (if step 1
   supports reading input; otherwise when it is added). A golden that does
   deadlock checks Go's message and exit status.
3. **No flakes.** CI runs every golden that starts goroutines 20 times, in
   Go and in Rust, and treats any run whose output, standard error or exit
   status differs as a defect, never a retry.

## Compatibility

Default mode is unaffected. Programs that rely on a data race, on
`runtime.Gosched`, `runtime.LockOSThread`, `GOMAXPROCS` and similar, are
refused in patina mode.

## Drawbacks

- An OS thread costs far more than a goroutine: a program that starts tens of
  thousands at once may exhaust memory or the thread limit, where Go's would
  not. The corpus starts at most a few dozen.
- A thread's stack is fixed (2 MiB, Rust's default), where a goroutine's
  grows; deep recursion in a goroutine can overflow it.
- One owner refuses Go's common pattern of goroutines closing over shared
  state guarded by a separate mutex, until step 3 recognises it.

## Rationale and alternatives

1. **Runtime: OS threads (recommended), Tokio's work-stealing runtime (the
   first draft), or a runtime of GoPatina's own.** Threads need no
   dependency (emitted crates have none, RFC 0019 decision 1), no async
   colouring of functions, and no CPU-bound detection: a blocking call just
   blocks its thread, as in Go. Tokio matches Go's cost per goroutine, but is
   the crates' first dependency, and needs the whole call graph coloured
   async and `spawn_blocking` for CPU-heavy goroutines. A green-thread
   runtime of our own matches Go best and is a large project of its own.
2. **Channels: an `rt::Chan` on Mutex and Condvar (recommended), or
   `std::sync::mpsc`.** `mpsc::sync_channel(0)` is a rendezvous, as Go's
   unbuffered channel is, but `mpsc` has one receiver (Go's channels have
   many), closes only when every sender is dropped (Go's `close` is
   explicit, and receivers see it at once), and has no `len`.
3. **Sharing: move into goroutines and channels (recommended), or
   `Arc<Mutex<T>>` added automatically (the first draft).** Moving keeps RFC
   0017's one owner and turns Go's races into compile errors; automatic locks
   change performance silently, and need knowing that every access can be
   locked without changing the program's meaning.
4. **select: uniformly random among ready cases (recommended), or the first
   ready case.** Random is Go's rule; a program whose output depends on the
   choice varies in Go too, and is skipped by the corpus.
5. **Mutex (step 3): a Mutex owns the fields it guards (recommended), or
   refuse sync.Mutex.** Go's `type c struct { mu sync.Mutex; v map[string]int }`
   becomes `Mutex<CFields>`: the checker requires that every access to a
   guarded field is between `Lock` and `Unlock` (or after `defer Unlock`) in
   the same function, which is the pattern of all three corpus programs.
6. **Deadlock: detect it and print Go's message (recommended), or let the
   program hang.** A hang is worse in tests and in use, and Go's message is
   part of its behaviour.
7. **Goroutine panics: end the program with Go's message (recommended), or
   let the thread die.** Go ends the whole program; a dead thread that leaves
   the others running would differ.

## Prior art

- Go's memory model and channel semantics (go.dev/ref/mem, the spec's
  "Channel types", "Send statements", "Select statements").
- Rust's `std::sync::mpsc`, `crossbeam-channel` (an MPMC channel and
  `select!` on Mutex and Condvar-like parking), and Tokio.
- c2rust leaves pthreads as threads; translations of Go by hand usually
  pick crossbeam or Tokio.

## Unresolved questions

- Whether a goroutine may borrow from `main` when `main` waits for it before
  the borrowed value changes (scoped threads, `std::thread::scope`): it would
  accept more programs, at the cost of proving the wait.
- `context.Context` cancellation, after step 3.

## Future possibilities

- Tokio, behind an option, for programs with very many goroutines.
- Unbounded worker pools mapped onto a fixed thread pool.
