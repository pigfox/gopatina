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

> **Read this first.** Default mode, `-emit-rust`, `-patina` and
> `-patina=check` work for the phase 1 subset of Go
> ([docs/patina-subset.md](docs/patina-subset.md)), which covers `hello` and
> `ownership`. Goroutines are planned (RFC 0002), so everything shown for
> `counter` in the patina modes is **planned output (design, not yet
> implemented)**, labelled as such, beside the real response of the CLI. No
> performance numbers appear here.

## The three modes at a glance

| | Default | `-emit-rust` | `-patina=check` | `-patina` |
| --- | --- | --- | --- | --- |
| Accepts | All Go | The patina subset | The patina subset | The patina subset |
| GoPatina borrow and race checks | No | Yes | Yes | Yes |
| Output | Go binary | Readable Rust crate (stops there) | Go binary | Native binary built by rustc from the same crate |
| Garbage collector | Yes (Go's) | None in the generated code | **Yes (Go's)** | None |
| Needs Rust installed | No | No (to build the crate: yes) | No | Yes |
| Status | **Works today** | **Works today** for the phase 1 subset | **Works today** for the phase 1 subset | **Works today** for the phase 1 subset |

`-patina=check` is for machines where Rust cannot be installed: the code is
held to patina's rules, but the binary is an ordinary Go binary (see
[RFC 0001, Deployment and portability](docs/rfcs/0001-gopatina.md#deployment-and-portability)).

## Try it today

The output below is real, copied from running the commands against this
repository with Go 1.27 and Rust 1.93 on Linux.

```sh
go install github.com/pigfox/gopatina/cmd/gopatina@main
git clone https://github.com/pigfox/gopatina && cd gopatina
```

Default mode is plain Go:

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

The patina modes:

```console
$ gopatina run -patina ./examples/hello
Hello, Gopher! Hello, Gopher!
Ferris 3

$ gopatina build -patina=check -o hello ./examples/hello
gopatina: patina checks passed; building with the Go compiler (garbage collected, not Rust)

$ gopatina build -patina=check ./examples/ownership
./examples/ownership/main.go:33:2: m.Body is changed after m was handed to out.Queue on line 32
        out.Queue(m)     // line 32: out.Queue keeps m
        m.Body[0] = 'D'  // line 33: this also changes the copy out.Queue kept, which shares m.Body's memory
    patina mode does not copy m on its own: a copy would not share that memory, so the program could behave differently
    if out.Queue should keep the original, hand it a copy: Message{To: m.To, Body: bytes.Clone(m.Body)}
    or finish using m before handing it over
$ echo $?
1

$ gopatina run -patina ./examples/counter
./examples/counter/main.go:10:2: package sync is not supported in patina mode yet
    see docs/compat.md for what patina mode supports
./examples/counter/main.go:16:3: type sync.Mutex is not supported in patina mode yet
    see docs/compat.md for what patina mode supports
./examples/counter/main.go:17:3: type sync.WaitGroup is not supported in patina mode yet
    see docs/compat.md for what patina mode supports
./examples/counter/main.go:22:3: the go statement (goroutines are planned, RFC 0002) is not supported in patina mode yet
    see docs/compat.md for what patina mode supports
$ echo $?
1
```

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

## 2. `-emit-rust`: the Rust GoPatina writes

```sh
gopatina build -emit-rust -explain ./examples/hello
```

### What the borrow checker reports

GoPatina's borrow checker runs on the Go program first. With `-explain` it
lists every decision it made on its own:

```text
./examples/hello/main.go:21:13: parameter g is a shared borrow: render only reads it
./examples/hello/main.go:30:11: copied names[0] into first: names[0] is an element of names, and an element cannot be moved out; Go strings are immutable, so a copy keeps the program's meaning
wrote crate examples/hello/rust (6 files, listed in gopatina-manifest.sha256)
```

The second line is an **automatic fix**: Rust cannot move a string out of a
slice element, so the checker copies it, which is what Go's semantics already
guarantee. The first is an inference: `render` only reads `g`, so it borrows
it instead of taking it (the design on this page said "moved g into render";
a borrow is the better Rust, and the output is the same). `gopatina fix`,
which writes such fixes back into the Go source, is planned.

### The generated crate

This is [`examples/hello/rust`](examples/hello/rust), exactly as gopatina
writes it (a test checks that it stays so). `Cargo.toml`:

```toml
# Generated by GoPatina devel (gopatina build -emit-rust).
[package]
name = "hello"
version = "0.0.0"
edition = "2024"
rust-version = "1.85"
publish = false

[dependencies]

# Go integers wrap on overflow; with overflow checks off, Rust's do too.
[profile.dev]
overflow-checks = false

# Reproducible builds (docs/REPRODUCIBLE.md): the same crate and toolchain
# give a byte-identical binary.
[profile.release]
codegen-units = 1
debug = false
strip = true
incremental = false
overflow-checks = false
```

`src/main.rs`:

```rust
//! Generated by GoPatina from package main (main.go).
//! Edit the Go source and regenerate, or keep this crate and continue in Rust.

// Go allows unused functions, types, fields, parameters and assignments,
// wraps integers on overflow, and panics on a division by zero at run time;
// Rust warns about the first and rejects the others when it can see them.
#![allow(
    dead_code,
    unused_assignments,
    unused_variables,
    arithmetic_overflow,
    unconditional_panic
)]

mod rt;

/// Greeting is a small struct holding a string, so it is moved, not copied,
/// in patina mode.
#[derive(Clone, Default, PartialEq)]
pub struct Greeting {
    pub name: String,
    pub times: i64,
}

/// render builds the greeting text. It is the last user of g, so patina mode
/// moves g into it.
fn render(g: &Greeting) -> String {
    format!("Hello, {}! ", g.name).repeat(g.times as usize)
}

fn run(w: &mut dyn std::io::Write) {
    let g = Greeting {
        name: String::from("Gopher"),
        times: 2,
    };
    let _ = writeln!(w, "{}", rt::trim_space(&render(&g)));

    let mut names: Vec<String> = vec![String::from("Ferris"), String::from("Gopher")];
    let first = names[0].clone();
    names.push(String::from("Gordon")); // names changes while first is still in use
    let _ = writeln!(w, "{} {}", first, names.len() as i64);
}

fn main() {
    rt::exit_on_panic();
    run(&mut std::io::stdout());
}
```

`src/rt.rs` holds the two pieces of runtime support the program uses: the
panic hook that makes a panic exit with status 2, as in Go, and Go's
`strings.TrimSpace`. Beside them are `Cargo.lock`, `rust-toolchain.toml`
(Rust 1.93.0), `.cargo/config.toml` (the target) and
`gopatina-manifest.sha256`.

```console
$ cd examples/hello/rust && cargo run --release --quiet
Hello, Gopher! Hello, Gopher!
Ferris 3
```

Comments, names and structure carry over from the Go source. Go's `int`
becomes `i64`; the `io.Writer` parameter becomes `&mut dyn std::io::Write`.

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

> **Planned output (design, not yet implemented).** Goroutines are not in the
> phase 1 subset. This crate was written by hand to show the design of
> RFC 0002. It compiles with `cargo build` (Rust 1.93, Tokio 1.54.1)
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

`-patina=check` runs the same checks as `-patina`, then builds with the Go
compiler. It is for machines where Rust cannot be installed.

### A use after hand-off, with no safe fix

[`examples/ownership/main.go`](examples/ownership/main.go) is valid Go, and
plain Go runs it (see [Try it today](#try-it-today)): both lines print
"Disk", because the queued copy of `m` and `m` itself share `Body`'s backing
array. Patina mode does not allow two owners to share mutable memory like
that. This is the real output:

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
$ gopatina build -patina=check ./examples/ownership
./examples/ownership/main.go:33:2: m.Body is changed after m was handed to out.Queue on line 32
        out.Queue(m)     // line 32: out.Queue keeps m
        m.Body[0] = 'D'  // line 33: this also changes the copy out.Queue kept, which shares m.Body's memory
    patina mode does not copy m on its own: a copy would not share that memory, so the program could behave differently
    if out.Queue should keep the original, hand it a copy: Message{To: m.To, Body: bytes.Clone(m.Body)}
    or finish using m before handing it over
$ echo $?
1
```

There is no automatic fix here. Cloning `Body` would change what the
program prints, because in Go both lines print "Disk", so the checker
explains the problem and stops. The same error is reported by `-patina` and
`-emit-rust`.

### A data race between goroutines

> **Planned output (design, not yet implemented).** Goroutines are planned
> (RFC 0002); today `-patina=check` rejects this program because it uses
> them.

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
- The output in "Try it today" and in sections 2 and 3 was copied from real
  runs. `examples/hello` is built with `go build` and with `-patina` in CI,
  which requires the same output; `examples/ownership` is checked against
  the exact error above.
- `examples/hello/rust` is the crate gopatina emits; a test fails if it
  differs.
- The `counter` crate and the data-race output are still the hand-written
  design for goroutines, labelled as planned.
