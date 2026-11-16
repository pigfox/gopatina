# Examples: the three build modes

This page walks through GoPatina's three build modes with three small
programs in [`examples/`](examples/):

| Example | What it shows |
| --- | --- |
| [`examples/hello`](examples/hello/main.go) | A struct, a string, a slice: the smallest patina program |
| [`examples/ownership`](examples/ownership/main.go) | A value still used after it was handed off |
| [`examples/counter`](examples/counter/main.go) | Goroutines updating a shared counter behind a mutex |

All three are ordinary Go. They build and run with `go` and with `gopatina`
today, and they are unit-tested at 100% coverage like the rest of the
repository.

> **Read this first.** Only **default mode** works today. Everything shown
> below for `-emit-rust` and `-patina=check`, and for `-patina`, is
> **planned output (design, not yet implemented)**: it shows what GoPatina is
> designed to produce, following [RFC 0001](docs/rfcs/0001-gopatina.md) and
> [RFC 0002](docs/rfcs/0002-concurrency.md). Each planned block is labelled,
> and next to it is the **real** response of the current CLI. No performance
> numbers appear here, because none have been measured.

## The three modes at a glance

| | Default | `-emit-rust` | `-patina=check` | `-patina` |
| --- | --- | --- | --- | --- |
| Accepts | All Go | The patina subset | The patina subset | The patina subset |
| GoPatina borrow and race checks | No | Yes | Yes | Yes |
| Output | Go binary | Readable Rust crate (stops there) | Go binary | Native binary built by rustc from the same crate |
| Garbage collector | Yes (Go's) | None in the generated code | **Yes (Go's)** | None |
| Needs Rust installed | No | No (to build the crate: yes) | No | Yes |
| Status | **Works today** | Planned | Planned | Planned |

