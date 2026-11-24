# RFC 0017: One owner: assignment moves a pointer

- **Status:** Accepted (the project lead, 2026-11-23), with the
  recommended option on each of the four decisions below. Implementation
  in small steps, each its own pull request
- **Author(s):** GoPatina contributors
- **Created:** 2026-11-23
- **Discussion:** the pull request that adds this file
- **Supersedes / superseded by:** replaces RFC 0013's shared cells and
  withdraws RFC 0016's step 2 (cycle collection)

## Summary

The project lead, 2026-11-23:

> "Write Go syntax only, and then translate the Go syntax into Rust."
> "So the text that is written, function name, brackets, and so on, that
> is essentially just an interface to what is going to become Rust."

and, about `a := &Node{}; b := a`:

> "that should be illegal in Rust, in the Go patina, or actually a should
> die in this case, I think. And so basically there can be no two names
> for one object. So this is exactly what we're trying to fix."

This RFC makes that the rule for pointers in patina mode. **Every object
has one owner.** `b := a` moves the object from `a` to `b`. After that,
`a` is dead, and using it is a compile error that names the line where it
moved. A function that only uses a pointer borrows it for the call. A
function that keeps it takes it. Nothing is reference-counted, so the
generated Rust uses `Box` and plain borrows (`&`, `&mut`), as hand-written
Rust does.

The programs written stay Go: no new keywords, no annotations. Every
program patina mode accepts still builds with `go build` and does the
same thing. The rule only turns away programs that give one object two
names.

## Motivation

Today a pointer that two names share becomes a reference-counted cell,
`Rc<RefCell<T>>` (RFC 0013). That keeps Go's meaning, but it is not what
a person writing Rust would write. It costs memory and time on every
access, and cycles of such cells are never freed. The
[`cycles` benchmark](../../BENCHMARKS.md) shows that cost: 2.03 times the
hand-written Rust's time, and 185 MiB at peak against 2.96 MiB. RFC 0016
made pointers that are never shared a `Box`. The shared ones stay cells.

The project lead's goal is generated Rust as lean as hand-written Rust,
with no reference counting. Rust gets there by refusing two owners. This
RFC does the same, in Go syntax: the program is written so that each
object has one owner, and the compiler says exactly where a program
breaks that rule.

## Guide-level explanation

Each case below shows Go, then what patina mode does with it: the Rust it
writes, or the error it gives. The errors follow the style patina mode
already uses: the line, what happened, why, and a fix.

### 1. Assignment moves

```go
a := &Node{Val: 1}
b := a              // the Node moves to b
b.Val = 2
fmt.Println(b.Val)  // fine: 2
```

```rust
let a = Box::new(Node { val: 1 });
let mut b = a;
b.val = 2;
println!("{}", b.val);
```

Using `a` after the move is an error:

```go
a := &Node{Val: 1}
b := a
fmt.Println(a.Val, b.Val)
```

```text
./main.go:7:14: a is used after its Node moved to b on line 6
        b := a                     // line 6: the Node moves to b; a is dead from here
        fmt.Println(a.Val, b.Val)  // line 7
    in patina mode an object has one owner: after b := a, only b names the Node
    use b instead of a, or give b its own Node: b := &Node{Val: a.Val}
```

A dead variable comes back to life when it is assigned again
(`a = &Node{}`).

### 2. A temporary second name: a borrow

Walking a list or a tree needs a second name for a while:

```go
sum := 0
for p := head; p != nil; p = p.Next {
	sum += p.Val
}
fmt.Println(sum, head.Val)
```

Under the strictest reading this is an error, because `p := head` moves
the list to `p`. Rust lets you write a borrow here (`let mut p = &head`).
This RFC proposes the same, decided without new syntax: **a local that
is only read through, and is never stored, returned or handed to a
function that keeps it, borrows instead of moving.** The original stays
usable once the borrow ends. Changing or moving the original while the
borrow is still in use is an error, as in Rust:

```rust
let mut sum = 0;
let mut p = head.as_deref();
while let Some(n) = p {
    sum += n.val;
    p = n.next.as_deref();
}
println!("{} {}", sum, head.as_ref().unwrap().val);
```

```text
./main.go:9:3: head is changed on line 9 while p, a borrow of it from line 7, is still in use on line 10
    p only borrows head's list (it is never stored or returned), so head cannot change until p's last use
    finish with p first, or change the list through p
```

Whether to allow this is **decision 1** below. Without it, every list
walk has to be written as a call to a function that borrows its
parameter.

### 3. Function arguments: borrowed or moved, decided by the function

