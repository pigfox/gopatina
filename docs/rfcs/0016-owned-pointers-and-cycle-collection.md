# RFC 0016: Owned pointers and cycle collection

- **Status:** Accepted (the project lead, 2026-11-23). Implementation in two
  steps: owned pointers (step 1, implemented for pointers that are never
  written through: the analysis in #117, the `rt::Own` representation
  after it; writes and moves out of a place to follow), then cycle
  collection
- **Author(s):** GoPatina contributors
- **Created:** 2026-11-23
- **Discussion:** the pull request that adds this file
- **Supersedes / superseded by:** extends RFC 0013 (its future possibilities
  "`Box` ... where the analysis proves them exact" and "Reclaiming cycles")

## Summary

Two changes to how RFC 0013's heap pointers are represented, both in safe
Rust, both chosen by the analysis per pointer type:

1. **Owned pointers.** A pointer type whose values are never shared (each
   pointer value has one owner at a time, as in a tree built and walked
   from its root) is emitted as `Option<Box<T>>` instead of
   `Option<Rc<RefCell<T>>>`, with the borrow checker's ordinary rules for
   owned values.
2. **Cycle collection.** For the struct types that can form a cycle
   through pointer fields (the types `-explain` already warns about), the
   runtime collects unreachable cycles by trial deletion, so a program that
   builds and drops cycles frees them, as Go's collector does.

## Motivation

The first official three-arm benchmark (the test server, d207c32,
2026-11-23, 100 runs, 5σ intervals;
`bench/results/2026-11-23-server-d207c32-three-arm.json`) has two cases where
GoPatina is slower than both hand-written Rust and plain Go:

| Case | GoPatina / Rust, wall | GoPatina / Go, wall | GoPatina / Rust, instructions | GoPatina / Rust, peak memory |
| --- | --- | --- | --- | --- |
| `binary-trees` | 1.496 [1.422, 1.574] | 1.591 [1.523, 1.662] | 1.372 | 1.473 [1.467, 1.480] |
| `cycles` | 1.999 [1.890, 2.114] | 2.701 [2.532, 2.882] | 0.983 | 62.8 [60.6, 65.0] |

Where the time goes, measured on a workstation on 2026-11-23 (minimum wall
time of three interleaved runs; no performance counters there, so wall
time only):

**`binary-trees`** builds and frees millions of small trees.

| Program | Wall |
| --- | --- |
| Hand-written Rust, `Box<Node>` (allocates 16 bytes) | 5.03 s |
| The same, with the node padded to 40 bytes | 5.81 s |
| Hand-written Rust, `Rc<RefCell<Node>>` (allocates 40 bytes) | 6.00 s |
| GoPatina's crate edited by hand: no pointer copies to read, children lent inside the borrow, plain `Rc` drop | 6.36 s |
| GoPatina at d207c32 | 6.63 s |

Sizes, measured with `malloc_usable_size` (rustc 1.100.0, glibc):
`Box<Node>` allocates a 16-byte payload, which `malloc` serves from a
32-byte chunk. `Rc<RefCell<Node>>` allocates 40 bytes, the node plus two
reference counts and a borrow flag, served from a 48-byte chunk, as is the
`Box` node padded to 40 bytes. Size alone (the padded `Box`) explains about 15% of the 32%
gap, and the representation as a whole (the hand-written `Rc<RefCell>`)
about 19%. What remains is emitted code: reference-count changes for
pointers copied only to be read, and the release loop that keeps freeing a
long list from overflowing the stack. Safe Rust has no smaller shared
cell, so only a different representation closes this gap, and for a tree
`Box` is exact: every node has one owner.