`-patina=check` is for machines where Rust cannot be installed: the code is
held to patina's rules, but the binary is an ordinary Go binary (see
[RFC 0001, Deployment and portability](docs/rfcs/0001-gopatina.md#deployment-and-portability)).

## Try it today

This is everything that works right now. The output below is real,
copied from running the commands against this repository with Go 1.27 on
Linux.

```sh
go install github.com/pigfox/gopatina/cmd/gopatina@latest
git clone https://github.com/pigfox/gopatina && cd gopatina
```

```console
$ gopatina run ./examples/hello
Hello, Gopher! Hello, Gopher!
Ferris 3

$ gopatina run ./examples/ownership
queued: Disk almost full
local:  Disk almost full

$ gopatina run ./examples/counter
total: 4000

$ gopatina build -o hello ./examples/hello && ./hello
Hello, Gopher! Hello, Gopher!
Ferris 3
```

Without a patina flag, `gopatina` runs `go` with the same arguments, so the
results are exactly those of `go run` and `go build`.

The patina flags answer, today, with:

```console
$ gopatina build -emit-rust ./examples/hello
gopatina build -emit-rust: Rust source emitter not implemented yet (see RFC 0001)
$ echo $?
2

$ gopatina run -patina ./examples/counter
gopatina run -patina: patina native backend not implemented yet (see RFC 0001)
$ echo $?
2

$ gopatina build -patina=check ./examples/ownership
flag provided but not defined: -patina
usage: go build [-o output] [build flags] [packages]
Run 'go help build' for details.
$ echo $?
2
```

The CLI does not recognise `-patina=check` yet, so it passes the flag to
`go`, which rejects it. It will report "not implemented" like the other
patina flags in a later release.

## 1. Default mode: plain Go

[`examples/hello/main.go`](examples/hello/main.go):

```go
// Greeting is a small struct holding a string, so it is moved, not copied,
// in patina mode.
type Greeting struct {
	Name  string
	Times int
}

// render builds the greeting text. It is the last user of g, so patina mode
// moves g into it.
func render(g Greeting) string {
	return strings.Repeat("Hello, "+g.Name+"! ", g.Times)
}

func run(w io.Writer) {
	g := Greeting{Name: "Gopher", Times: 2}
	fmt.Fprintln(w, strings.TrimSpace(render(g)))

	names := []string{"Ferris", "Gopher"}
	first := names[0]
	names = append(names, "Gordon") // names changes while first is still in use
	fmt.Fprintln(w, first, len(names))
}
```

`gopatina build` and `gopatina run` treat it exactly like `go build` and
`go run` (see [Try it today](#try-it-today)). Nothing about default mode is
planned; it is the compatibility guarantee.

## 2. `-emit-rust`: the Rust GoPatina is designed to write

> **Planned output (design, not yet implemented).** Today this command prints
> "Rust source emitter not implemented yet" and exits 2.

```sh
gopatina build -emit-rust -explain ./examples/hello
```

### What the borrow checker reports

GoPatina's borrow checker runs on the Go program first. With `-explain` it
lists every decision it made on its own:

```text
# Planned output (design, not yet implemented)
./examples/hello/main.go:27:43: moved g into render: g is not used afterwards
./examples/hello/main.go:30:2: cloned names[0] into first: names changes on line 31
    while first is still in use (line 32); Go strings are immutable, so a copy
    keeps the program's meaning
wrote crate ./examples/hello/rust (Cargo.toml, src/main.rs)
```

The second line is an **automatic fix**. In Rust, `first` could not borrow
`names[0]` while `names` grows, so the checker copies the string, which is
what Go's semantics already guarantee. `gopatina fix -patina` would write the
fix back into the Go source, so the code says what it does.

### The generated crate

> **Planned output (design, not yet implemented).** This crate was written by
> hand to show the design. It compiles with `cargo build` (checked with Rust
> 1.93 on Linux, edition 2024) and prints the same output as the Go program.

`Cargo.toml`:

```toml
# Planned output of: gopatina build -emit-rust ./examples/hello
[package]
name = "hello"
version = "0.0.0"
edition = "2024"
rust-version = "1.85"
publish = false

[dependencies]
```

`src/main.rs`:

```rust
// Planned output of: gopatina build -emit-rust ./examples/hello
// Generated from examples/hello/main.go. Design, not yet implemented.

/// Greeting is a small struct holding a string, so it is moved, not copied,
/// in patina mode.
pub struct Greeting {
    pub name: String,
    pub times: i64,
}

/// render builds the greeting text. It is the last user of g, so patina mode
/// moves g into it.
fn render(g: Greeting) -> String {
    format!("Hello, {}! ", g.name).repeat(g.times as usize)
}

fn run() {
    let g = Greeting { name: String::from("Gopher"), times: 2 };
    println!("{}", render(g).trim());

    let mut names: Vec<String> = vec![String::from("Ferris"), String::from("Gopher")];
    // patina: cloned names[0] (names changes below while first is in use)
    let first = names[0].clone();
    names.push(String::from("Gordon")); // names changes while first is still in use
    println!("{} {}", first, names.len());
}

fn main() {
    run();
}
```

```console
$ cargo run --release        # in the hand-written crate above
Hello, Gopher! Hello, Gopher!
Ferris 3
```

Comments, names and structure carry over from the Go source. Go's `int`
becomes `i64` (an open question in RFC 0001 is `i64` versus `isize`).

### Goroutines become Tokio tasks

[`examples/counter/main.go`](examples/counter/main.go):

```go
// count starts workers goroutines, each adding perWorker to a shared total.
func count(workers, perWorker int) int {
	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		total int
	)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perWorker; j++ {
				mu.Lock()
				total++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return total
}
```

> **Planned output (design, not yet implemented).** Hand-written to show the
> design of RFC 0002. It compiles with `cargo build` (Rust 1.93, Tokio 1.54.1)
> and prints `total: 4000`, like the Go program.

`Cargo.toml` (the runtime crate is pinned, per RFC 0003):

```toml
# Planned output of: gopatina build -emit-rust ./examples/counter
[package]
name = "counter"
version = "0.0.0"
edition = "2024"
rust-version = "1.85"
publish = false

[dependencies]
# Pinned runtime crate (RFC 0003); goroutines run as Tokio tasks (RFC 0002).
tokio = { version = "=1.54.1", features = ["rt-multi-thread", "macros"] }
```

`src/main.rs`:

```rust
// Planned output of: gopatina build -emit-rust ./examples/counter
// Generated from examples/counter/main.go. Design, not yet implemented.

use std::sync::{Arc, Mutex};
use tokio::task::JoinSet;

/// count starts workers goroutines, each adding per_worker to a shared total.
// patina: async because it waits for goroutines (wg.Wait)
async fn count(workers: i64, per_worker: i64) -> i64 {
    // patina: mu guards total, so they become one Arc<Mutex<i64>>
    let total = Arc::new(Mutex::new(0_i64));
    // patina: sync.WaitGroup -> JoinSet
    let mut wg = JoinSet::new();
    for _ in 0..workers {
        let total = Arc::clone(&total);
        // patina: go func() { ... }() -> a task on the multi-threaded runtime
        wg.spawn(async move {
            for _ in 0..per_worker {
                *total.lock().unwrap() += 1;
            }
        });
    }
    while let Some(done) = wg.join_next().await {
        done.unwrap();
    }
    *total.lock().unwrap()
}

async fn run() {
    println!("total: {}", count(4, 1000).await);
}

#[tokio::main(flavor = "multi_thread")]
async fn main() {
    run().await;
}
```

What changed, and why:

| Go | Rust | Decided by |
| --- | --- | --- |
| `go func() { ... }()` | `wg.spawn(async move { ... })` on Tokio's multi-threaded runtime | RFC 0002 mapping |
| `sync.WaitGroup`, `wg.Wait()` | `JoinSet`, `join_next().await` | RFC 0002 mapping |
| `mu` guarding `total` | one `Arc<Mutex<i64>>` | Borrow checker: `total` is shared across goroutines and always accessed under `mu` |
| `count` (no `async` in Go) | `async fn count` | Automatic async colouring: it waits for goroutines |

The Go code never says `async`, `await`, `Arc` or `Mutex<i64>`.

## 3. `-patina=check`: patina's checks, then the Go compiler

> **Planned output (design, not yet implemented).** Today the CLI passes
> `-patina=check` to `go`, which rejects it (see [Try it today](#try-it-today)).

`-patina=check` runs the same borrow and race checks as `-patina`, then
builds with the Go compiler. It is for machines where Rust cannot be
installed.

### A use after hand-off, with no safe fix

[`examples/ownership/main.go`](examples/ownership/main.go) is valid Go, and
plain Go runs it (see [Try it today](#try-it-today)): both lines print
"Disk", because the queued copy of `m` and `m` itself share `Body`'s backing
array. Patina mode does not allow two owners to share mutable memory like
that:

```go
func run(w io.Writer) {
	var out Outbox
	m := Message{To: "ops", Body: []byte("disk almost full")}
	out.Queue(m)
	m.Body[0] = 'D' // also changes the queued message

	fmt.Fprintf(w, "queued: %s\n", out.queued[0].Body)
	fmt.Fprintf(w, "local:  %s\n", m.Body)
}
```

```text
# Planned output (design, not yet implemented)
$ gopatina build -patina=check ./examples/ownership
./examples/ownership/main.go:33:2: m.Body is changed after m was handed to out.Queue on line 32
        out.Queue(m)      // line 32: the outbox now holds a copy of m
        m.Body[0] = 'D'   // line 33: this also changes the outbox's copy,
                          // because both share Body's backing array
    if the outbox should keep the original text, queue a copy:
        out.Queue(Message{To: m.To, Body: bytes.Clone(m.Body)})
    if both should change, change m before queueing it
$ echo $?
1
```

There is no automatic fix here. Cloning `Body` would change what the
program prints, because in Go both lines print "Disk", so the checker
explains the problem and stops. The same error is reported by `-patina` and
`-emit-rust`.

### A data race between goroutines

Take the counter **without** its mutex. This is a race in Go:

```go
for i := 0; i < workers; i++ {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < perWorker; j++ {
			total++ // written by every goroutine, unprotected
		}
	}()
}
wg.Wait()
return total
```

Plain Go compiles it; `go test -race` may catch it at run time. Patina mode
catches it at compile time:

```text
# Planned output (design, not yet implemented)
$ gopatina build -patina=check -explain ./counter-racy
./counter-racy/main.go:12:4: total is written by the goroutines started on line 9
    at the same time, with no synchronisation
    fixed automatically: total is protected by a mutex (the Go build gets a
    sync.Mutex around each access; -patina would generate Arc<Mutex<i64>>)
    to make this explicit in the source, run: gopatina fix -patina ./counter-racy
built ./counter-racy (Go compiler, garbage collected)
```

Here an automatic fix keeps the program's meaning (every increment counts),
so it is applied and reported. With the strict mode of RFC 0001, it would be
an error instead.

### What the `-patina=check` binary is

| Checked at compile time (like `-patina`) | Not changed (unlike `-patina`) |
| --- | --- |
| No use after hand-off of shared mutable memory | Still Go's garbage collector |
| No data races between goroutines | Go's runtime and scheduler, not Tokio |
| Ownership and borrows consistent with Rust's rules | Go's performance characteristics, not Rust's |
| Only the patina subset of Go is used | No Rust crate interop |

The build output says which compiler produced the binary. The same source
can later be built with `-patina` without changes.

## How the examples are kept honest

- The Go programs are real, run by CI, and tested at 100% coverage.
- The "Try it today" output was copied from real runs.
- The Rust crates on this page were written by hand to show the design, and
  were compiled and run with cargo before publishing. They will become
  golden tests once `-emit-rust` exists (see [docs/QUALITY.md](docs/QUALITY.md)).
- The borrow-checker messages are the designed wording, not output of
  existing code.
