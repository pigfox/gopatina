# How much real Go the patina subset accepts: the method

GoPatina's patina modes accept a defined subset of Go
([patina-subset.md](patina-subset.md)) and translate it one to one. This
document describes how `cmd/gopatina-coverage` measures how much real Go
code falls inside that subset, and what the number does and does not mean.
It holds no results: they are reviewed before anything is published.

## What is measured

**The corpus** ([coverage-corpus.txt](coverage-corpus.txt)): the standard
library of the Go toolchain that runs the measurement, and 51 widely used
open-source modules chosen to span what Go is used for. `gopatina-coverage
resolve` pins each module to a version and records its `go.sum` hash and,
where the Go proxy knows it, its commit; the lock file it writes *is* the
corpus, and a measurement names it.

**The checker**: GoPatina's own subset checker (`internal/subset`), the one
that decides what `-patina` accepts. Nothing is translated or built with
Rust, so the measurement is of the subset's rules, not of the translator.

**The counts**, each with a 95% interval:

- **Functions and methods accepted.** A function is accepted when the
  checker reports nothing inside it (signature and body), and every
  declaration it uses (a type, a function, a method, a variable or a
  constant), of its own package or of any other package of the corpus, is
  accepted too, as a patina build decides (RFC 0009): a fixed point over the
  whole corpus. So a function that is clean itself but takes a struct with a
  map field, or calls a function in another package that uses `defer`, is
  not accepted, and it inherits that declaration's features. A use of a
  package the measurement did not measure (skipped, or outside the corpus)
  is the feature "outside the measured corpus". The bridged packages (`fmt`
  and the phase 1 functions of `strings`, `errors`, `os`, `io`, `slices`,
  `bytes`) and the `math/bits` intrinsics are decided by the checker's own
  rules. A use of `reflect` or `unsafe` excludes only the functions of its
  own package that reach it (see below).
- **Lines in accepted functions**: lines from `func` to the closing brace,
  so long functions weigh more.
- **Files accepted**: files with no diagnostic at all.
- **Packages wholly accepted**: every declaration, function and import
  accepted.

**Provided packages (RFC 0011).** GoPatina provides some standard-library
packages in Rust instead of translating them: the bridged ones (`fmt`,
`strings`, `bytes`, `errors`, `io`, `os`), `sync`, `sync/atomic`, `runtime`,
`time`, `math/rand`, `math/rand/v2`, `sort`, `syscall` and the runtime's
internal packages below them (`load.Provided` lists them). The measurement
applies that boundary: a use of a provided package is decided by its
supported API, and nothing below the API is reached; a use outside it is the
feature "provided package: P", so the report shows what each provided
package would unlock. The provided packages' own functions are measured but
counted apart (`functions_provided`) and left out of every share: they are
not translated, so they are not Go that GoPatina compiles.

**Out of scope: reflect and unsafe.** Programs that use package `reflect`
or `unsafe` themselves are out of scope (the project lead, 2026-11-21). A
function that uses either, itself or through code of its own package that it
reaches, is counted apart, as "excluded", and left out of every share: 100%
means 100% of the functions that do not. A function that reaches `reflect`
or `unsafe` only through another package, of its own module, another module
or the standard library
(`strings.Builder`, `strconv` and `sync` use `unsafe` inside), stays in the
shares: what it uses is a surface GoPatina has to provide, by bridging or
translation, so it is rejected with the feature "reflect or unsafe inside
dependencies", not excluded. The report gives both counts beside every
share (`functions_excluded_reflect_unsafe_direct`,
`functions_reaching_reflect_unsafe_via_dependencies`). Functions without a
Go body (assembly, mostly in the standard library) are counted the same
way: rejected, with the feature "functions without a body (assembly)", a
surface to provide with Rust equivalents, as RFC 0009's intrinsics do for
`math/bits`. The diagnostic names the function (`math.archSqrt`), every
rejected function records which such functions it reaches, and the report
ranks them (`assembly_needed`): how many rejected functions reach each, and
how many would be accepted if only it were provided.

**The reasons.** Each diagnostic is assigned a feature (maps, closures and
function values, interfaces, goroutines and channels, defer, generics,
other integer and float types, pointers, slicing, standard-library calls,
calls into other modules, named non-struct types such as `type Kind int`,
type aliases, embedded fields, and so on; `internal/coverage` lists the
rules).
A rejected function carries the features of its own diagnostics and of
everything it depends on. The report gives, per feature:

- how many rejected functions need it (among others);
- how many would be accepted if *only* that feature were added;
- a greedy roadmap: add the feature that lets the most functions in, then
  the next, with the share accepted after each step.

Diagnostics the categories do not cover are counted as "other" and listed
by message in each package's record, so the categories can be checked.

## What counts as code

- **Included:** every non-test `.go` file of every package, as `go list`
  selects them for the measuring machine's `GOOS` and `GOARCH`, without the
  `patina` build tag.
- **Excluded, and counted separately:** test files (`_test.go`); generated
  files (a `// Code generated ... DO NOT EDIT.` line before the package
  clause), whose declarations still count for the functions that use them;
  vendored packages; packages with cgo files (`import "C"`), which cannot be
  type-checked without cgo; packages that fail to load or type-check. Every
  skipped package is listed with its reason.
- **Libraries measured as packages of a program.** The rule "only package
  main" and the need for `func main` are left out, since they reject every
  library alike; each package is measured as a package of a program that
  uses it, with the packages it imports as the program's other packages.

## The quality of the number

- **Bootstrap intervals.** Packages are drawn with replacement, 2,000 times
  by default with a fixed seed, and the 2.5th and 97.5th percentiles of each
  share are reported. Packages, not functions, are the unit drawn, so a few
  large packages cannot make the interval look narrower than it is.
- **Groups.** Every count is reported for the whole corpus, for the
  standard library alone and for the third-party modules alone, and the
  share of functions accepted for each module.
- **Sensitivity to the corpus.** The share of functions accepted is
  recomputed with each module left out in turn; the lowest and highest
  values show how much any one module moves the number.

## Known biases

- **An upper bound for what patina mode builds.** Only the subset checker
  runs; GoPatina's borrow checker, which runs after it, can reject more, and
  the fixed point counts a function accepted when everything it uses is
  accepted, not whether the program around it would be.
- **Libraries checked as programs.** The two adaptations above measure
  whether code could be expressed, not whether it can be built today.
- **The corpus.** Chosen by judgment, not drawn at random: popular, mature
  modules, which may use more advanced Go than typical application code.
  The standard library is large, and low-level parts of it (`runtime`,
  `syscall`, `unsafe`) are far from any subset.
- **One platform.** Files are selected for one `GOOS` and `GOARCH`.
- **Dependent counts.** Functions of one package are not independent draws;
  the bootstrap over packages accounts for that within the corpus, not for
  the choice of corpus.

## Running it

On the test server, not a workstation: it builds export data for every
package of the corpus. From the repository root:

```sh
go build -o gopatina-coverage ./cmd/gopatina-coverage
./gopatina-coverage resolve docs/coverage-corpus.txt > coverage.lock
./gopatina-coverage measure -boot 2000 -seed 1 coverage.lock > coverage.json 2> coverage.log
```

`coverage.json` holds the corpus as locked, the Go version and platform,
the summary, every package and function, and every skipped package.
`coverage.log` has one line per module and the headline share with its
interval.