**`cycles`** builds doubly linked lists and drops them. GoPatina executes
as many instructions as the hand-written Rust (ratio 0.983) and is twice as
slow, because it never frees a list (RFC 0013's known limitation): its peak
is 185 MiB against Rust's 3 MiB, and the time goes to the kernel providing
fresh pages. The hand-written Rust changed to leak every list
(`std::mem::forget`) reproduces GoPatina exactly, on the same workstation:
189,720 KB against GoPatina's 189,748 KB peak, 0.308 s against 0.300 s.
Reclaiming the cycles closes it.

## Guide-level explanation

Nothing changes in Go. A tree:

```go
type Node struct{ Left, Right *Node }

func bottomUp(d int) *Node {
	if d <= 0 {
		return &Node{}
	}
	return &Node{Left: bottomUp(d - 1), Right: bottomUp(d - 1)}
}

func (n *Node) check() int {
	if n.Left == nil {
		return 1
	}
	return 1 + n.Left.check() + n.Right.check()
}
```

No `*Node` is ever copied while another copy is used, so `*Node` is an
**owned pointer**, and the crate reads like the hand-written Rust:

```rust
pub struct Node {
    pub left: Option<Box<Node>>,
    pub right: Option<Box<Node>>,
}

impl Node {
    fn check(n: &Node) -> i64 {
        if n.left.is_none() {
            return 1;
        }
        1 + Node::check(n.left.as_deref().unwrap()) + Node::check(n.right.as_deref().unwrap())
    }
}
```

(A nil dereference still panics with Go's message, through a runtime
helper instead of `unwrap`.) Add one line that keeps a second copy, such as
`keep = n.Left`, and `*Node` is shared again: a heap cell as in RFC 0013.
`-explain` says which it chose for each pointer type, and, for a shared
one, the first line that shares it.

A doubly linked list is shared by nature (`x.Prev = tail; tail.Next = x`),
so it stays a heap cell. Its type can form a cycle, so the runtime
collects its cycles: when the program drops the last outside pointer to a
list, the list is freed at the next collection, without any change to the
program. `-explain`'s RFC 0013 warning, "a cycle through it is not
reclaimed", is replaced by a note that the type's cycles are collected.

## Reference-level explanation

### Owned pointer types

The decision is per pointer type `*T` (all values of one type share a Rust
type, so they share a representation). `*T` is owned when every pointer
value of type `*T` in the program is used only as:

- **moved**: its last use, into a variable, field, slice element,
  argument, result or interface;
- **read through**: a field access, method call or dereference, which
  borrows it (a write through it borrows it mutably);
- **tested for nil**: `p == nil`, `p != nil`;
- **lent** to a parameter that the function summaries show only reads or
  writes through it and does not keep it.

and `*T` is not:

- compared with another non-nil `*T` (identity needs sharing);
- the address of a variable (`&x` with `x` declared, a cell variable), only
  of a composite literal or `new(T)`;
- in a package-level variable, a closure capture or a method value;
- reachable from itself through a field of a type that is not owned (a
  shared `*U` inside an owned `*T` is fine; the reverse makes `*T`
  shared, since copies of the `*U` copy access to it).

These are the borrow checker's existing rules for an owned value of type
`T`, applied to `*T`, plus the conditions above that make a pointer
different from a value. A program in which any value of `*T` breaks
them keeps `*T` as a heap cell, exactly as today, so no accepted program
is rejected.

**Representation.** `*T` owned is `rt::Own<T>`, a newtype of
`Option<Box<T>>`, built only through the crate's helpers (one place to
change, as for `rt::Ptr`). (Since RFC 0017 it is the plain
`Option<Box<T>>`, with no newtype, and a lent one is an `Option<&T>`;
see docs/patina-subset.md.) A read is `p.get()` (`&T`, or Go's nil
dereference panic), a write `p.get_mut()`. A move out of a field, which Go
does freely (`t.Left = push(t.Left, k)`), is `std::mem::take`: Go's
semantics are kept because the field is overwritten before it is read
again, which the borrow checker proves for the statement (otherwise the
type is shared). Dropping an `Own` frees its tree iteratively, through the
same release loop as `rt::Ptr`, so freeing a long list cannot overflow the
stack.

### Cycle collection

