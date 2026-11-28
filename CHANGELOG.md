# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Until 1.0.0, minor versions may contain breaking changes.

## [Unreleased]

### Added

- **Closures over strings (RFC 0018).** Function literals take string
  parameters (as `&str`, so every closure fits `impl Fn(&str)`), use and
  change strings of their enclosing function, and return strings; what a
  closure keeps or hands on is copied, and a returned closure owns the
  strings it uses. `testdata/golden/closures_strings` covers local, passed
  and returned closures.
- **recover, RFC 0020.** `defer func() { if r := recover(); r != nil { ... } }()`
  stops a panic: the function runs its body under `rt::recovering`
  (`catch_unwind`) and returns as Go's does after a recovered panic, named
  results as they are, printing nothing. The panic hook is installed once
  and is silent; emitted crates always unwind, checked by a test.
  `testdata/golden/recover` covers Go by Example's recover and more.
- **Math functions as function values.** `compute(math.Pow)` passes
  `rt::pow`, the runtime function a call of `math.Pow` uses, so the result
  is Go's bit for bit; `Log`, `Exp`, `Sin`, `Cos`, `Sqrt`, `Floor`, `Ceil`
  and `Trunc` too. `testdata/golden/closures_math` is Go Tour's
  function-values, whose closure gc fuses on arm64 and amd64 v3.
- `scripts/gates.sh` runs every local gate (gofmt, vet, race tests with
  100% coverage, the Rust suites, the difftest and, with `--parity`, the
  parity run) with the Go release `.go-version` pins, through `GOTOOLCHAIN`,
  so local float and FMA results match CI's.
- **Range over a map, step 3 of RFC 0019.** `for k, v := range m` is
  `for (k, v) in m.iter()` (or `keys()`, `values()`), in the hash table's
  order, which Go leaves unspecified. Changing the map inside the loop is
  refused. `testdata/golden/maps_range` prints only order-independent
  results: sums, counts, maps built from the entries, and sorted keys.
- **Package-level maps and struct values, step 2 of RFC 0019.** A
  package-level map made by a literal that nothing changes is a `LazyLock`
  static, and map values may be structs of numbers and bools. Go Tour's
  map-literals and map-literals-continued are covered by
  `testdata/golden/maps_global`. Layout follows rustfmt for a closure
  that breaks (a block) and for a lone array argument or element.
- **Maps, step 1 of RFC 0019.** A local `map[K]V` of string, integer or bool
  keys and number, bool or string values is an `rt::Map`, a `HashMap` with a
  fixed hasher: `make`, literals, reads with the zero value for a missing
  key, comma-ok, assignment, `+=`, `++`, `delete`, `len`, and printing with
  the keys sorted, as `fmt` prints a map. `testdata/golden/maps_local`
  covers it, and Go Tour's mutating-maps.
- **Deferred closures, step 5 of RFC 0018.** `defer func() {...}()` is
  moved into the function's `rt::Defers` and runs, in Go's order, when the
  function returns or a panic unwinds. What it uses must not change after
  the `defer`, and its body must not be able to panic (a second panic would
  abort). `testdata/golden/closures_defer` covers it.
- **Recursive closures, step 4 of RFC 0018.** `var fib func(int) int;
  fib = func(n int) int { ... fib(n-1) ... }`, whose literal uses nothing
  of its enclosing function but itself, is a nested `fn fib(n: i64) -> i64`.
  `testdata/golden/closures_recursive` covers Go by Example's recursion.
- **Returned closures, step 3 of RFC 0018.** A function returning a
  function, `func adder() func(int) int`, returns
  `Box<dyn FnMut(i64) -> i64>`, and each `return func(...) {...}` is
  `Box::new(move |...| ...)`, owning what it captures. A variable holding
  one (`pos, neg := adder(), adder()`) is called, or passed to a parameter
  of function type. `testdata/golden/closures_escape` covers Go by
  Example's intSeq and the Tour's adder and fibonacci.
- **Function-typed parameters, step 2 of RFC 0018.** A parameter of
  function type is a generic `impl Fn(...) -> R`, or `impl FnMut` when a
  closure passed to it at some call changes what it captures. Its
  argument is a function of the program, a closure (passed by value when
  it changes nothing, which makes it `Copy`, or lent `&mut`), or a function
  literal in the call, bound just before the statement.
  `testdata/golden/closures_param` covers it.
- **Closures, step 1 (RFC 0018).** `f := func(...) {...}` called where it
  is made: a Rust closure borrowing the numbers and bools it uses, `mut`
  when it changes one. Its parameters are numbers or bools, its result a
  number, bool or string. Using a variable while a closure that changes
  it is still to be called (or changing one it reads) is refused, in Go's
  terms. The body is checked as a function of its own, so its returns are
  its own. Hoisting, evaluation order, NaN tracking and interface results
  stop at function literals. `testdata/golden/closures_local` covers it.
- **panic(err)** and other non-string panics, printed as Go's runtime
  prints them: an error's `Error()`, `runtime error: panic called with nil
  argument` for a nil error, a Stringer's `String()`. Other values are not
  yet supported.
- **The verb %q** of a string, and of a value printed through its String
  or Error method, with a width and the `-` flag. It is Go's
  `strconv.Quote`: double quotes, `\"` and `\\`, `\a \b \f \n \r \t \v`,
  and `\xNN`, `\uNNNN` or `\UNNNNNNNN` for every rune Go does not consider
  printable. `rt::quote` carries `strconv/isprint.go`'s tables, from the Go
  release the crate was made with. It matched `strconv.Quote` on every
  Unicode scalar value (1,112,064 runes). A constant is quoted when the
  crate is made. Not yet: `%q` of a rune or a `[]byte`, `%+q`, `%#q`, a
  precision, or the `0` flag.
- **Printing structs.** `fmt.Println(p)`, `Print`, `Sprint` and plain
  `%v` of a struct value print Go's `{f1 f2 ...}`: each field by its own
  `%v`, a nested struct in braces, a slice in brackets, floats as Go
  formats them. Each printed struct type gets a `show_` function in the
  crate. Fields of printable basic types, slices of them and such structs
  are supported. Not yet supported: a field `fmt` prints through its
  String or Error method (Go calls it for exported fields only), pointer
  and interface fields, and `%+v`, `%#v` or a width.
  `testdata/golden/print_structs` covers it. The slice helper interfaces
  used, `iface_slice`, is now `rt::show_slice`, shared by both.
- **A canary for the next Rust edition.** Rust 2027 has no number yet
  (nightly calls it `future`). `scripts/edition-canary.sh` compiles every
  committed crate without dependencies with nightly's next edition, with
  future-incompatible and edition-future-compatibility lints and every
  warning denied. All 189 compile today. It runs in the `toolchains`
  workflow, and a scheduled failure opens an issue.
