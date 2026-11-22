# RFC 0010: Package-level variables

- **Status:** Draft
- **Author(s):** GoPatina contributors
- **Created:** 2026-11-22
- **Discussion:** the pull request that adds this file
- **Supersedes / superseded by:** none

## Summary

Accept, in patina mode, package-level `var` declarations of every package
of the program, with Go's initialization order and semantics, in safe Rust
(no `static mut`, no `unsafe`). Each variable is classified by what the
program does with it:

- **read-only after initialization** (tables, configuration, sentinel
  values): a `static` `LazyLock`, forced in Go's initialization order;
- **changed, of a copied kind** (numbers, booleans, structs of them):
  a `thread_local!` `Cell`;
- **changed, a string**: a `thread_local!` `RefCell<String>`;
- **changed, a slice or a struct holding one**: rejected by this RFC, with
  its own message, until the borrow analysis covers globals (see
  "Future possibilities").

## Motivation

Package-level variables are one of the roadmap's next steps, and the
standard library's most used ones block much of what RFC 0009 can
translate: lookup tables (`math/bits`' `len8tab`, `unicode`'s range
tables), sentinel values, configuration. Real programs use package-level
counters, flags and caches everywhere.

## Guide-level explanation

```go
package main

import "fmt"

var (
	a = c + b // initialized third: it needs b and c
	b = f("b")
	c = f("c")
	d = 3 // initialized first: b and c need it through f
)

var count int

var names = []string{"x", "y"}

func f(s string) int {
	count++
	fmt.Println("init", s, count, d)
	return d * count
}

func init() { fmt.Println("init()", a, b, c) }

func next() int {
	count++
	return count
}

func main() {
	fmt.Println(a, b, c, d, next(), next(), names[1], len(names))
}
```

prints, as Go 1.27.1 prints it,

```
init b 1 3
init c 2 3
init() 9 3 6
9 3 6 3 3 4 y 2
```

`a`, `b`, `c`, `d` and `names` are never changed after initialization, so
they are read-only statics; `count` is changed, and an `int`:

```rust
static A: LazyLock<i64> = LazyLock::new(|| *C + *B);
static NAMES: LazyLock<Vec<String>> = LazyLock::new(|| vec![...]);
thread_local! {
    static COUNT: Cell<i64> = const { Cell::new(0) };
}

fn main() {
    rt::exit_on_panic();
    LazyLock::force(&D);
    LazyLock::force(&B);
    LazyLock::force(&C);
    LazyLock::force(&A);
    LazyLock::force(&NAMES);
    init1();
    ...
}
```

## Reference-level explanation

### What is accepted

A package-level `var` declaration in any translated package, of a type the
subset supports, with or without initializers, one or several names per
spec, including `var a, b = f()` with a function of two results. Still
rejected, each with its own message: variables of `error` type (they are
part of the error-handling RFC: Go compares them by identity), variables of
`io.Writer` type, and the changed slices described below.

### Initialization order

Go initializes a package's variables stepwise, each step taking the
variable earliest in declaration order that depends on no uninitialized
variable, where dependencies are followed through functions and methods.
`go/types` computes exactly this order (`Info.InitOrder`). Packages are
initialized in RFC 0009's order, and within a package, all its variables
before its `init` functions. `main` does all of it, in that order, before
its own body: it forces each read-only static (`LazyLock::force`) and sets
each changed variable's initial value, so every initializer's side effects
(output, panics) happen in Go's order.

### Classification

A whole-program pass over the translated code decides for each variable
whether anything changes it after initialization: an assignment (`=`,
`op=`, `++`, `--`), `&v`, a method with a pointer receiver called on it, a
call that receives it by exclusive borrow (`RefMut`), a write through an
element or a field (`v[i] = x`, `v.f = x`), or `append` to it.

| Variable | Rust | Read | Write |
| --- | --- | --- | --- |
| never changed | `static V: LazyLock<T>` | `*V`, `V[i]`, `&*V` | — |
| changed, copied kind | `thread_local!` `Cell<T>` | `V.get()` | `V.set(e)`; a field: get, change, set |
| changed, string | `thread_local!` `RefCell<String>` | `V.with_borrow(|v| v.clone())`, or `|v| v.len()` and the like without a clone | `V.replace(e)`, `V.with_borrow_mut(|v| v.push_str(e))` |

A `Cell` or `RefCell` with a constant initial value uses `thread_local!`'s
`const` form, so an access is a thread-local load with no initialization
check.

**Why thread-locals.** Goroutines are not in the subset (RFC 0002), so a
program has one goroutine, and a thread-local is exactly Go's package
variable. RFC 0002 will revisit them for concurrency (atomics and mutexes,
by Go's memory model).

**Why changed slices are rejected.** In Go a copy of a slice shares its
backing array: `s := names; names[0] = "z"` changes `s[0]`. A `RefCell`
copy cannot share; the borrow analysis already decides such sharing for
local variables and parameters, and extending it to globals is its own
design. Read-only slices have no such question, and are accepted.

### Panics in initializers

A panic in a variable's initializer happens where Go's does: forcing the
`LazyLock` in `main` runs the initializer, and a panic in it ends the
program through the existing panic hook, with Go's message and status.

## Compatibility

Default mode is unchanged; programs accepted today emit the same crate.

## Testing

- **Golden programs:** the example above; initialization through
  functions and methods across files and across packages; `var a, b = f()`;
  read-only tables of each supported type read in loops; counters and flags
  of every integer width, `float32`/`float64`, bools and structs of them,
  changed through `=`, `op=`, `++`, fields and pointer-receiver methods;
  changed strings; a panic in an initializer.
- **Rejections:** a changed slice global (with how it is changed), an
  `error` and an `io.Writer` global.
- **Generator:** every program's second package gains read-only tables and
  counters, and the program reads and changes them, from a random stream of
  its own.
- **Differential run:** the per-feature 95% gate; **benchmarks:** none uses
  globals, so their code is unchanged; a small benchmark with a hot loop
  over a read-only table and a global counter checks the cost.

## Drawbacks

- A changed copied-kind global costs a thread-local access per read and
  write. On x86-64 Linux with the `const` form that is one load or store at
  a fixed offset from the thread pointer, but it is not a plain memory
  access; hand-written Rust would usually pass such state explicitly.
- Strings read from a changed global are cloned where the borrow could
  overlap a write; later analysis can avoid more clones.

## Rationale and alternatives

- **`static mut`** is the direct translation and needs `unsafe`, which the
  project rules out.
- **A context struct passed to every function that uses globals** (`g:
  &mut Globals`) is zero-cost and lets the existing borrow analysis decide
  sharing, including for slices. It changes every signature on the path to
  a global and needs the borrow analysis extended; it is the likely design
  for changed slices, maps and structs ("Future possibilities"), and may
  later replace the thread-locals where the speed bar needs it.
- **`Mutex` or atomics for every global** would be ready for goroutines,
  but cost more today for no observable difference.

## Prior art

- Rust's own idiom for lazily computed read-only globals is `LazyLock` (std
  since Rust 1.80); for mutable single-thread state, `thread_local!` with
  `Cell`/`RefCell`.
- gc initializes package variables with generated code in exactly the
  order `go/types` reports, which this RFC follows.

## Unresolved questions

- Whether the sentinel-error case (`var ErrX = errors.New(...)`, `io.EOF`)
  belongs here or in the error-handling RFC; this draft defers it there,
  since its meaning is identity comparison.

## Future possibilities

- Changed slices, maps and structs holding them, through a context struct
  and the borrow analysis.
- Concurrency-safe globals with RFC 0002.