The struct types that can reach themselves through heap pointer fields are
already known (RFC 0013's `-explain` warning). For each, the crate
implements an `rt::Trace` trait: visit each heap pointer the value holds
(its fields, slice elements and interface values that hold one).

The runtime keeps a buffer of **candidate** cells: a cell whose reference
count is decremented to a value above zero may now be part of a garbage
cycle (Bacon and Rajan's synchronous cycle collection, 2001). When the
buffer passes a threshold, the runtime runs trial deletion over the
candidates:

1. For each cell reachable from a candidate, count the references to it
   from other cells in that subgraph (trace each cell once).
2. A cell whose `Rc::strong_count`, less the buffer's own handle, equals
   its internal count is referenced only from inside the subgraph; one
   with more is reachable from outside, and so is everything it reaches.
3. The cells left are garbage. The collector clears their heap pointer
   fields (setting them to nil through the cell's `RefCell`), which breaks
   every cycle; `Rc` then frees them in the ordinary way.

All of this is safe Rust: `Rc::strong_count`, `RefCell` borrows, and
`Rc<dyn Trace>` handles. A collection runs only between statements of the
program, never inside a cell's borrow, so clearing fields cannot meet a
borrow. Go programs cannot observe when memory is freed (the subset has no
finalizers or `runtime` package), so freeing a cycle later than Go's
collector would, or earlier, changes no output.

**Cost.** Only types that can form a cycle pay anything: a push into the
candidate buffer on a decrement that does not free, and the collections.
Types that cannot form a cycle, and owned pointers, are unchanged.

## Compatibility

No change in default mode. In patina mode no program changes behaviour:
owned pointers are chosen only where they are exact, and collection frees
only what no pointer can reach. Emitted crates change (smaller and closer
to hand-written Rust for trees); the goldens record it.

## Testing

- Goldens for each rule above: an owned tree, each condition that makes a
  type shared, a move out of a field, a nil dereference through an owned
  pointer, a list freed iteratively, and cycles of one, two and many types
  collected.
- The differential test generator (`internal/progen`) builds trees that
  stay owned and lists that become shared, and drops cycles.
- A memory test: a program that builds and drops cycles in a loop runs in
  bounded memory.
- The benchmarks: `binary-trees` and `cycles` against hand-written Rust and
  Go, with the same TOST statistics.

## Drawbacks

- **Two representations for pointers.** Readers of an emitted crate see
  `Own` in one place and `Ptr` in another; `-explain` says why for each.
- **A small change can flip a type.** One added line that keeps a copy
  makes a type shared and slower. `-explain` names the line.
- **Collection pauses.** Trial deletion runs over the candidates, so a
  program that drops large cyclic structures pauses while they are traced,
  as Go's collector does work for them.

## Rationale and alternatives

- **A smaller cell in `unsafe` Rust.** A single word holding the count and
  the borrow state would make a shared node a 24-byte payload, served from
  a 32-byte chunk. RFC 0013 keeps the output safe Rust, and an owned pointer is
  exact without it for the shapes that need it most.
- **`Weak` back pointers** for doubly linked lists and parent pointers.
  Exact only when the node a back pointer reaches is always kept alive by
  its owning pointer, which is hard to prove in general (a Go program may
  keep only the `Prev` of a node). The collector covers every shape;
  `Weak` may later be chosen where the analysis proves it.
- **Arenas.** Fast for trees, but they need each allocation's lifetime
  bounded and change every pointer into an index.
- **Doing nothing.** The two cases stay 1.5 and 2 times slower than
  hand-written Rust, against the goal of identical speed.

## Prior art

- Rust's own `Box` and `Rc`: the hand-written versions of the benchmarks.
- CPython and PHP collect reference-counted cycles by trial deletion;
  Swift leaves cycles to the programmer.
- Bacon and Rajan, "Concurrent Cycle Collection in Reference Counted
  Systems" (ECOOP 2001), its synchronous algorithm.

## Decisions

The project lead answered both open questions: "Yes, on both." (Peter,
2026-11-23).

- **Order.** Owned pointers (`Option<Box<T>>` for pointer types that are
  never shared) come first, cycle collection second.
- **Collection threshold.** Chosen by the benchmarks: both a fixed
  candidate count and one that grows with the surviving cells are
  implemented and measured, and the faster is kept, recorded with its
  numbers.

## Unresolved questions

None.

## Future possibilities

- `Weak` back pointers where the analysis proves them exact.
- Owned pointers in slices and maps.
- `Arc` and a lock for shared pointers with RFC 0002, and a concurrent
  collector.