- **math.Pow**, bit for bit with Go on every chip. Go writes it in Go
  (`pow.go`) on every target: Exp and Log for the fraction of y, then
  squarings for its integer part, with Frexp and Ldexp through the
  subnormals. GoPatina ports it with each target's Exp and Log. gc fuses
  one multiply-add on arm64 and from `GOAMD64=v3`, a square then its
  doubling, which always rounds to the same value, so one port serves
  every target. `testdata/golden/math_pow` checks the special cases and
  400,000 random pairs.
- **Go's pin and a watcher.** CI's Go is pinned to go1.27.4
  (toolchain.json, `.go-version`), with the `fma-sites.txt` records
  rewritten from it, so TestFMASites compares them exactly instead of
  logging a version difference. `scripts/go-bump.sh` moves the pin. Each
  day the toolchain bump workflow tries the newest Go release (a pull
  request when every gate passes) and each new release candidate (an issue
  with the result).
- **math.Sin and math.Cos**, bit for bit with Go on every chip. Go writes
  them in Go (`sin.go`, and `trig_reduce.go`'s Payne-Hanek reduction from
  2**29 on), and gc fuses their multiply-adds differently for each target:
  on arm64 the reduction and every polynomial step, from `GOAMD64=v3` each
  polynomial's outer add, below v3 none. GoPatina ports each choice;
  nothing is refused at run time. `testdata/golden/math_trig` checks
  400,000 random arguments (every exponent, and within 64 pi of zero) and
  the edge values on amd64 at v1 and v3, on arm64, and on a Nehalem.
- **-static**: `gopatina build -patina -static` (and `-emit-rust -static`)
  builds a fully static binary on linux/amd64 and linux/arm64. It uses
  Rust's musl target, linked by `rust-lld` with Rust's own musl runtime,
  so it needs no C toolchain, and it is reproducible. It is opt-in: musl's
  allocator is 1.25 to 2.3 times slower than glibc's on the allocating
  benchmarks (bench/results/2026-11-27-allocators.md), so gnu stays the
  default, as RFC 0001's amendment records.
- **Hardware FMA on amd64.** A crate built for baseline x86-64 compiled
  each `mul_add` to a call to the library's `fma`, which musl does in
  software. math.Exp's fused path is now compiled for AVX and FMA, and
  `rt::fma_amd64` uses VFMADD when std::arch finds FMA, so each fused
  multiply-add is one instruction in line. Both run only where the
  processor has the feature (the same check that already chose the fused
  path), and the values are unchanged: rounded once either way.
