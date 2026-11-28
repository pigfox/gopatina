# RFC 0020: recover

- **Status:** Accepted (2026-11-28), with the recommended option on each
  of the five decisions below and the three conditions under
  "Conditions of acceptance". The coordinator decided for the project
  lead, under his standing order of 2026-11-27: "Stop telling me what to
  do. Just get it done, okay?" The decisions: recover only directly in a
  deferred literal; its value is Go's panic text, an Option<String>; Go's
  result semantics; catch_unwind in rt::recovering; deferred bodies keep
  the no-panic rule
- **Author(s):** GoPatina contributors
- **Created:** 2026-11-28
- **Discussion:** the pull request that adds this file
- **Supersedes / superseded by:** none

## Summary

Patina mode accepts `recover()` called directly by a deferred function
literal (RFC 0018, step 5), as Go's own rule for recover requires. A
function that defers such a literal runs its body under
`std::panic::catch_unwind`; a panic in the body, or in what it calls,
unwinds to it, runs the deferred calls, and `recover()` in one of them
returns the panic's value and stops the panic: the function then returns
normally, as Go does: each named result with the value it has, and the
zero value for an unnamed one. The five decisions below were made with the
recommended option on each (see Status).

## Motivation

The parity corpus has one program refused for recover alone,
gobyexample/recover (its one fingerprint: "a call other than fmt.Print,
Println, Printf or a math function in a deferred function literal"). Real Go
programs use `defer func() { if r := recover(); r != nil { ... } }()` to turn a
panic into an error, to log and carry on in a loop of independent tasks, and
in tests. Patina mode already has the parts: `rt::Defers` runs deferred calls
as a panic unwinds (#189), and `rt::run_main` gives Go's message and exit
status for a panic that is not recovered.

## Guide-level explanation

```go
func safeDiv(a, b int) (q int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recovered:", r)
		}
	}()
	return a / b
}

func main() {
	fmt.Println(safeDiv(7, 2)) // 3
	fmt.Println(safeDiv(1, 0)) // recovered: runtime error: integer divide by zero
	                           // 0
}
```

In Rust:

```rust
fn safe_div(a: i64, b: i64) -> i64 {
    let q: i64 = 0; // the named result, outside the closure
    rt::recovering(|deferred| {
        deferred.call(move || {
            if let Some(r) = rt::recover() {
                println!("recovered: {}", r);
            }
        });
        a / b
    })
    .unwrap_or(q) // recovered: the named result as it is
}
```

`rt::recovering` runs the body under `catch_unwind`. On a panic it records
the panic's value (Go's text of it, as `rt::run_main` already makes), runs
the deferred calls, and, if one called `rt::recover()` and took the value,
returns the function's results as Go does (here the named result `q`, still
0); otherwise it resumes the panic, which goes on
unwinding to the caller, or to `rt::run_main`, which prints it as Go does.
Without a panic, the deferred calls run when the body returns, and
`rt::recover()` returns `None`, as Go's recover returns nil.

## Reference-level explanation

**Where recover may be called.** Only directly in a deferred function
literal, `defer func() { ... recover() ... }()`, in any statement of its
body. Go's recover returns nil anywhere else (not called by a deferred
function, or called by a function the deferred one calls); patina mode
refuses those (decision 1).

