# RFC 0018: Closures and function values

- **Status:** Accepted (2026-11-27), with the recommended option on each
  of the four decisions below. The coordinator decided for the project
  lead, under his standing order of that day: "Stop telling me what to do.
  Just get it done, okay?" The decisions: escaping closures move their
  captures; copying a function value moves it; a closure that captures
  only itself is a nested fn; function-typed parameters are generic
  `impl Fn`. Implementation in five steps, each its own pull request:
  non-escaping closures (step 1, in place: called where it is made, with
  number and bool parameters and captures, and a number, bool or string
  result); function-typed parameters; escaping closures and
  function-typed variables; recursive closures; defer of a closure
- **Author(s):** GoPatina contributors
- **Created:** 2026-11-27
- **Discussion:** the pull request that adds this file
- **Supersedes / superseded by:** none

## Summary

Patina mode accepts function literals (closures), function types and calls
of function values. It follows RFC 0017's rule: every object has one owner,
and a captured variable is an object. A closure that is only called while
its variables stay put borrows them, as a Rust closure does. A closure that
outlives them (returned, stored, sent on) owns them: they move into it, and
using one in the enclosing function afterwards is an error that says so. A
function value copied with `g := f` is moved, as a pointer is. A closure
that calls only itself, through the variable that holds it, is a nested
function. The four decisions below were made with the recommended option
on each (see Status).

## Motivation

The parity corpus (testdata/parity, 137 real programs from the Go tour and
Go by Example) refuses function literals in 10 programs, function types in
10 and calls of function values in 9. That is 17 programs in all, and the
refusals of 5 are about closures alone: gobyexample/closures and
recursion, gotour/moretypes/function-closures and gotour/solutions/fib,
which this RFC unlocks, and gobyexample/recover, which also needs
`recover()` (not in this RFC). Maps unlock
none on their own (10 programs need them, all with something else), and
channels need goroutines first. Closures are also the way into the
standard library's callback APIs (sort.Slice, strings.Map, slices.SortFunc,
the iterators of RFC 0011), so most later work builds on this.

## Guide-level explanation

### A closure called where it is made: a borrow

```go
total := 0
add := func(n int) { total += n }
add(1)
add(2)
fmt.Println(total) // 3
```