- **toolchain.json**, the one place the toolchain versions are written
  (the pinned Rust release, the MSRV, the edition, go.mod's `go`).
  `TestToolchainDrift` fails when a copy disagrees: internal/emit's
  constants, every committed crate, the workflows, the README.
  `scripts/toolchain-bump.sh` rewrites every copy, and a weekly
  `toolchain bump` workflow runs it for the newest Rust stable and beta,
  runs every gate on the branch (goldens, float bit for bit on three
  targets, difftest on amd64 and arm64, clippy, bench) and opens a pull
  request for stable when they pass, or an issue when one fails.
- **math.Log**, bit for bit with Go on every chip: amd64's assembly
  (`log_amd64.s`, whose own Frexp leaves a subnormal argument as it is,
  whose reduction takes `f1 <= Sqrt2/2` where `log.go` takes `<`, and which
  returns Go's NaN for a negative NaN), and on arm64 `log.go` as gc compiles
  it there, ten of its multiply-adds fused (the sites gc reports with
  `-d=fmahash`). `testdata/golden/math_log` runs on amd64, arm64 and a
  Nehalem (qemu, with `math_exp`, both built static).
- **math.Exp**, bit for bit with Go on every chip. Go writes it in
  assembly for each target, and GoPatina ports each: amd64's
  (`exp_amd64.s`), fused when the processor has AVX and FMA and GODEBUG
  does not turn them off (`cpu.fma=off`, below `GOAMD64=v3`), as Go decides
  at start-up, and arm64's (`exp_arm64.s`). `testdata/golden/math_exp`
  checks 400,000 random arguments and the edge values on amd64 with FMA,
  without it (a CI job running both programs with `GODEBUG=cpu.fma=off`),
  and on arm64.
- **NaN payloads match gc** (go-rust-disconnects.md, entry 36). Rust
  leaves a NaN result's payload unspecified, and rustc folded what it
  could see to its own NaN, where gc computes on the hardware: `Inf-Inf`
  printed `0x7ff8…` through `math.Float64bits` instead of x86's `0xfff8…`.
  The float operations whose values a bit-level read sees now give the
  target's NaN (`rt::add_nan` and the like, plain IEEE otherwise); a `+`
  or `*` of two values that may both be NaN, whose bits are read, is
  refused, since gc may swap their operands. The generated programs
  print the bits of NaN results, and testdata/golden/nan_payloads checks
  them on amd64 and arm64.
- **defer of a print** (`defer fmt.Println("done")`, `Print`, `Printf`) of
  numbers, strings and booleans: arguments evaluated at the `defer`, run
  last first on return and while a panic unwinds, before Go's panic
  message and exit status 2; `os.Exit` skips them. `recover` and other
  deferred calls are not supported yet.
- **Float results match gc on every chip** (go-rust-disconnects.md, entry
  35). gc fuses a float multiply and add into one rounding on arm64 and on
  amd64 from `GOAMD64=v3`, and Rust never does, so on those targets a
  program could print other bits than its `go build`. GoPatina now asks gc,
  for the same target and Go version (`-gcflags=all=-d=fmahash=vy`, checked
  first on a probe), where it fuses, and fuses exactly there with gc's NaN
  rules (`rt::fma_arm64`, `rt::fma_amd64`); the crate's header names the gc
  it matches. What it cannot match yet is refused, never written unfused.
  CI runs every golden program on arm64 and at `GOAMD64=v3`.
- **Fused multiply-adds through a local, and float32**: a product held in
  a local set once to it (`p := a * b`, then `p + c`), which gc fuses
  across statements, is fused too, with its factors as they were at the
  multiply; float32 multiply-adds have their own helpers. On arm64 this
  was 51 of 1,608 generated programs printing other float32 bits than
  `go build`.
- **Parity on real programs** (`cmd/gopatina-parity`, docs/QUALITY.md).
  GoPatina measured on 190 programs written for people: Go by Example and
  A Tour of Go, vendored in `testdata/parity` with their licences. Each is
  taken through Go, `-emit-rust`, cargo and clippy, and its output compared
  with Go's; programs that need the network, other programs, signals, or
  whose output varies are skipped, never counted as passing. Failures are
  clustered by fingerprint. Baseline: of 139 tested, 23.7% accepted, 23.0%
  compiled, 21.6% matching Go. A weekly workflow fails when fewer programs
  get through than in the baseline.
- **Package math, the functions exact by IEEE 754** (RFC 0011). `Sqrt`,
  `Abs`, `Floor`, `Ceil`, `Trunc`, `Copysign`, `Signbit`, `IsNaN`, `IsInf`,
  `Inf`, `NaN`, `Float64bits` and `Float64frombits` are provided in Rust,
  not translated, each giving Go's result bit for bit, NaNs included
  (Go's quiet NaN from `Floor`, `Ceil` and `Trunc`, Go's own `NaN()`).
  `testdata/golden/math_exact` checks them over edge values and 200,000
  random inputs on amd64 and, in a CI job of its own, arm64. `Max` and
  `Min` are left out: Go returns a different NaN for them on amd64 and on
  arm64.
- **The verb `%g` of a float** (`fmt.Printf("%g", x)`, with a width and
  the `-` flag). For a float it is Go's `%v`: the shortest digits that read
  back as the number, with an exponent below 1e-4 and from 1e+06 up.
  `%g` with a precision, which counts significant digits, and `%+g`, which
  prints a sign where `%+v` does not, are not supported yet.

### Removed

- **Shared cells** (`rt::Ptr`, RFC 0013). Every pointer has one owner
  (RFC 0017), so the reference-counted cells, their runtime pieces
  (`rt::Ptr`, `rt::show_ptr`) and the borrow checker's cell rules are
  gone. A pointer type whose objects would still have more than one owner
  is not supported yet, with the use that gives them a second one. A
  value method that changes its copy, called through a pointer, stays an
  error. `-explain` no longer warns of cycles that are not reclaimed: none
  can form.

### Changed

- **Parity clusters a standard-library refusal by the program's own call**
  (`cmd/gopatina-parity`). A refusal met inside standard-library code the
  program reaches is now "calls filepath.Join, whose standard-library code
  patina mode does not support yet", the first call of its "reached from
  main through:" chain outside package main, instead of the internal
  package five calls below it; code reached through a package's
  initialization keeps the package's name. Names in these fingerprints are
  no longer taken for the program's own (`fs._`, `the standard package _`).
- **Difftest disk use is bounded** (docs/QUALITY.md). Each batch's crate
  cache and cargo targets go with the batch, the run's Go build cache is
  cleared past `-gocache-max` (2048 MiB), a run stops before a batch when
  less than `-min-free` (15%) of its disk is free, naming the batch to
  resume from, a signal stops the builds running (each in a process group
  of its own, so a timed-out or stopped build takes its compilers with it)
  before the run removes its files, and a run removes the temporary
  directory of an earlier run that was killed outright. A long run on the tools box had grown its
  temporary directory to 48 GB, 40 GB of it Go build cache.

- **Interfaces own the pointers they hold** (RFC 0012 with RFC 0017). A
  pointer converted to an interface moves into it (`Dyn::PtrT(Option<Box<T>>)`),
  so the types an interface holds have one owner instead of a shared cell
  (`rt::Ptr`). `x.(*T)`, `errors.As(err, &q)` and a type switch's pointer
  binding borrow the pointer the interface owns; a range over interface
  values that changes them iterates mutably (`iter_mut`); `%w` moves the
  error it wraps. New errors, each with its hint: a pointer converted
  while still used, a field's pointer converted, an interface value
  holding a pointer copied while still used, and a method keeping its
  receiver called through an interface. A nil pointer variable may be
  converted and still used. pointers_cells, pointers_ifaces and
  pointers_methods hold no cell now (4 crates to 1 in
  testdata/shared-cells.txt). New golden pointers_iface_borrows; reject
  goldens owned_iface_converted_used, owned_iface_field_converted,
  owned_iface_copied, owned_iface_assert_interleaved and
  owned_iface_kept_receiver.

- **Interface values hold their value, with no reference count**
  (RFC 0012 with RFC 0017). An interface value is `Option<Dyn>`, or
  `Option<Box<Dyn>>` where a dynamic value can hold an interface value,
  in place of `Option<Rc<Dyn>>`. A copy that is only read while its source
  does not change is a borrow (`b := a` is `let b = &a;`); any other copy
  is a clone, Go's copy. Error values keep Go's identity through an id
  their copies share (`rt::error_id`), and `rt::Error` no longer compares
  by message. 19 crates leave testdata/shared-cells.txt (23 to 4). New
  bench cases `iface-copy` and `iface-keep`.

- **Second owners are errors** (RFC 0017). A use that gives an object a
  second owner made its pointer type a shared cell (`rt::Ptr`) until now;
  it is now an error naming the use, why Rust has one owner, and how to
  write it: a pointer copied out of a place and kept, a range variable
  that keeps an element, a variable's address held while the variable is
  still used by name, and a borrow a function lends back kept. Types an
  interface holds keep their cells until interface values are owned.
  Reject goldens `owned_aliases_kept`, `owned_address_kept` and
  `owned_returned_borrow_kept`; pointers_owned and pointers_slices now hold
  no cell.

### Added

- A value method of a type holding a slice, called through an owned
  pointer, borrows the box's value (RFC 0017): `&self` where it reads,
  `&mut self` where it changes the slice's elements (`s.clear()`,
  `nums_set(&mut ns, 9)`), as Go's copy shares the slice. Until now the
  copy kept the type a shared cell. pointers_more holds no cell now.
  Golden `owned_value_receivers`.

- A function may walk from its input to the node it returns (RFC 0017):
  `for p := head; p != nil; p = p.Next { if p.V == v { return p } }` lends
  `head` back, `fn find(head: Option<&Node>, v: i64) -> Option<&Node>`.
  Until now the walk kept the type a shared cell. Golden
  `owned_walked_borrow`.

- A function that lends back a variable's address makes a local borrow
  of the variable (RFC 0017): `keepC := hold(&c)` is
  `let keep_c = hold(Some(&mut c))`, as `p := f(x)` was for a pointer
  variable. Until now it was an error (since second owners are errors),
  and before that a shared cell. A change to `c` while `keepC` is in use
  is refused by the borrow checker (reject golden
  `owned_returned_borrow_addr`).

- More local values move into a new box at their `&` (RFC 0017), instead
  of living in a shared cell: each of several names declared together
  (`a, b := "a", "b"; return &a, &b`), and a local slice of data
  (`ns := Ints{1, 2}; return &ns` is `vec![1, 2]`), which is then indexed
  and measured through its pointer (`(*p)[0] = 5`, `len(*p)` are
  `p[0] = 5`, `p.len()`). A string or other value borrowed through an
  owner that may be nil is `y.as_ref().unwrap()`, not `&*`. Golden
  `owned_escape_values`.

- A struct value holding owned pointers may be a field of an owned
  pointer's struct (RFC 0017): `type Outer struct{ In Inner }` with
  `type Inner struct{ Back *Outer }` is `pub r#in: Inner` beside
  `pub back: Option<Box<Outer>>`, not a shared cell. pointers_cycles holds
  no cell now; `o.In.Back = o`, a cycle, is refused (reject golden
  `owned_struct_value_cycle`).

- A field's address lent to a call keeps its owned pointer owned (RFC
  0017): `inc(&p.n)` is `inc(&mut p.n)`, `(&p.in).Inc()` and `p.in.Inc()`
  are `p.r#in.inc()`, and through a pointer that may be nil the field is
  reached with `as_mut()`. Until now such a lend made the type a shared
  cell. Another argument reading through the same pointer is computed
  first (a string's as a copy); one that changes it is an error (reject
  golden `owned_lend_through_nil`). An `Option<&mut T>` parameter lent to a
  call computed before an earlier read of it is reborrowed
  (`t.as_deref_mut()`), not moved. Golden `owned_field_lends`.

