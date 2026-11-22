# RFC 0013: Pointers that escape

- **Status:** Accepted (#86). Implementation in three steps: escape
  analysis and heap cells (step 1, implemented); slices of pointers and
  `*T` in interfaces (step 2); the `-explain` cycle warning and the pointer
  benchmark (step 3)
- **Author(s):** GoPatina contributors
- **Created:** 2026-11-22
- **Discussion:** the pull request that adds this file
- **Supersedes / superseded by:** extends phase 1's pointers (borrows only)

## Summary

Accept, in patina mode, pointers that outlive the statement that makes
them: returned, stored in variables, fields, slices and interfaces,
compared, and passed to functions that keep them. A pointer that never
escapes stays what it is today, a Rust borrow. A pointer that escapes
points at a **heap cell**: the pointee lives in a reference-counted cell
(`Ptr<T> = Option<Rc<RefCell<T>>>`, `None` for nil), and every access
borrows the cell for that access only, so Go's free aliasing never meets a
Rust borrow held across a call.

## Motivation

Phase 1 supports pointers only as borrows: `p := &x`, `&T{...}`, and
pointer parameters and receivers. Linked structures (`Next *Node`),
constructors (`func New() *T`), pointer fields, pointers in slices and
maps, pointer receivers on values kept in structs, and `*T` as an
interface's dynamic type are all rejected. The coverage measurement at 2ceee0d
(cov12, 2026-11-22), after interfaces and errors (RFC 0012), ranks pointers
first on the roadmap. Its diagnostics about pointers, by message:

| Diagnostics | Message | This RFC |
| --- | --- | --- |
| 14,953 | a pointer variable not starting as `&x` or `&T{}` | heap pointers |
| 11,505 | converting a pointer to an interface | `*T` as a dynamic type |
| 9,083 | a struct field of pointer type | heap pointers |
| 8,510 | a pointer result | heap pointers |
| 7,205 | assigning to a pointer variable | heap pointers |
| 4,889 | taking the address of this expression | escaping `&x`; not `&s[i]`, `&x.f` |
| 2,960 | a slice of pointers | heap pointers |
| 2,839 | a pointer in another position | heap pointers |
| 566 | passing nil for a pointer | nil is `None` |
| 72 | a pointer to a pointer | `Ptr<Ptr<T>>` |

The design below covers every row but the interior pointers among
"taking the address of this expression". (The shares themselves are
reviewed before anything is published; only counts of diagnostics are
quoted.)

## Guide-level explanation

```go
type Node struct {
	Val  int
	Next *Node
}

type Counter struct{ N int }

func (c *Counter) Inc() { c.N++ }

func push(head *Node, v int) *Node { return &Node{Val: v, Next: head} }

func newCounter() *Counter { return &Counter{} }

func main() {
	var list *Node
	for i := 1; i <= 3; i++ {
		list = push(list, i)
	}
	for n := list; n != nil; n = n.Next {
		fmt.Print(n.Val, " ")
	}
	fmt.Println()
	c := newCounter()
	d := c
	d.Inc()
	c.Inc()
	fmt.Println(c.N, d.N, c == d, c == newCounter())
	x := 5
	p := &x
	q := p
	*q = 7
	fmt.Println(x, *p, p == q)
}
```

prints, as Go 1.27.1 prints it,

```
3 2 1 
2 2 true false
7 7 true
```

`list`, `n`, `c`, `d`, `p` and `q` are heap pointers (`Ptr<Node>`,
`Ptr<Counter>`, `Ptr<i64>`): they are stored, returned, copied and
compared. `x` escapes through `p`, so `x` itself becomes a cell, and `x`,
`*p` and `*q` all read it. A pointer that only lends a value to a call
(`f(&x)`, where `f` keeps nothing) stays a borrow, as in phase 1.

## Reference-level explanation

### Which pointers escape

The borrow checker (stage 3) decides, for each pointer value, whether it
escapes. It escapes when it is:

- returned, or stored in a package-level variable, a field, a slice
  element, a map or an interface;
- assigned to a pointer variable after that variable's declaration, or
  copied into a second variable that is used after the first changes;
- compared with `==` or `!=` with another pointer (identity needs a heap
  address);
- passed to a parameter that escapes, as the borrow checker's summaries of
  functions already tell for every parameter.

A variable whose address escapes (`p := &x` with `p` escaping) is a heap
cell itself, from its declaration: every read and write of it goes through
the cell. `&T{...}` and `new(T)` that escape allocate a cell. Everything
else is a borrow, exactly as today, so programs accepted today emit the
same crate.

### Representation

`*T` that escapes is `Ptr<T> = Option<Rc<RefCell<T>>>`, built only through
one crate helper, so the shareable form for goroutines is one change, as
for interface values (RFC 0012's amendment): `Arc` and a lock when RFC 0002
lands. A struct holding a `Ptr` copies it as Go copies a pointer: the copy
points at the same cell.

### Accesses

Every access borrows the cell for that access only:

- a read of `*p` or `p.f` copies the value out (`p.f` of a `Copy` type, a
  clone of a string, a copy of the `Rc` for a pointer field);
- a write `*p = e` or `p.f = e` evaluates `e` first, then borrows the cell
  mutably to store it;
- `p.m(args)` for a pointer-receiver method passes the cell itself
  (`&Ptr<T>`), not a borrow of the value, so the method's own accesses are
  each their own borrow;
- a value-receiver method on a heap pointer copies the value out first,
  as Go does.

No borrow of a cell is ever held while other code runs, so `RefCell`'s
run-time check cannot fail: two accesses never overlap. The emitter writes
each access as one expression with no call inside the borrow; the tests
include aliasing written to catch a violation.

### nil

`None` is nil. Dereferencing nil, through `*p`, `p.f` or a method that
reads its receiver, panics as Go does: standard error starts with
`panic: runtime error: invalid memory address or nil pointer dereference`,
and the exit status is 2 (Go then prints a `[signal SIGSEGV ...]` line and
a stack, which the tests do not compare: they compare standard output,
the exit status and the first line of standard error, as for every
panic). The panic is raised by the one helper every access goes through,
so no access can reach Rust's own `unwrap` message. A method call on a nil
pointer is allowed, as in Go, until the method dereferences it. A nil
pointer of a dynamic type converted to an interface makes a non-nil
interface, as in Go: the variant holds `None`.

### Pointers into slices and fields

A pointer to a slice element or a struct field (`&s[i]`, `&x.f`) that does
not escape stays a borrow, as today. One that escapes is **rejected** in
this RFC, with its own message. Making it exact needs a cell per element
or per field (every slice of `T` a slice of cells, every struct with an
addressed field holding that field in a cell), which changes the
representation and the speed of every slice and struct the program has,
for a pattern the measurement will size. A later RFC can add it where the
analysis bounds it; until then the rejection names it.

### Method values

`p.M` used as a value (not called) is a method value, a closure binding
the receiver; it comes with the RFC for closures and function values, and
stays rejected here, as it is today.

### Proven integer ranges (RFC 0006)

Borrows are unchanged, so every loop RFC 0006 specializes today is
specialized the same way: its counters and bounds are locals or borrows,
never cells. A variable becomes a cell only when its address escapes, and
a loop whose counter or bound is a cell is not specialized: another
pointer may change it inside the loop, so its range cannot be proven. The
specialization's own precondition already requires locals no one else can
change, so this adds no case to it; the benchmarks, which use no escaping
pointers, are unaffected, and the tests check that the specialized golden
programs' crates do not change.

### Comparison

`p == q` is identity (`Rc::ptr_eq`), with `nil == nil`. Pointers are
comparable, so structs holding them and interfaces holding them compare
as Go compares them.

### Interfaces

`*T` becomes a dynamic type (RFC 0012): the variant holds `Ptr<T>`, its
method set has `T`'s value and pointer methods, `%T` prints `*main.T`, and
`%v` prints `&{...}` for a pointer to a struct, as `fmt` does.

### Printing

`%v` of a pointer to a struct prints `&{1 2}`; of any other pointer, its
address, which Go does not specify and the tests cannot compare: printing
a non-struct pointer's address is rejected.

### Not in this RFC

Pointers to slice elements and to fields that escape (`&s[i]`, `&x.f`
kept: interior pointers), `unsafe.Pointer` and `uintptr` arithmetic, and
pointers whose cycle the program relies on being collected (see below).

## Compatibility

Default mode is unchanged. Programs accepted today use only borrows and
emit the same crate.

## Testing

- **Golden programs:** the crates of RFC 0006's golden programs, unchanged;
  the example above; linked lists and trees;
  constructors; pointer fields, slices and maps of pointers; pointer
  receivers through copies; aliases written through one and read through
  another, also inside pointer-receiver methods that reach the same cell
  through a parameter; nil dereference and nil-receiver calls; `*T` in
  interfaces; comparison.
- **Rejections:** each case above that is out of scope.
- **-explain:** a golden program's explain.txt records the cycle warning
  for a self-referencing and a mutually referencing struct type, and none
  for a type that cannot reach itself.
- **Generator:** each program builds a linked structure from its values,
  walks it, mutates it through aliases and compares pointers, from a
  random stream of its own.
- **Benchmarks:** a pointer benchmark (a linked list and a binary tree)
  against hand-written Rust using `Rc<RefCell<T>>` for the same sharing;
  the speed bar applies to it.

## Drawbacks

- **Cycles leak.** Go's garbage collector reclaims a cycle of pointers;
  a reference count does not. Output is unaffected, but a long-running
  program that builds and drops cycles (a doubly linked list, a graph
  with back edges, a node pointing at itself) keeps their memory. This is
  a known limitation, stated where users see it:
  - the site's description of what patina mode supports, and
    `docs/patina-subset.md`, say that pointer cycles are not reclaimed;
  - `-explain` warns at each pointer field whose type can reach itself
    through pointer fields (`Next *Node`, or `A.B *B` with `B.A *A`): a
    struct type that can form a cycle, so a program can leak memory where
    it builds one. The warning names the fields of the cycle, for example
    `Node.Next can point back to a Node: a cycle through it is not
    reclaimed (RFC 0013)`. It is a warning, not a rejection: a list or a
    tree that is never closed into a cycle frees everything;
  - the pointer benchmark measures memory too, with a cyclic case.
- **The cost of a check per access.** `RefCell`'s borrow flag is checked
  on every access. Hand-written Rust with the same sharing pays it too;
  hand-written Rust that restructures ownership may not, which the
  pointer benchmark will show.

## Rationale and alternatives

- **Arenas with indices.** Exact and fast for trees, but changes every
  pointer into a (arena, index) pair and needs whole-program knowledge of
  each allocation's lifetime; a possible optimization for allocation sites
  the analysis can bound, later.
- **`Box` with unique ownership.** Exact only when no pointer is ever
  copied; the escape analysis can choose it for such sites as a later
  optimization.
- **Raw pointers.** `unsafe`; GoPatina's output stays safe Rust.

## Prior art

- gc's escape analysis decides heap or stack for each allocation; this RFC
  decides borrow or cell the same way.
- Swift and Objective-C use reference counting for class instances, with
  cycles left to the programmer.

## Unresolved questions

- How many of the 4,889 "taking the address" diagnostics are interior
  pointers that escape, which the measurement will tell once the rest is
  accepted.
- Whether a cycle collector or arenas come first, decided by measured
  programs.

## Future possibilities

- **Reclaiming cycles.** Two options, to be chosen by measured programs:
  `Weak` back pointers for the shapes the analysis can recognize (a
  doubly linked list's `Prev`, a tree's `Parent`: a field whose cycle
  always runs back through the field that owns it), which is exact and
  free of a collector; or a cycle collector for the cells (trial deletion
  over the cells of the struct types `-explain` already flags), which
  covers every shape at the cost of a pause.
- `Box` and arenas where the analysis proves them exact; `Arc` and locks
  with RFC 0002; interior pointers with a projection of the cell.