The writer does not mark arguments. As patina mode already does for
slices and strings, each parameter's mode comes from what the function
body does with it:

| What the function does with the pointer | Mode | Rust parameter |
| --- | --- | --- |
| only reads through it | shared borrow | `&Node` |
| changes the object (`p.Val = 2`) | exclusive borrow | `&mut Node` |
| keeps it: stores, returns or hands it to a keeper | moved in | `Box<Node>` |

```go
func total(t *Tree) int { ... }          // reads only: borrows
func (s *Stack) Push(n *Node) {          // keeps n: n moves in
	n.Next = s.top
	s.top = n
}

s.Push(n)
n.Val = 2   // error
```

```text
./main.go:21:2: n is used after its Node moved into s.Push on line 20
        s.Push(n)  // line 20: Push keeps n (it stores n in s.top, main.go:14)
        n.Val = 2  // line 21
    change n before handing it over, or read it back through s
```

`gopatina build -patina -explain` lists each parameter's mode and the
line that decided it. To make a function borrow instead, the writer
changes what the function does (it stops keeping the pointer), not a
marker.

### 4. Methods and receivers

A receiver follows the same rule. A method that reads is `&self`, one that
changes fields is `&mut self`, and one that keeps its receiver is `self`,
which moves the receiver into the call:

```go
func (t *Tree) Depth() int  { ... }             // fn depth(&self) -> i64
func (t *Tree) Insert(v int) { ... }            // fn insert(&mut self, v: i64)
func (n *Node) AddTo(l *List) { l.items = append(l.items, n) }  // fn add_to(self: Box<Node>, l: &mut List)
```

After `n.AddTo(l)`, `n` is dead, as after any move.

### 5. Returns

Returning a pointer moves it to the caller:

```go
func newTree() *Tree { t := &Tree{}; ...; return t }   // fn new_tree() -> Box<Tree>
```

Returning a field of an object the function only borrows would make two
names, the field and the result. Two ways out, and which one applies is
**decision 4**:

```go
func (t *Tree) Left() *Node { return t.left }
```

- *As Rust's getters do*: the result is a borrow of `t`, usable while `t`
  is unchanged and never kept (`fn left(&self) -> Option<&Node>`). A
  caller that tries to keep it gets an error naming `t`.
- *Strictly*: an error, with the fix of detaching the field (below) or
  returning a copy.

### 6. Fields, slices and maps that hold pointers

Storing moves in. `t.left = n` moves `n` into the field, and the field's
old object is freed. `append(nodes, n)` moves `n` into the slice.

Reading through a field or element borrows: `t.left.Val` and
`nodes[i].Val` are fine, and so is `for _, n := range nodes { sum += n.Val }`,
where `n` borrows each element in turn.

Taking an object out of a field or element would leave two names for it,
so it is an error unless the place is cleared or refilled in the next
statement. That pattern is Go's way of writing Rust's `take`:

```go
l := t.left
t.left = nil         // detaches: Rust's t.left.take()
```

```text
./main.go:30:7: x := nodes[i] would give nodes[i]'s Node a second name
    take it out instead: x := nodes[i] followed by nodes[i] = nil
    or only read through nodes[i]
```

Maps are not in patina mode yet. When they arrive, they follow the same
rules: storing moves in, lookups borrow, and `v := m[k]` followed by
`delete(m, k)` takes the value out.

### 7. An explicit copy

Go has no clone. Copying the object a pointer points at, `c := *a`, is
already Go, and patina mode allows it when the copy shares nothing: the
struct holds only numbers, booleans and strings. If the struct holds
pointers or slices, a plain copy would give its children two owners, so it
is an error that says to write a method:

```go
func (n *Node) Clone() *Node {
	c := &Node{Val: n.Val}
	if n.Next != nil {
		c.Next = n.Next.Clone()
	}
	return c
}
```

That is ordinary Go, translated like any other method. (A later RFC could
recognize such methods and emit `#[derive(Clone)]`.)

### 8. Loops

Moving a variable declared outside a loop, inside the loop, is an error
unless the loop gives it a new object before its next use:

```go
for i := 0; i < 3; i++ {
	n := &Node{Val: i}
	n.Next = head     // head's list moves into n.Next
	head = n          // and head gets a new owner before the next turn: fine
}
```

```text
./main.go:12:9: keep(n) moves n on every turn of the loop, but n gets no new Node before the next turn
    give n a new object in the loop, or move it after the loop
```

### 9. Closures