- A temporary borrow may be compared with another pointer, by address
  (`for p := head; p != stop; p = p.Next` is
  `while p.map(std::ptr::from_ref) != stop.map(std::ptr::from_ref)`),
  and an element of a slice of owned pointers may be given a pointer
  (`ps[0] = leaf(7)`, `ps[0] = Some(leaf(7))`). Both kept the type a
  shared cell until now.

- Owned pointers compare by address without sharing their type (RFC 0017):
  `a == b` is `std::ptr::eq(&*a, &*b)` for two never nil,
  `a.is_some_and(|p| std::ptr::eq(p, b))` where one may be nil, and
  `a.map(std::ptr::from_ref) == b.map(std::ptr::from_ref)` where both may
  be, for owners, borrows (parameters, local and returned borrows) and new
  pointers alike. Until now a comparison with another pointer made the type
  a shared cell. A parameter compared that way stays a `&T`, and a local
  compared that way stays a `Box<T>`. Programs that kept a second pointer to
  an object beside its first, and compared the two, were compiling only
  because of that comparison; they are now refused as the aliasing they are
  (reject golden `owned_aliases_compared`).

- A loop condition or `switch` case that reads a variable before a call
  that changes it (`for x+addTo(&x, 1) < 20 { .. }`), refused until now,
  compiles: Rust's `loop { let call = ..; if x + call >= 20 { break; } .. }`
  makes the call first each time round, as gc does, the post statement
  still run before each `continue`; a case's calls are made in an `else` of
  the cases before it. Golden `call_before_read`.

### Fixed

- A loop nest that indexes with a constant expression whose parts are not
  integers, such as `a[i+3/2.0*2]`, no longer crashes the emitter (found by
  FuzzEmit in CI); the fuzz job now uploads a failing input.
- A panic message of several lines now indents each line after the first
  with a tab, as Go's runtime does; it was printed as it was.
- `panic` of a named string type printed the bare string; Go prints
  `main.T("s")`.
- **Clippy warning**: a program keeping one of main's unexported struct
  types in an interface got a crate that warned `private_interfaces`: the
  interface enum `Dyn` is public and held the private struct. Such a crate
  now allows the lint, as a crate of several packages already did
  (gobyexample/interfaces in the parity corpus).
- **Wrong output**: a constant shifted by a count known only at run time
  (`uint64(1) << s`, `var c int64 = 1 << s`) was shifted as Rust's
  default `i32`, since its type was not written: `1 << 40` gave 0. It now
  has the type Go gives it (`rt::shl(1_u64, ...)`). The benchmark
  `binary_trees` computed `1 << (maxDepth - d + 4)` this way, wrong from
  depth 28 on.
- **Refused**: an `&x` given to a parameter that its callee keeps, with
  nothing using `x` after (`keepIn(h, &tt)`), was refused as "&tt is copied
  and kept, so its object has two owners" since second owners became
  errors, though only one owner exists; before that it compiled through a
  shared cell. It is now an escape box, `x` moved into a new Box at the
  `&`: `keep_in(&mut h, Some(Box::new(tt)))`. Golden `owned_kept_addr`.

- **Shared cells**: a pointer whose type another package of the program
  declares (`var head *list.Node; head = list.Push(head, 1)`) was always an
  `rt::Ptr` cell, never owned, because only main's own types and functions
  counted as the program's. They now do for every translated package, so
  such a pointer is an `Option<Box<list_0::Node>>` like main's own. A
  multi-package binary also allows `private_interfaces`, since main's
  exported types may hold another package's, which are `pub(crate)`.
  Golden `owned_packages`.

- **Wrong output**: an expression that read a variable, or a place under
  one, and then called a function or method that changed it printed a
  different result from Go. `fmt.Println(p.V+inc(p))` printed the old `p.V`
  plus the call's result; Go (gc) makes the call first and reads the new
  `p.V`. The same held for a slice element (`xs[0]+first(xs)`), a local
  through `&x` (`x+addTo(&x, 3)`), a receiver (`q.X+q.Inc()`), an element's
  receiver, an `if` condition and a `switch` tag. The Go spec leaves the
  order unspecified; GoPatina matches gc. The calls are now made first,
  into locals. Golden `call_before_read`; the generator writes each shape
  in every program. The benchmark crates' Rust is unchanged, so the
  published numbers stand.
- A call of another package's function returning an error was left out of
  the error lowering, and its `Result` destructured as a tuple, so the crate
  did not build; such calls are now lowered like calls in the same package.
  And `_, err = f()` kept as a value took the zero value's type from `_`,
  which has none. Golden `errors_packages`; the generator's lib package
  returns an error to every program.
- The borrow checker no longer rejects reading a value in one operand and
  handing it by value to a call in a later one, as in
  `fmt.Println(k, pass(k))` or `s + pass(s)` ("another operand changes k"):
  Go passes a copy, so the value is copied, with an `-explain` note.
  Values that share a slice stay rejected, since a copy would not share it.
  Golden `consume_copy`; the generator writes the pattern in every program.
- The coverage measurement excludes a function for reflect or unsafe only
  when it, or code of its own package, uses them; through another package
  (of its module, another module or the standard library) that is the
  feature "reflect or unsafe
  inside dependencies", and the function stays in the shares. With the
  cross-package rule, the exclusion had also taken every function reaching
  std internals that use unsafe (and, counted per module, every std function
  reaching runtime or internal packages), shrinking the denominator. The report gives
  both counts. "init and functions without a body" is now "functions without
  a body (assembly)".
- The subset checker no longer panics on a `String` or `Error` method
  declared without a body when its type is printed (found by FuzzCheck in
  CI; the method is rejected where it is declared).