**The value.** `r := recover()` is an `Option<String>`: `None` when there is
no panic (Go's nil), else the panic's value as `%v` prints it, as Go's
runtime prints it after "panic: " (decision 2). `r != nil` and `r == nil` are
`is_some()` and `is_none()`; printing `r` prints the text; `r` may be
assigned to another variable of the same kind. Type assertions on it
(`r.(error)`, `r.(string)`) are refused in this RFC.

| Go panic | the value recover returns, printed |
| --- | --- |
| `panic("a problem")` | `a problem` |
| `panic(err)` | `err.Error()` (RFC 0012's error values) |
| `panic(fmt.Sprintf(...))` | the string |
| index out of range, nil map write, divide by zero, nil pointer | Go's runtime error text, `runtime error: index out of range [5] with length 3` (the same translation `rt::run_main` uses) |

**Results.** A recovered function returns as Go's does (decision 3): an
unnamed result is its zero value, a named result the value it has when the
panic stopped the body (`func f() (n int) { ...; n = 5; panic(...) }`
returns 5). The emitter declares named results outside the closure that
`catch_unwind` runs, so the body changes them in place and the recovered
path returns them. A deferred literal cannot change a named result: it owns
copies of what it uses (RFC 0018, step 5), so Go's
`defer func() { if r := recover(); r != nil { err = ... } }()` stays
refused, as any deferred literal changing a variable is.

**Mechanism.** `rt::recovering(|deferred| body)` (decision 4):

1. makes the function's `rt::Defers` and runs `body` under
   `catch_unwind(AssertUnwindSafe(...))`;
2. on `Err(payload)`, stores Go's text of the panic in a thread-local slot;
3. runs the deferred calls, last first; `rt::recover()` takes the slot's
   value (so a second call returns `None`, as Go's does);
4. if the slot is empty, returns `None`, and the function returns its
   results as above; else it calls `resume_unwind(payload)`. Without a
   panic it returns `Some` of the body's value.

The emitted crates already build with `panic = "unwind"` (defers need it).
The panic hook `rt::run_main` sets keeps recording the message of the latest
panic; a recovered panic prints nothing, as in Go.

**What a deferred function may do.** RFC 0018 step 5's rule stays: a deferred
literal's body may not do anything that can panic, since Rust aborts on a
panic while one unwinds. With `rt::recovering`, the deferred calls of a
recovering function run after `catch_unwind` returns, not during the unwind,
so the rule could be relaxed there; this RFC keeps it (decision 5).

## Conditions of acceptance

Each is written into the implementation and tested.

1. **A recovered panic prints nothing.** Rust's panic hook runs before
   `catch_unwind` catches the unwind, and the default one prints
   "thread 'main' panicked at ..." to standard error; Go prints nothing for
   a recovered panic. The runtime installs a hook that records the panic's
   text and prints nothing (as `rt::run_main`'s hook already does), so
   standard error stays empty unless the panic reaches `rt::run_main`, which
   prints Go's message. A golden program that recovers checks that its
   standard error is empty and its exit status 0.
2. **Emitted crates always unwind.** `catch_unwind` cannot catch a panic
   under `panic = "abort"`, and neither can `rt::Defers` run deferred calls.
   Every profile of an emitted `Cargo.toml` keeps the default, `unwind`; a
   test fails if any profile, in any golden crate or any crate the emitter
   writes, sets `panic = "abort"`.
3. **AssertUnwindSafe is sound here.** The body closure borrows the
   function's state, its named results and, through its parameters, its
   caller's, so it is not `UnwindSafe`, and `rt::recovering` wraps it in
   `AssertUnwindSafe`. That marker is not about memory safety (a panic
   never leaves memory unsafe in Rust; a `RefCell`'s borrow flag, for one,
   is reset as its guard drops while unwinding); it warns that a value may
   be seen half-changed after a panic. That is Go's behaviour exactly:
   after a recovered panic, Go code sees whatever the body had changed
   before the panic, in its named results and through its pointers, and
   nothing else. Patina mode exposes the same, and only that: the named
   results (decision 3) and the deferred literal's own copies (RFC 0018,
   step 5); the body's locals are gone with its closure.

## Compatibility

Plain Go mode does not change. Programs refused for recover may now be
accepted; their output is Go's.

## Drawbacks

- `catch_unwind` costs little on the path without a panic, but it is a
  function boundary the optimizer sees through less well; only functions that
  defer a recovering literal pay it.
- A recovered value as text loses its dynamic type: `r.(error)` and type
  switches on it are refused until a later RFC gives `recover` an interface
  result.

## Rationale and alternatives

1. **Where recover is allowed: directly in a deferred function literal
   (recommended), or anywhere.** Anywhere would need Go's rule that recover
   is nil unless called by the deferred function itself, which means knowing
   at each call whether the caller is running as a deferred call. Refusing
   the other places loses nothing real: there recover is always nil.
2. **The value: the panic's text, `Option<String>` (recommended), or an
   interface value (`rt::Iface`).** The text covers comparing with nil and
   printing, which is what nearly every recover does. An interface value
   keeps the dynamic type for `r.(error)` and type switches, at the cost of
   boxing every panic value and teaching the runtime's own errors to be
   `runtime.Error` values.
3. **Results after recovering: Go's (recommended), or refuse a recovering
   function with named results.** Go's means a named result keeps the value
   the body gave it before the panic, which needs the named results declared
   outside the `catch_unwind` closure; refusing them is simpler, and leaves
   only zero values.
4. **Mechanism: `catch_unwind` in an `rt::recovering` wrapper
   (recommended), or rewriting panics as `Result` returns.** `catch_unwind`
   reuses the unwinding Defers already relies on and changes nothing for
   functions that do not recover. A `Result` rewrite would change the
   signature of every function a panic can pass through.
5. **Deferred bodies: keep step 5's no-panic rule (recommended), or relax it
   for recovering functions.** Relaxing it needs Go's rules for a panic during
   a deferred call (the new panic replaces the old, and recover returns the
   new one), which no program of the corpus needs.

## Prior art

- Go's spec, "Handling panics": recover returns nil unless called directly by
  a deferred function while panicking.
- Rust's `std::panic::catch_unwind` and `resume_unwind` are the documented
  way to stop and restart an unwind at a boundary.

## Unresolved questions

- Whether `recover()` in `main`'s deferred literal should also suppress
  `rt::run_main`'s message: it does, since the panic never reaches it.
- The value of a panic with a value of a struct type with a String method:
  its text, through RFC 0012's printing, like any `%v`.

## Future possibilities

- An interface result for recover, with type assertions (decision 2's
  alternative).
- Named results changed by a deferred function, with recover returning an
  error through them: Go's common pattern
  `defer func() { if r := recover(); r != nil { err = fmt.Errorf(...) } }()`.
