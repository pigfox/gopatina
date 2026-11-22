# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Until 1.0.0, minor versions may contain breaking changes.

## [Unreleased]

### Fixed

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