- A slice literal without a type from its context (`fmt.Println([]int{1 <<
  40})`, `len([]int{-1 << 63})`) did not compile: its constants became
  Rust's default `i32`. The first constant element now carries its type when
  that default would be wrong (and for `float32`, which would be `f64`).
- A width on a slice (`%6v`, `%-5v`) pads each element, as Go does; the
  whole slice was padded. Golden program `fmt_verbs`.
- Float-to-integer conversions follow Go's compiler, not Rust's saturating
  `as`: on x86-64, NaN and values that do not fit give MinInt64 or MinInt32
  (gc's CVTTSD2SQ/CVTTSD2SL, measured against gc on amd64), and `uint8(f)`
  goes through int32 (`uint8(-2.5)` is 254); on arm64, Rust's conversion,
  which is gc's lowering there (not yet tested on arm64 hardware). Golden
  program `float_to_int`; the generator converts floats far out of range,
  NaN and the infinities in every program, from a random stream of its own.
- A float whose shortest digits have two equally close candidates prints
  the even one, as Go does (`1.2848823997124482e+15`); Rust's shortest
  digits took the upper one. Found by the new generator cases.
- A shift by a constant at least the operand's width (`x << 70`, `b >> 8` for
  a byte, `x <<= 64`) is Go's result, 0 or −1, with the operand still
  evaluated; it was Rust's shift, which does not implement it. Found by the
  new generator cases; golden program `shift_wide`.
- A match arm whose only statement is a `match` or a `loop`, or a switch with
  an init statement, is written the way rustfmt writes it, so the emitted
  crate stays rustfmt-clean. Crates allow `clippy::manual_rem_euclid` and
  `clippy::nonminimal_bool`, patterns the Go source controls.
- A left operand of <, <= or << that ends with a cast, such as
  i as i64 * i as i64 << 27, is parenthesized, so rustc no longer reads it
  as generic arguments; the defect-rate run found the case on the test
  server. -(-x) is no longer written as --x. The generator writes such
  shifts and comparisons in every loop nest, and gopatina-difftest makes
  tool paths absolute and stops when a tool cannot run.
- gopatina.org no longer scrolls sideways on phones: a Markdown link left in
  the evidence report showed a raw URL that could not wrap. Long words now
  wrap everywhere, a site test rejects Markdown links in the page, and CI
  loads the page in Chrome at 320 to 430 px and fails if it is wider than
  the screen (`scripts/site-overflow.mjs`).
- `make` with a length or capacity known only at run time now panics the
  way Go's makeslice does: "runtime error: makeslice: len out of range" for
  a negative length or one whose elements exceed Go's largest allocation,
  and "makeslice: cap out of range" for a negative or too-large capacity or
  a length above the capacity. Before, a negative length panicked with
  Rust's "capacity overflow", and a three-argument `make` with a run-time
  capacity ignored the capacity. Five golden programs cover the cases.
- Patina mode preserves writes to a slice forwarded through wrapper
  functions by borrowing the original backing array instead of copying it.
- Loops bounded by `len(s)` recheck the length on each iteration when a
  call can replace `s` through a pointer.
- Slice range loops diagnose header replacement through pointers, including
  forwarded calls, instead of reading replacement elements by mistake.
- The loader type-checks for the selected `GOARCH`, so a constant suffix
  bound that fits a 64-bit `int` but overflows a 32-bit one is rejected
  before emission.
- Go/Rust differential tests compare the first stderr line again (Go's stack
  trace after a panic is ignored), restoring checks on panic messages.
- Generated crates are now checked with `cargo clippy -D warnings` in CI.
  Empty `fmt.Println()` calls emit `println!()`, trailing-newline `print`/`eprint`
  forms emit `println!`/`eprintln!`, `s == ""`/`s != ""` comparisons emit
  `is_empty()`/`!is_empty()`, simple function-tail `let`+`return` patterns are
  folded into tail expressions, and `err.Error()` in a format argument emits the
  error directly; remaining Go-idiom patterns are covered by an explicit,
  commented allow list.
- Go 1.21's `min` and `max` built-ins are supported for integer arguments,
  including calls with more than two arguments; float calls are rejected
  rather than silently given Rust's different NaN behaviour.
- `strings.TrimSpace` now uses Go's `unicode.White_Space` code points written
  out in the generated runtime, instead of Rust's version-dependent
  `is_whitespace`, so the result no longer depends on the Rust Unicode tables.
- Emitted binaries print Go's own `runtime error:` text for divide-by-zero
  and index-out-of-bounds panics, while a program's `panic("...")` payload is
  marked at emission and printed verbatim. Differential tests compare the
  first stderr line without translating it.

### Changed