`add` borrows `total` while it is used; Rust's `let mut add = |n: i64|
total += n;`. Reading `total` between two calls of `add` is the
change-while-borrowed error of RFC 0017 step 5, in Go's terms: "total is
read while the closure add, which changes it, is still used (line 3)". The
fix in Go is the same as in Rust: use `add` last, or read `total` after.

### A closure that outlives its variables: a move

```go
func intSeq() func() int {
	i := 0
	return func() int {
		i++
		return i
	}
}
```

The closure is returned, so `i` moves into it: `Box::new(move || { i += 1;
i })`, typed `Box<dyn FnMut() -> i64>`. Each call of `intSeq` makes a new
counter, as in Go. If `intSeq` used `i` after making the closure, that use
would be the use-after-move error: "i is used after it moved into the
closure returned on line 3".

### Function values: one owner

```go
next := intSeq()
f := next   // next moves to f
fmt.Println(f())
fmt.Println(next()) // error: next is used after it moved to f
```

In Go, `next` and `f` would share one counter; in patina mode a closure,
like a pointer, has one owner. A named function (`f := strconv.Itoa`, or
one of the program's own) has no state, so it is copied: a Rust `fn`
pointer.

### Recursion through a variable: a nested function

```go
var fib func(n int) int
fib = func(n int) int {
	if n < 2 {
		return n
	}
	return fib(n-1) + fib(n-2)
}
```

A closure that captures nothing but the variable it is assigned to becomes
a nested `fn fib(n: i64) -> i64` in Rust. A closure that captures itself
and other variables is refused, with that reason.

### defer of a closure

`defer func() { ... }()` runs the closure when the function returns, as Go
does, through the defer mechanism patina mode already has for
`defer fmt.Println` (#189). With `recover()` it is RFC 0019's: patina mode
has no recover yet.

## Reference-level explanation

**Representation.**

| Go | Rust |
| --- | --- |
| a function literal used only in its own function, called or passed down | a closure, `|args| body`, borrowing what it captures (`Fn`, or `FnMut` when it changes a capture) |
| a parameter of function type `func(T) R` | `impl Fn(T) -> R` or `&mut impl FnMut(T) -> R` (a generic parameter; no allocation) |
| a function literal that escapes (returned, stored in a variable that outlives a capture, a field, a slice, passed to a parameter that keeps it) | `Box<dyn FnMut(T) -> R>`, its captures moved in |
| a variable, field or result of function type | `Option<Box<dyn FnMut(T) -> R>>`, `None` for nil; `Box<…>` when the program never makes it nil (RFC 0017 step 2's analysis) |
| a named function used as a value | a `fn(T) -> R` pointer, copied |
| calling a nil function value | Go's panic, "invalid memory address or nil pointer dereference" |

Escape is decided by the analysis RFC 0017 already does for pointers: the
closure is an object, and its captures are its fields.

**Captures.** A captured variable is borrowed by a non-escaping closure,
for as long as the closure is used, and moved into an escaping one. The
borrow checker's errors keep RFC 0017's wording, with "the closure X" as
the borrower or new owner.

**Fn or FnMut.** A closure that assigns a captured variable, or calls a
method that changes it, is `FnMut`; otherwise `Fn`. A parameter is
`FnMut` if any closure passed to it is.

**Equality.** Go compares function values only with nil: `f == nil`
becomes `f.is_none()`.

**Not in this RFC:** goroutines running closures (RFC 0002), closures as
map values (maps), method values `x.M` (bound methods), generic function
types, and `recover`.

## Compatibility

Plain Go mode does not change. In patina mode, programs that were refused
for "a function literal", "a function type" or "calling a function value"
may now be accepted. A program that shares one closure between two
variables, or reads a variable a live closure changes, gets RFC 0017's
errors in Go terms.

## Drawbacks

- One owner forbids a Go idiom: two function values sharing one closure.
  It is rare in the corpus (no program does it), but it exists.
- `Box<dyn FnMut>` allocates once per escaping closure and calls through a
  pointer, where gc also allocates and calls through a pointer: the same
  costs.

## Rationale and alternatives

The four decisions:

1. **Escaping closures: move (recommended) or share.** Sharing, as Go
   does, needs `Rc<RefCell<…>>` for every captured variable a closure
   changes, with run-time borrow checks: the cells RFC 0017 removed for
   pointers. Moving keeps one owner and plain Rust, and every program in
   the corpus works with it.
2. **Copying a function value: move (recommended) or refuse.** Moving is
   RFC 0017's rule for pointers. Refusing `g := f` is simpler, but
   function values are passed around much more than they are copied, and
   passing one already moves it.
3. **A closure that captures itself: a nested fn (recommended) or a fixed
   point.** A nested function covers recursion (gobyexample/recursion,
   gotour/solutions/fib), and needs no allocation. A fixed-point
   combinator covers more (a recursive closure that also captures), at the
   cost of unreadable Rust.
4. **Function-typed parameters: generic `impl Fn` (recommended) or
   `&dyn Fn`.** Generics compile each call site to a direct call, which
   rustc can inline (sort.Slice's comparator, the hot path). `&dyn` is
   smaller code and slower.

Not doing this leaves every program with a closure refused: 17 of the
corpus's 137 today, 4 of which this RFC alone would let match, and every
use of a callback API later.

## Prior art

- Rust's closures are this design: borrowing by default, `move` to own,
  `Fn`/`FnMut`/`FnOnce` by what the body does.
- TinyGo and gccgo keep Go's semantics, with closures sharing heap-allocated
  variables. Their output is not meant to be read or kept as Rust.
- c2rust translates C function pointers to `Option<unsafe extern "C" fn>`,
  the same nil-as-`None` shape as here.

## Unresolved questions

- Whether `FnOnce` (a closure called at most once, such as most deferred
  ones) is worth inferring, for closures that move a capture out.
- How a closure that captures a loop variable reads in Rust, now that Go
  1.22 gives each iteration its own variable. It is a copy per iteration,
  which is what Go does too.

## Future possibilities

- The callback APIs of provided packages: sort.Slice, slices.SortFunc,
  strings.Map, strings.FieldsFunc.
- Range-over-func iterators (Go 1.23), on top of closures.
- Goroutines running closures (RFC 0002).