Patina mode has no closures yet; their RFC (function types) comes next
and follows this one. Under this rule, a closure that only reads a pointer
it captures borrows it. A closure that is kept or returned takes the
pointer (Rust's `move ||`), and the outer name is dead afterwards.

### 10. Interfaces and comparison

Converting a pointer to an interface (`var s fmt.Stringer = n`) moves it
into the interface value.

`p == nil` is fine. `p == q` for two pointer variables is an error: under
one owner they can never point at the same object, so the comparison says
nothing.

## Reference-level explanation

The rule applies to every pointer type `*T` of the program and to the
values that hold them: structs with pointer fields, slices of pointers,
and interface values holding a pointer. A place is a variable, a field
path (`t.left.right`) or a slice element (`s[i]`).

1. **Ownership.** Each object made by `&T{...}` or `new(T)` has exactly
   one owning place at a time. Owned pointers are emitted as RFC 0016's
   `rt::Own<T>` (`Option<Box<T>>`), and borrows as `&T` or `&mut T`.
2. **Moves.** An assignment, a call argument whose parameter is moved
   in, a `return`, a composite literal field, an `append` element, a
   channel send (when channels arrive) and a conversion to an interface
   all move. The source place becomes dead. Using a dead place, by
   reading, writing through, passing or comparing it, is an error. Only
   assigning to it revives it.
3. **Borrows.** A parameter, a receiver and (with decision 1) a local
   that the function never keeps is a borrow. Rust's rules apply: while a
   shared borrow is in use, the owner is not changed. While an exclusive
   borrow is in use, the owner is not used at all. A borrow ends at its
   last use (Rust's non-lexical lifetimes).
4. **Taking out of a place.** Moving out of a field or element is allowed
   only when the next statement assigns that place (`nil` or a new value).
   It is emitted as `take()` or `std::mem::replace`.
5. **Paths.** The checker tracks moves per place, so `t.left` can be
   moved while `t.right` stays usable, as in Rust.
6. **Errors** name the use, the move and its line, the reason (which
   statement keeps the pointer), and a fix. `-explain` lists every move
   and every borrow.
7. **What is not affected.** Numbers, booleans, strings and structs of
   them keep today's rules (copied, or moved when unused afterwards).
   Slices keep today's rule, which already refuses a second name whose
   changes could be seen ("a copy of a slice shares its elements").

**Go's meaning is kept.** A program accepted under these rules never uses
a pointer after it moved, so where Go would have two names, the program
has one, and `go build` of the same source does the same thing. Freeing
an object when its owner dies, rather than at Go's next collection,
cannot be observed: patina mode has no finalizers.

## Which Go programs stop compiling

Programs that give one object two lasting names:

- doubly linked lists, trees with parent pointers, and graphs whose nodes
  are reached by more than one path;
- a cache or index that holds a pointer the program also holds elsewhere;
- `b := a` with both used later, beyond a temporary borrow;
- handing one pointer to two functions that both keep it;
- comparing two pointer variables;
- GoPatina's own `cycles` benchmark, and the 10 golden programs that
  share pointers today (`pointers_alias`, `pointers_cells`,
  `pointers_cycles`, `pointers_deep`, `pointers_heap`, `pointers_ifaces`,
  `pointers_methods`, `pointers_more`, `pointers_owned`,
  `pointers_slices`).

The usual rewrite is the one Rust programmers use: objects in a slice,
referred to by index.

```go
type Graph struct{ nodes []GNode }
type GNode struct {
	val   int
	edges []int // indexes into Graph.nodes
}
```

## Relationship to the current patina mode

**Decision 2**: replace, or add a stricter flag.

- **Replace (recommended).** Patina mode's pointer rule becomes "one
  owner". A pointer that is shared today stops being a reference-counted
  cell and becomes an error. The error is RFC 0016's analysis, which
  already finds each shared type and why ("pointers of type *T are
  shared: ... (line N)"). RFC 0013's cells and RFC 0016's step 2, cycle
  collection, go away, since no cycle can form. There is one mode to
  learn and one to document, and the generated Rust has no `Rc`.
- **Stricter flag**, `-patina=owned`. Today's mode stays, with its cells,
  and the new rule is opt-in. More programs are accepted, but there are
  two modes to document and test, and the default output still contains
  `Rc`.

## What it does to the TinyGo yardstick

The standard-library target
([docs/stdlib](../stdlib/README.md): at least TinyGo's 163 importable
packages and 101 passing their tests) is measured against Go's sharing
rules. TinyGo translates Go as written, sharing included. Under one
owner, a package whose own code shares pointers can never be translated
whole: `container/list` and `container/ring` are doubly linked, for
example. Their tests can then never pass through GoPatina. Importing is
hit less, because a blank import translates only a package's
initialization.

So the two goals pull apart for those packages, and **decision 3** is how
to count them:

- (a) provide them in Rust instead of translating them (RFC 0011), as
  `fmt` already is, so a program can use `container/list` while its own
  code keeps one owner;
- (b) keep shared cells for translated library code only, and one owner
  for the program's own code;
- (c) count them as out of scope, as `reflect` and `unsafe` are, and
  state the target as TinyGo's coverage of the remaining packages.

Before deciding, the number can be measured. RFC 0016's analysis can
report, for each of the 172 packages, whether its code shares a pointer.
That measurement would come first, as its own PR.

## Compatibility

- Default mode (`go build`) is unchanged.
- With "replace", patina-mode programs that share pointers today stop
  compiling, each with an error naming the shared type, the line and a
  fix. These are GoPatina's own pointer goldens and the `cycles`
  benchmark. Each is rewritten with one owner or moved to the reject
  tests.

## Testing

- A reject golden for each error above (use after move, change while
  borrowed, moving in a loop, taking out of a field without clearing it,
  copying a struct that holds pointers, comparing two pointers), each
  with its `want.txt`.
- Goldens for each accepted case, built, run, compared with Go, and
  clippy- and rustfmt-clean.
- The program generator writes only one-owner programs (lists, trees,
  stacks, moves through calls, takes) and the difftest compares each with
  Go.
- The emitted crates for these programs contain no `Rc` and no `RefCell`;
  a test checks that.

## Drawbacks

- Some ordinary Go stops compiling. Programs with back pointers have to be
  rewritten with indices, which Go programmers do not usually do.
- Inferred modes make a function's signature depend on its body: when a
  function starts keeping a pointer, its callers' later uses become
  errors. The error says why, and `-explain` shows it, but the change is
  felt at a distance.
- The standard-library yardstick needs decision 3.

## Rationale and alternatives

- **Keep reference counting** (today). It accepts more Go, but the output
  is not what a Rust programmer writes, and it is slower (the benchmarks
  show it).
- **Annotations for borrow and move** (comments or special names). Rejected:
  the project lead's rule is Go syntax only, and the body already says
  what the function does.
- **A collector in the runtime.** That would be Go's model, not Rust's,
  and it is what GoPatina exists to avoid.

## Prior art

- Rust's ownership and borrowing, which this follows case by case.
- Swift's and C++'s move semantics (`std::move`), which keep the moved-from
  name usable; Rust's and this RFC's choice is that it is dead.
- Vale and Austral (single ownership with checked borrows), and arena or
  index-based graphs in Rust.

## Decisions for the project lead

The project lead, 2026-11-23, after seeing the four decisions with their
recommendations:

> "Yes, one owner per object. You don't fucking stop. Keep moving. You
> know what the end goal is. Don't stop for me to give you micromanagement
> instructions. Keep moving, okay?"

So each takes its recommendation: temporary borrows of locals are
allowed; the shared cells are replaced, with no flag; the packages that
share pointers are measured first; and a returned field is a borrow.

The acceptance bar, the project lead, 2026-11-23:

> "Yes. Make it look like Rust. Okay? That's the whole point of this
> exercise. Otherwise, it's a waste of time."

So the generated Rust is held to what a Rust programmer would write for
the same program: owned values and `Box`, `&` and `&mut` borrows,
`Option` instead of nil checks through runtime cells, plain structs and
`Vec`, methods on `&self` or `&mut self`, and no `rt::` wrapper type where
`std` does the job. A test fails if `Rc`, `RefCell` or a runtime cell type
appears in patina-mode output. Side-by-side goldens (the Go, the generated
Rust and a hand-written reference) show how close it reads, starting with
`binary-trees` and one pointer golden. The output stays clippy-clean and
rustfmt leaves it unchanged.

1. **Temporary borrows of locals** (section 2): allow them, inferred as
   Rust does (recommended), or every `b := a` is a move.
2. **Replace or flag** (above): replace today's shared cells (recommended),
   or add `-patina=owned`.
3. **The TinyGo yardstick** for packages that share pointers: (a) provide
   them in Rust, (b) shared cells for library code only, or (c) out of
   scope. The recommendation is to measure first, then decide.
4. **Getters** (section 5): a returned field is a borrow, as in Rust
   (recommended), or an error.

## Unresolved questions

- The exact error texts, settled with the reject goldens.
- Closures follow in the function-types RFC.

## Future possibilities

- Recognizing `Clone` methods and emitting `#[derive(Clone)]`.
- An arena helper for graphs: a provided package that holds nodes in a
  slice behind index handles.