- The coverage measurement applies RFC 0011's boundary: a use of a provided
  package (bridged, `sync`, `sync/atomic`, `runtime`, `time`, `math/rand`,
  `sort`, `syscall` and the runtime's internal packages) is decided by its
  API, and nothing below it is reached; a use outside the supported API is
  "provided package: P". The provided packages' own functions are counted
  apart (`functions_provided`), outside the shares.
- The coverage measurement decides acceptance across packages, as patina
  builds do (RFC 0009): a function is accepted when everything it uses, in
  any measured package, is accepted, and inherits the features of what it
  uses; a use of an unmeasured package is "outside the measured corpus".
- The Go oracle of the differential tests (`gopatina-difftest` and the golden
  `TestDifferential`) is Go's reference build, `go build -gcflags=all=-N -l`.
  The generated tests found two programs whose optimized gc build (amd64;
  one from Go 1.22.0, the other from 1.26.0, both up to 1.27.1) prints results that contradict the specification and a
  hand calculation, while the reference build and GoPatina agree; both,
  reduced, are golden programs (`gc_inline_carry_1`, `_2`).
  `gopatina-difftest` still builds each batch optimized too and reports
  programs whose two Go builds disagree as `go_disagreements`, kept for
  review and not counted as defects.
- The coverage measurement counts functions that use `reflect` or `unsafe`
  apart, as excluded (out of scope), and leaves them out of every share;
  "struct features" is split into named non-struct types, type aliases,
  embedded and promoted fields, struct fields of type error, and anonymous
  structs and local types.
- `bench/run.sh` builds GoPatina's crate and the hand-written Rust with the
  same rustc by default (`--toolchain same`, `--rust stable`), the official
  comparison, and records the rustc that built each binary, read from the
  binary; `--toolchain shipped` keeps GoPatina's pinned rustc as a secondary
  comparison. `bench/asm-check.sh --toolchain T` compares under one
  toolchain. A test-server counter run had compared GoPatina built by rustc
  1.93.0 with Rust built by 1.100.0, which differ in codegen.
- gopatina.org is several pages, each at its own address, instead of one page
  of #anchors: overview, how it works, get started, evidence, benchmarks,
  correctness, roadmap and FAQ, each with its own title, description,
  canonical URL, Open Graph and Twitter tags, breadcrumb and structured data
  (TechArticle with citations, FAQPage, HowTo), and each also served as
  Markdown. The sitemap lists every page with its date; old #links are sent
  to their pages by a few lines of vanilla JavaScript on the home page;
  addresses have no trailing slash (301), and unknown ones get a 404 page.
  Pages are written once in Markdown and rendered by the server.
- The correctness bar is shown at 6σ confidence: 0 defects in at least
  6,099,050 differential tests (α = Φ(−6) ≈ 9.87 × 10⁻¹⁰), with 881,098 tests
  at 95% as the milestone on the way. `gopatina-difftest` reports both
  bounds, and docs/QUALITY.md gives the 62-part command for the full run.
- Documentation describes phase 1 as it is: docs/patina-subset.md specifies
  the subset and the ownership rules; README, EXAMPLES.md (real output for
  hello and ownership, counter still the design), the feature matrix,
  RFC 0001 and RFC 0003, docs/REPRODUCIBLE.md, docs/QUALITY.md,
  bench/README.md and BENCHMARKS.md are updated; everything else stays
  planned. No official benchmark figures are published.
- Every pull request, including the maintainer's own and automated ones, is
  merged only after the project lead's approval; CI for outside contributors
  runs after maintainer approval (GOVERNANCE.md, CONTRIBUTING.md).
- Benchmarks: pull requests get ratios only (GoPatina, Rust and Go in the
  same job, medians) on GitHub-hosted runners; official numbers come only
  from the maintainer's test server, on merged code, when the server is
  quiet (docs/QUALITY.md, bench/README.md).
- The site now matches the merged compiler: gopatina.org, `index.md`,
  `llms.txt`, the JSON-LD `dateModified` and the sitemap report 2026-11-18,
  75 golden programs, 24 rejection fixtures and 3 Go-only semantics cases,
  and describe the supported read-only `s[i:]` view.
- The slice-suffix RFC is numbered 0004 and records its merge in PR #26;
  `docs/patina-subset.md` links to the final number.

### Added

- Sentinel errors: package-level variables of an interface type that
  nothing changes (`var ErrNotFound = errors.New("not found")`), of any
  package of the program, as thread-local statics initialized in Go's
  order and read as copies (RFC 0012). Golden `sentinel_errors`; the
  generator's lib package returns a sentinel error to every program.
- The provided `errors` API: `fmt.Errorf` with `%w` (Go's
  `*fmt.wrapError`), `errors.Unwrap`, `errors.Is` and `errors.As`, as Go's
  errors package defines them, through wrapped errors and the program's
  `Unwrap() error` and `Is(error) bool` methods. Golden `errors_api`; the
  generator wraps and walks every program's errors.
- `error` as an ordinary interface value (RFC 0012): errors can be kept,
  stored in variables, fields and slices, passed, compared (`errors.New`'s
  values are equal only to themselves), printed, switched on and asserted;
  any type with an `Error() string` method is an error. Functions still
  return `Result<T, Iface>`; a call whose error is not checked at once is
  split into its values (zero when the error is not nil) and the error, and
  a return of an error that may be nil checks it. `%s` of a nil error
  prints `%!s(<nil>)` as Go does. Golden `errors_values`; the generator
  gives every program an error type and keeps its errors as values.
- Interfaces (RFC 0012): interface types (named, literal, embedded,
  `any`), conversions to them, method calls through them, `==` and `!=`,
  type assertions (`x.(T)` and `v, ok := x.(T)`), type switches, and
  `Println` and plain `%v`/`%T` of interface values, all with Go's panics
  (nil method call, failed assertion, missing method, uncomparable
  types). An interface value is one enum of the program's dynamic types
  behind a reference count; calls are `match`es. A reached type now keeps
  the methods an interface call may reach. Goldens `interfaces`,
  `interfaces_more`, `iface_packages` and five `iface_panic_*`; the
  generator declares an interface and two implementing types per program.
  `error` as an interface value comes next.
- `time.Duration`, the first provided type (RFC 0011): its constants,
  arithmetic, conversions and the methods `String`, `Nanoseconds`,
  `Microseconds`, `Milliseconds`, `Seconds`, `Minutes`, `Hours`,
  `Truncate`, `Round` and `Abs`, transcribed from Go's `time` package into
  `rt.rs`; fmt prints a Duration through `String`, also with widths and in
  slices. Patina builds no longer translate provided packages from source
  (the loader stops at `load.Provided`, as the coverage measurement does).
  Golden `time_duration`; the generator makes Durations from its integers
  in every unit and calls every method. The upstream gc issue is filed:
  golang/go#83292.
- Golden `gc_subqconst`: a gc miscompile found by the gate (program 26733,
  seed 1): optimized gc on amd64 (Go 1.21.0 to 1.27.1) drops `2^32` from
  `x + 2^31 + 2^31`. The cause is the amd64 rule
  `(SUBQconst (SUBQconst x [d]) [c])`, which negates `c` in 32 bits; a gc
  with that condition computed in 64 bits gets this program and
  `gc_inline_carry_1` to `_3` right, so all four are one bug.
  `docs/upstream/gc-inline-carry.md` (still a draft, not filed) now names
  the rule and a fix.
- Patina mode supports more of package `strings` (RFC 0011's first provided
  API): `ToLower`, `ToUpper`, `EqualFold`, `Split`, `Cut`, `TrimPrefix`,
  `TrimSuffix`, `ReplaceAll` and `IndexByte`, with Go's semantics: case
  mapping and folding by Go's own Unicode tables (generated from Go 1.27,
  Unicode 17.0.0), and `Split` and `ReplaceAll` with an empty separator as
  Go defines them. Golden `strings_provided`; the generator calls them on
  strings with the Unicode corners.
- Package-level variables (RFC 0010): read-only ones as static LazyLocks
  forced in Go's initialization order (go/types' InitOrder, per package,
  before the init functions); changed numbers, booleans and structs of them
  as thread-local Cells; changed strings as thread-local RefCells. In a
  simple statement that reads or changes one, calls run first, as gc runs
  them (the Go specification leaves that order open). Rejected with their
  own messages: changed slices, a variable's address, pointer-receiver
  methods on one, multi-value initializers, a read-only slice passed to a
  function that may change it, and conditions that read a changed variable
  and call a function. Golden programs package_vars, package_var_panic and
  gc_inline_carry_3 (the gc inline-carry bug across packages, found by the
  gate); rejection fixture global_slice_arg; generator cases in every
  program.
- The emitter keeps parentheses around arithmetic inside bitwise and shift
  operators (clippy's precedence lint), and crates allow
  clippy::match_single_binding (a Go switch with only a default case).
- Programs of several packages (RFC 0009): the packages a program imports,
  of its own module, other modules and the standard library, are translated
  from their Go source into Rust modules of the one crate (`pub(crate)`
  items, `crate::m::` paths), with only what `main` and the `init` functions
  reach; Go's package initialization order (the Go 1.21 rule, checked against
  `go run`). An unsupported declaration in another package is reported where
  it is, with how `main` reaches it. `math/bits` intrinsics (`Add64`, `Sub64`,
  `Mul64`, the rotates, zero counts, `Len`, `ReverseBytes`) give gc's results,
  also outside Go's documented domain (a carry counts as 1 when its low 32
  bits are not zero, on x86-64; measured). Golden program `multi_package`,
  rejection fixture `package_reach`; the generator adds a second package to
  every batch. The coverage measurement's cross-package rule is still to
  come.
- Shifts by a count known only at run time (`x << s`, `x >>= s`), with Go's
  0 or −1 at and past the width and Go's panic "runtime error: negative
  shift amount" (`rt::shl`, `rt::shr`); for `x op= s` every index of the
  place is computed once, before the count, as Go evaluates them.
- `init` functions, called in the order Go runs them (by file, then source
  order) before `main`. Golden program `var_shift_init`; the generator
  shifts values of every integer type by run-time counts, from a random
  stream of its own.
- Every integer type and `float32` in patina mode (RFC 0008): `int8`,
  `int16`, `uint`, `uint16`, `uint32`, `uint64`, `uintptr`, `float32`, as
  the Rust primitive of the same width, with wrapping arithmetic, signed
  division, shifts by constants up to past the width, and printing.
  Float-to-integer conversions as gc does them on amd64 (`rt::f64_to_u64`
  for 64-bit unsigned types); `float32` printed with Go's shortest 32-bit
  digits (`rt::Float32`); an out-of-range index of a 64-bit unsigned type
  panics with the index printed unsigned, as Go prints it (`rt::uindex`).
  Golden program `numeric_types`; the generator's named types take every
  integer width, and its float part adds `float32`.
- Coverage measurement: calls into the standard library's vendored packages,
  `%` verbs with the space flag, and struct types from another package are
  classified (they were "other").
- fmt: `%x` and `%X` of strings and `[]byte`; the `#` and `0` flags of `%x`
  and `%X` (Go zero-pads the digits, sign included, prefix not, for an
  integer, and the whole text for a string); slices printed through their
  elements' `String` or `Error` method (`rt::SliceMap`); `%x` of a value with
  a `String` method. The generator prints with these widths and flags in
  every program, from a random stream of its own.
- Named types and type aliases in patina mode (RFC 0007): `type T U` where
  `U` is a supported basic or slice type, or a struct of the package, and
  `type A = B`, as Rust type aliases of the underlying type; methods of named
  non-struct types as functions taking the receiver first; conversions
  between types with the same underlying type; `fmt` printing through
  `String` and `Error` (`Error` first, value method set only) where the
  method cannot panic. Golden programs `named_types` and `iota_consts`
  (formerly a rejection fixture); the generator adds a named integer type of
  a random width to every program, from its own random stream, so the rest
  of each program is unchanged.
- `cmd/gopatina-coverage` and `internal/coverage` measure how much real Go
  the patina subset accepts, with GoPatina's own subset checker: functions,
  lines, files and packages accepted over a pinned corpus (the standard
  library and 51 widely used modules, `docs/coverage-corpus.txt`), the
  features that keep code out, a roadmap ranking, 95% bootstrap intervals
  over packages, a standard-library and third-party split, and leave-one-
  module-out sensitivity. The method and its biases are in
  docs/COVERAGE.md; no results are published yet.
- The speed bar's first test-server results (2026-11-22) are published with
  their raw JSON: with one rustc (1.100.0) for both, GoPatina executes the
  same user instructions as hand-written Rust to within 0.001% on fib, sieve
  and suffix-sum, equivalent at 5σ and 6σ, so the bar is met on its primary
  metric; cycles and wall time, measured under load, are not decided, and
  the bar on run time waits for a quiet run.
- The first test-server run, a 100-run wall-time pilot (2026-11-22), is
  published with its raw JSON: all three benchmarks inconclusive, because the
  machine's wall-time noise (7% to 20%) is far wider than the ±0.5% margin.
- The two bars are stated with their hypotheses and exact formulas in
  docs/QUALITY.md, BENCHMARKS.md and the README: speed, equivalent to
  hand-written Rust within ±0.5% by a TOST equivalence test at 5σ; and
  correctness, the Six Sigma bar of at most 3.4 defects per million (4.5σ
  after the conventional 1.5σ shift), shown by 0 defects in at least 881,098
  differential tests. gopatina.org reports its evidence as a scientific
  report (hypotheses, method, apparatus, statistical analysis, results,
  threats to validity, reproducibility, references), with both results
  pending until the test server's runs.
- RFC 0006 loop specialization: when the emitter proves a loop nest's
  integer arithmetic cannot wrap, its induction variables are written as
  `usize`, as hand-written Rust would; when the proof needs one bound, such
  as `n <= 3037000500` for the sieve, the nest is written twice, a fast copy
  under that condition and the general copy otherwise (at most 60 lines, one
  condition). Go semantics are unchanged in both copies. `-explain` reports
  every specialization and refusal; eight golden programs cover the cases,
  including both sides of a threshold. The sieve benchmark's hot loop now
  compiles to the same instructions as the hand-written Rust.
- `internal/progen` generates the loop nests RFC 0006 specializes (counted,
  sieve-like and cube nests), now and then at exactly a threshold, and a
  panic case one past the cube threshold, so the defect-rate run tests both
  copies of a specialized nest.
- Every generated batch for `cmd/gopatina-difftest` ends with a panic case,
  a program that usually panics at run time (index out of range, division
  by zero, `make` out of range, explicit `panic`). It must match Go's
  output, first stderr line and exit status, so the defect-rate run now
  also tests Go's panic behaviour; the report counts panic cases and tests.
- `bench/run.sh --equiv` and `cmd/gopatina-bench` test whether GoPatina and
  hand-written Rust run at the same speed: interleaved ABBA runs and a two
  one-sided tests (TOST) equivalence test on the paired log ratios, at a
  ±0.5% margin and 5 sigma by default (both settings): "identical speed"
  means equivalent within ±0.5% at 5σ. Wall time by default;
  `--metric perf` also counts user instructions and cycles where the kernel
  allows. `bench/asm-check.sh` compares the machine code of the generated and
  hand-written Rust for the functions in each case's `hot` file.
- `cmd/gopatina-difftest` and `internal/progen` measure a defect rate:
  random phase 1 programs, built with `go build` and with `-patina`, run and
  compared program by program, many to a batch. With no defects, the report
  gives the 95% upper bound on the rate (881,098 clean programs bound it at
  3.4 per million). Generated batches are checked against the subset and
  borrow checker in the unit tests.
- A 2026-11-21 local equivalence run with CPU performance counters is
  published in BENCHMARKS.md and on gopatina.org: `fib` and `suffix-sum`
  execute the same instructions as hand-written Rust to within 0.0003%;
  `sieve` executes 1.302 times the instructions and is about 7% slower at
  5σ. The documents and the site define identical speed as equivalent within
  ±0.5% at 5σ.
- Patina supports `read(s[i:])` as a standalone call to a proven read-only
  function over `[]int`. The temporary view checks bounds before the
  callee runs, without copying the slice or inspecting its capacity.
- `FuzzCheck` and `FuzzBorrow` fuzz the subset gate and borrow checker from
  the accepted and rejected fixtures; CI runs each for 60 seconds alongside
  `FuzzEmit`.
- A `suffix-sum` benchmark case measures the read-only `s[i:]` path against
  hand-written Rust and plain Go.
- Two full local verification runs are published in BENCHMARKS.md with raw
  JSON and environment metadata. All three cases stayed within the 1.10
  GoPatina/Rust target (worst 1.043); they are explicitly not official
  test-server results.
- A 2026-11-21 local run on `6cb858f` and a machine-code comparison against
  the hand-written Rust are published in BENCHMARKS.md: `fib` and
  `suffix-sum` compile to the same hot loops, and `sieve` keeps one extra
  bounds check per inner-loop pass. The wall-clock ratios are stated as noisy,
  the goal is stated as identical speed, and the README and gopatina.org no
  longer say that no figures are published.
- docs/go-rust-disconnects.md lists every known place where Go's and Rust's
  semantics differ, with Go's exact behaviour, the options, a recommended fix
  and whether it blocks full compatibility; default mode is stated as a pure
  pass-through to the go tool.
- Patina front end: the package loader (go/build with the patina build tag,
  go/parser, go/types at the module's Go version) and the phase 1 subset
  checker, which rejects every construct outside the subset with a Go-style
  error naming it, and plans how errors lower to Result. Test programs under
  testdata/golden (accepted) and testdata/reject (rejected).
- Stage 3, the GoPatina borrow checker (internal/borrow): ownership kinds,
  parameter and receiver modes inferred across the call graph, liveness that
  moves a value on its last use and copies a string or a small struct when it
  is used again, checks for pointers and call arguments, an -explain report
  of every decision, and Go-style errors with both lines and a fix when no
  copy keeps Go's meaning.
- Stage 4, the Rust code generator (internal/emit): readable, rustfmt-clean
  Rust 2024, deterministic, as a cargo crate with Cargo.toml (the
  reproducible release profile), Cargo.lock, rust-toolchain.toml (Rust
  1.93.0), .cargo/config.toml (the target), src/rt.rs (only the runtime
  support used) and gopatina-manifest.sha256. Golden crates for every test
  program, and FuzzEmit, run by CI for 60 seconds.
- The patina modes in the CLI: gopatina build -emit-rust, build -patina, run
  -patina, build/run -patina=check, -explain, and gopatina verify, which
  checks deterministic emission and that the -patina binary is
  byte-identical to a plain cargo build --release --locked of the emitted
  crate. Differential tests (go build and -patina must print the same output
  and exit with the same status) for every golden program with Rust stable
  and the MSRV, and the benchmark harness bench/run.sh with the fib and sieve
  cases (ratios only).
- BENCHMARKS.md, the published home of official benchmark results, and a
  Benchmarks section in the README. Every official run is the full suite,
  published as bench/results/<date>-<version>.json with a summary through an
  approved pull request, then shown in BENCHMARKS.md, the README and the
  release notes. Nothing has been measured yet.
- EXAMPLES.md and runnable examples (examples/hello, examples/ownership,
  examples/counter) walking through default mode (real output), -emit-rust
  and -patina=check (planned output, labelled as design).
- gopatina.org one-page site (site/): a small Go server built with gopatina's
  default mode, with the page, its Markdown version, llms.txt, robots.txt,
  sitemap.xml and a health check; a Dockerfile and docker-compose.yml.
- Reproducible, checksum-verified builds: RFC 0001 section and
  docs/REPRODUCIBLE.md (deterministic emission, no hidden compiler, planned
  gopatina verify); example crates with pinned toolchains and lockfiles,
  scripts/repro-check.sh and a reproducible CI workflow that builds each crate
  twice and compares sha256.

## [0.0.1] - 2026-11-16

Phase 0: RFC and pass-through CLI. A pre-release; GoPatina is at the design
stage and is not a working compiler.

### Added

- Dual license (MIT OR Apache-2.0).
- Governance, contributing guide, code of conduct, security policy, issue
  templates and pull request template.
- Continuous integration: gofmt, go vet and go mod tidy checks, and
  go test -race on Ubuntu and macOS with the two supported Go releases.
- Pull request checks for a Conventional Commit title and DCO sign-off.
- Dependabot updates for Go modules and GitHub Actions.
- RFC 0001, the GoPatina design: three build modes (default, -emit-rust,
  -patina) with Rust underneath for both patina flags; the GoPatina borrow
  checker (ownership and lifetime inference, automatic fixes, -explain,
  gopatina fix, Go-style errors); deployment and portability (static
  binaries, cross-compilation, -patina=check, toolchain provisioning).
- The RFC template.
- gopatina CLI prototype: passes every command through to the go command
  unchanged; -patina and -emit-rust report that they are not implemented yet
  and exit 2.
- Quality policy (docs/QUALITY.md): 100% unit-test coverage, enforced in CI,
  and the test layers each phase requires.
- Performance policy: benchmarks against hand-written Rust on the latest
  stable toolchain, the planned suite and harness (bench/), and a bench
  workflow that skips measurement until there is generated code. No results
  exist yet.
- Full README: mission, status, the three build modes, roadmap, quality and
  performance policy, quick start, prior art.
- RFC 0002: concurrency in patina mode (goroutines on Tokio's multi-threaded
  runtime, automatic async colouring, data races as compile errors or
  automatic fixes).
- RFC 0003: toolchain compatibility and versioning; docs/compat.md, the Go
  feature-support matrix for patina mode; a supported-toolchains table in
  the README.
- Toolchains workflow: Go stable and oldstable with Rust stable and MSRV
  (required), Go tip and Rust beta and nightly canaries, weekly schedule;
  the Rust steps are placeholders until there is generated code.

[Unreleased]: https://github.com/pigfox/gopatina/compare/v0.0.1...HEAD
[0.0.1]: https://github.com/pigfox/gopatina/releases/tag/v0.0.1
