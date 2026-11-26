# GoPatina parity

190 programs: 138 tested, 52 skipped.

| of 138 tested | programs | rate |
|---|---|---|
| accepted by patina mode | 33 | 23.9% |
| compiled to Rust | 33 | 23.9% |
| output matches Go | 31 | 22.5% |
| clippy clean (of 33 linted) | 31 | 93.9% |

## Failure clusters, largest first

| programs | fingerprint | examples |
|---|---|---|
| 16 | uses the standard package math, which patina mode does not support yet | gobyexample/constants, gotour/basics/basic-types, gotour/basics/imports |
| 13 | uses the standard package unicode/utf8, which patina mode does not support yet | gobyexample/command-line-flags, gobyexample/command-line-subcommands, gobyexample/defer |
| 12 | a channel type (goroutines and channels are planned, RFC N) is not supported in patina mode yet | gobyexample/channel-buffering, gobyexample/channel-directions, gobyexample/channel-synchronization |
| 11 | uses the standard package internal/stringslite, which patina mode does not support yet | gobyexample/command-line-flags, gobyexample/command-line-subcommands, gobyexample/defer |
| 10 | a function literal (closure) is not supported in patina mode yet | gobyexample/atomic-counters, gobyexample/closures, gobyexample/directories |
| 10 | a function type is not supported in patina mode yet | gobyexample/closures, gobyexample/directories, gobyexample/mutexes |
| 9 | a map type is not supported in patina mode yet | gobyexample/enums, gobyexample/maps, gobyexample/mutexes |
| 9 | calling a function value is not supported in patina mode yet | gobyexample/closures, gobyexample/directories, gobyexample/recursion |
| 9 | defer is not supported in patina mode yet | gobyexample/defer, gobyexample/directories, gobyexample/exit |
| 9 | panic with a value that is not a string is not supported in patina mode yet | gobyexample/defer, gobyexample/directories, gobyexample/enums |
| 9 | the go statement (goroutines are planned, RFC N) is not supported in patina mode yet | gobyexample/channel-synchronization, gobyexample/channels, gobyexample/closing-channels |
| 9 | uses the standard package internal/strconv, which patina mode does not support yet | gobyexample/base64-encoding, gobyexample/command-line-flags, gobyexample/command-line-subcommands |
| 7 | a channel send (planned, RFC N) is not supported in patina mode yet | gobyexample/channel-buffering, gobyexample/channel-directions, gobyexample/channel-synchronization |
| 7 | make of chan string is not supported in patina mode yet | gobyexample/channel-buffering, gobyexample/channel-directions, gobyexample/channels |
| 7 | uses the standard package io/fs, which patina mode does not support yet | gobyexample/defer, gobyexample/directories, gobyexample/file-paths |
| 7 | uses the standard package iter, which patina mode does not support yet | gobyexample/command-line-flags, gobyexample/command-line-subcommands, gobyexample/maps |
| 6 | printing a value of type main._ is not supported in patina mode yet | gobyexample/structs, gotour/methods/indirection, gotour/moretypes/maps |
| 6 | uses the standard package internal/filepathlite, which patina mode does not support yet | gobyexample/defer, gobyexample/directories, gobyexample/file-paths |
| 6 | uses the standard package strconv, which patina mode does not support yet | gobyexample/command-line-flags, gobyexample/command-line-subcommands, gobyexample/number-parsing |
| 5 | _ is copied and kept, so its object has two owners | gobyexample/defer, gobyexample/reading-files, gobyexample/text-templates |
| 5 | the <- operator is not supported in patina mode yet | gobyexample/channel-buffering, gobyexample/channel-directions, gobyexample/channels |
| 5 | the cap built-in is not supported in patina mode yet | gobyexample/slices, gotour/moretypes/append, gotour/moretypes/making-slices |
| 5 | uses the standard package unicode, which patina mode does not support yet | gobyexample/command-line-flags, gobyexample/command-line-subcommands, gobyexample/regular-expressions |
| 4 | a slice expression (s[i:j]) is not supported in patina mode yet | gobyexample/command-line-arguments, gobyexample/command-line-subcommands, gobyexample/reading-files |
| 4 | an array type is not supported in patina mode yet | gobyexample/arrays, gotour/moretypes/array, gotour/moretypes/slices |
| 4 | format string: "S": the verb %q is not supported in patina mode yet | gotour/basics/zero, gotour/concurrency/exercise-web-crawler, gotour/methods/reader |
| 4 | make of chan int is not supported in patina mode yet | gobyexample/closing-channels, gotour/concurrency/buffered-channels, gotour/concurrency/range-and-close |
| 4 | os.TempDir is not supported in patina mode yet | gobyexample/defer, gobyexample/panic, gobyexample/reading-files |
| 4 | select (planned, RFC N) is not supported in patina mode yet | gobyexample/non-blocking-channel-operations, gobyexample/select, gobyexample/timeouts |
| 4 | type os.File is not supported in patina mode yet | gobyexample/defer, gobyexample/panic, gobyexample/reading-files |
| 4 | uses the standard package _, which patina mode does not support yet | gobyexample/defer, gobyexample/directories, gobyexample/panic |
| 4 | uses the standard package _/filepath, which patina mode does not support yet | gobyexample/defer, gobyexample/directories, gobyexample/panic |
| 4 | uses the standard package bufio, which patina mode does not support yet | gobyexample/line-filters, gobyexample/reading-files, gobyexample/writing-files |
| 4 | uses the standard package internal/unsafeheader, which patina mode does not support yet | gobyexample/command-line-flags, gobyexample/command-line-subcommands, gobyexample/text-templates |
| 4 | uses the standard package reflect, which patina mode does not support yet | gobyexample/command-line-flags, gobyexample/command-line-subcommands, gobyexample/text-templates |
| 3 | a channel receive as a statement is not supported in patina mode yet | gobyexample/channel-synchronization, gobyexample/closing-channels, gobyexample/timers |
| 3 | a slice expression (_[i:j]) is not supported in patina mode yet | gotour/moretypes/slice-bounds, gotour/moretypes/slice-len-cap, gotour/moretypes/slices |
| 3 | an anonymous struct type is not supported in patina mode yet | gobyexample/structs, gobyexample/text-templates, gotour/moretypes/slice-literals |
| 3 | converting a []byte to a string (patina strings must be valid UTF-N) is not supported in patina mode yet | gobyexample/base64-encoding, gobyexample/reading-files, gobyexample/xml |
| 3 | format string: "S": the verb %g is not supported in patina mode yet | gotour/basics/imports, gotour/flowcontrol/if-and-else, gotour/solutions/errors |
| 3 | make of chan bool is not supported in patina mode yet | gobyexample/channel-synchronization, gobyexample/closing-channels, gobyexample/non-blocking-channel-operations |
| 3 | make of map[string]int is not supported in patina mode yet | gobyexample/maps, gotour/concurrency/mutex-counter, gotour/moretypes/mutating-maps |
| 3 | os.Create is not supported in patina mode yet | gobyexample/defer, gobyexample/panic, gobyexample/writing-files |
| 3 | package sync is not supported in patina mode yet | gobyexample/atomic-counters, gobyexample/mutexes, gotour/concurrency/mutex-counter |
| 3 | printing a pointer of type *main._, which has no String or Error method (fmt prints its address) is not supported in patina mode yet | gobyexample/structs, gotour/methods/indirection, gotour/moretypes/struct-literals |
| 3 | the address of _ kept where its object would have more than one owner is not supported in patina mode yet | gobyexample/command-line-flags, gobyexample/xml, gotour/moretypes/pointers |
| 3 | the close built-in is not supported in patina mode yet | gobyexample/closing-channels, gobyexample/range-over-channels, gotour/concurrency/range-and-close |
| 3 | time.Now is not supported in patina mode yet | gobyexample/switch, gotour/flowcontrol/switch-evaluation-order, gotour/flowcontrol/switch-with-no-condition |
| 3 | time.Sleep is not supported in patina mode yet | gobyexample/channel-synchronization, gobyexample/timers, gotour/concurrency/mutex-counter |
| 3 | uses the standard package path, which patina mode does not support yet | gobyexample/file-paths, gobyexample/text-templates, gobyexample/writing-files |
| 2 | _ function literal (closure) is not supported in patina mode yet | gobyexample/range-over-iterators, gobyexample/sorting-by-functions |
| 2 | _ is converted to an interface while it is still used, so its object has two owners | gobyexample/reading-files, gobyexample/xml |
| 2 | _._ is copied and kept in _._, so its object has two owners | gobyexample/generics, gobyexample/range-over-iterators |
| 2 | _._._ is copied and kept in _._, so its object has two owners | gobyexample/generics, gobyexample/range-over-iterators |
| 2 | a generic function (Go N.N) is not supported in patina mode yet | gobyexample/generics, gotour/generics/index |
| 2 | a generic type (Go N.N) is not supported in patina mode yet | gobyexample/generics, gotour/generics/list |
| 2 | a type declared inside a function is not supported in patina mode yet | gobyexample/struct-embedding, gobyexample/xml |
| 2 | a value returned together with a non-nil error must be the zero value | gobyexample/custom-errors, gobyexample/errors |
| 2 | an instantiated generic function (Go N.N) is not supported in patina mode yet | gobyexample/custom-errors, gobyexample/generics |
| 2 | comparing a slice with nil is not supported in patina mode yet | gobyexample/slices, gotour/moretypes/nil-slices |
| 2 | converting a pointer to a type other than a named type of the program to an interface is not supported in patina mode yet | gobyexample/line-filters, gobyexample/reading-files |
| 2 | os.Args is not supported in patina mode yet | gobyexample/command-line-arguments, gobyexample/command-line-subcommands |
| 2 | output: exit status differs | gotour/methods/interface-values, gotour/methods/interface-values-with-nil |
| 2 | printing _ value of type [N]int is not supported in patina mode yet | gobyexample/arrays, gotour/moretypes/array |
| 2 | printing _ value of type [N]string is not supported in patina mode yet | gotour/moretypes/array, gotour/moretypes/slices-pointers |
| 2 | printing a value of type (float64, error) is not supported in patina mode yet | gotour/methods/exercise-errors, gotour/solutions/errors |
| 2 | printing a value of type map[string]int is not supported in patina mode yet | gobyexample/maps, gobyexample/mutexes |
| 2 | printing a value of type map[string]main._ is not supported in patina mode yet | gotour/moretypes/map-literals, gotour/moretypes/map-literals-continued |
| 2 | strings.NewReader is not supported in patina mode yet | gotour/methods/reader, gotour/solutions/rot13 |
| 2 | the delete built-in is not supported in patina mode yet | gobyexample/maps, gotour/moretypes/mutating-maps |
| 2 | the results of _ used here other than as v..., err := _(...) or return _(...) | gotour/methods/exercise-errors, gotour/solutions/errors |
| 2 | type strings.Reader is not supported in patina mode yet | gotour/methods/reader, gotour/solutions/rot13 |
| 2 | type sync.Mutex is not supported in patina mode yet | gobyexample/mutexes, gotour/concurrency/mutex-counter |
| 2 | type sync.WaitGroup is not supported in patina mode yet | gobyexample/atomic-counters, gobyexample/mutexes |
| 2 | type time.Time is not supported in patina mode yet | gobyexample/switch, gotour/flowcontrol/switch-with-no-condition |
| 2 | uses the standard package cmp, which patina mode does not support yet | gobyexample/sorting-by-functions, gobyexample/text-templates |
| 2 | uses the standard package flag, which patina mode does not support yet | gobyexample/command-line-flags, gobyexample/command-line-subcommands |
| 2 | uses the standard package path/filepath, which patina mode does not support yet | gobyexample/file-paths, gobyexample/writing-files |
| 2 | using the package-level variable Args this way (a copy of a slice shares its elements) is not supported in patina mode yet | gobyexample/command-line-arguments, gobyexample/command-line-subcommands |
| 1 | _ bare return in _ function that returns an error | gotour/solutions/rot13 |
| 1 | _ function type is not supported in patina mode yet | gobyexample/sorting-by-functions |
| 1 | _ generic type (Go N.N) is not supported in patina mode yet | gobyexample/range-over-iterators |
| 1 | _ method on _ generic type (Go N.N) is not supported in patina mode yet | gobyexample/range-over-iterators |
| 1 | _ slice expression (_[i:j]) is not supported in patina mode yet | gotour/moretypes/making-slices |
| 1 | _ slice expression (s[i:j]) is not supported in patina mode yet | gotour/moretypes/slices-pointers |
| 1 | _ type declared inside _ function is not supported in patina mode yet | gobyexample/sorting-by-functions |
| 1 | a *_ with more than one owner (a package-level variable holds it) is not supported in patina mode yet | gotour/moretypes/struct-literals |
| 1 | a *_ with more than one owner (a slice, map, array, channel or struct value holds it) is not supported in patina mode yet | gotour/concurrency/exercise-web-crawler |
| 1 | a call with ... arguments is not supported in patina mode yet | gobyexample/variadic-functions |
| 1 | a complex number is not supported in patina mode yet | gotour/basics/basic-types |
| 1 | a method on a generic type (Go N.N) is not supported in patina mode yet | gobyexample/generics |
| 1 | a package-level variable of type map[string]main._ that the program changes is not supported in patina mode yet | gotour/moretypes/maps |
| 1 | a pointer here (RFC N supports pointers in variables, parameters, results and fields) is not supported in patina mode yet | gotour/moretypes/struct-literals |
| 1 | a promoted field is not supported in patina mode yet | gobyexample/struct-embedding |
| 1 | a renamed or dot import is not supported in patina mode yet | gobyexample/string-functions |
| 1 | a slice expression (_[_:_]) is not supported in patina mode yet | gobyexample/slices |
| 1 | a slice expression (_[_:j]) is not supported in patina mode yet | gobyexample/strings-and-runes |
| 1 | a variadic function is not supported in patina mode yet | gobyexample/variadic-functions |
| 1 | an embedded field is not supported in patina mode yet | gobyexample/struct-embedding |
| 1 | bytes.ToUpper is not supported _ patina mode yet | gobyexample/regular-expressions |
| 1 | calling a pointer-receiver method on a package-level variable is not supported in patina mode yet | gobyexample/base64-encoding |
| 1 | clippy: error: this loop never actually loops | gobyexample/for |
| 1 | clippy: error: unnecessary use of `_` | gotour/moretypes/range-continued |
| 1 | converting _ pointer to _ type other than _ named type of the program to an interface is not supported in patina mode yet | gotour/solutions/rot13 |
| 1 | converting a []byte to a string (patina strings must be valid UTF-N) is not supported _ patina mode yet | gobyexample/regular-expressions |
| 1 | converting to an interface a value whose _ method may panic is not supported in patina mode yet | gotour/solutions/errors |
| 1 | format string: "S": the verb %#U is not supported in patina mode yet | gobyexample/strings-and-runes |
| 1 | io.Copy is not supported in patina mode yet | gotour/solutions/rot13 |
| 1 | io.EOF is not supported in patina mode yet | gotour/methods/reader |
| 1 | io.ReadAtLeast is not supported in patina mode yet | gobyexample/reading-files |
| 1 | make of map[string]main._ is not supported in patina mode yet | gotour/moretypes/maps |
| 1 | os.Chdir is not supported in patina mode yet | gobyexample/directories |
| 1 | os.Environ is not supported in patina mode yet | gobyexample/environment-variables |
| 1 | os.Getenv is not supported in patina mode yet | gobyexample/environment-variables |
| 1 | os.Mkdir is not supported in patina mode yet | gobyexample/directories |
| 1 | os.MkdirAll is not supported in patina mode yet | gobyexample/directories |
| 1 | os.Open is not supported in patina mode yet | gobyexample/reading-files |
| 1 | os.ReadDir is not supported in patina mode yet | gobyexample/directories |
| 1 | os.ReadFile is not supported in patina mode yet | gobyexample/reading-files |
| 1 | os.Setenv is not supported in patina mode yet | gobyexample/environment-variables |
| 1 | os.Stdin is converted to an interface, so its object has two owners | gobyexample/line-filters |
| 1 | os.Stdin is not supported in patina mode yet | gobyexample/line-filters |
| 1 | os.WriteFile is not supported in patina mode yet | gobyexample/writing-files |
| 1 | package encoding/base64 is not supported in patina mode yet | gobyexample/base64-encoding |
| 1 | package runtime is not supported in patina mode yet | gotour/flowcontrol/switch |
| 1 | package sync/atomic is not supported in patina mode yet | gobyexample/atomic-counters |
| 1 | printing _ value of type [N][N]int is not supported in patina mode yet | gobyexample/arrays |
| 1 | printing _ value of type []main._ is not supported in patina mode yet | gobyexample/sorting-by-functions |
| 1 | printing a value of type (_ int, _ int) is not supported in patina mode yet | gotour/basics/named-results |
| 1 | printing a value of type (r uint32, g uint32, b uint32, a uint32) is not supported in patina mode yet | gotour/methods/images |
| 1 | printing a value of type [][]int is not supported _ patina mode yet | gobyexample/regular-expressions |
| 1 | printing a value of type [][]int is not supported in patina mode yet | gobyexample/slices |
| 1 | printing a value of type []struct{_ int; _ bool} is not supported in patina mode yet | gotour/moretypes/slice-literals |
| 1 | printing a value of type struct{_ string; _ bool} is not supported in patina mode yet | gobyexample/structs |
| 1 | printing a value whose String method may panic is not supported in patina mode yet | gotour/methods/images |
| 1 | printing a value whose _ method may panic is not supported in patina mode yet | gobyexample/enums |
| 1 | printing an interface value in a program that converts *strconv.NumError to one: printing a value whose Error method may panic is not supported in patina mode yet | gobyexample/number-parsing |
| 1 | range over _ function (Go N.N) is not supported in patina mode yet | gobyexample/range-over-iterators |
| 1 | range over a channel (chan int) is not supported in patina mode yet | gotour/concurrency/range-and-close |
| 1 | range over a channel (chan string) is not supported in patina mode yet | gobyexample/range-over-channels |
| 1 | slices.Collect is not supported in patina mode yet | gobyexample/range-over-iterators |
| 1 | slices.Equal is not supported in patina mode yet | gobyexample/slices |
| 1 | slices.IsSorted is not supported in patina mode yet | gobyexample/sorting |
| 1 | slices.Sort is not supported in patina mode yet | gobyexample/sorting |
| 1 | slices.SortFunc is not supported in patina mode yet | gobyexample/sorting-by-functions |
| 1 | strings.SplitN is not supported in patina mode yet | gobyexample/environment-variables |
| 1 | strings.SplitSeq is not supported in patina mode yet | gobyexample/range-over-iterators |
| 1 | template.Must(_.Parse("S")) lends back a pointer its caller owns, and it is kept in _, so the object has two owners | gobyexample/text-templates |
| 1 | template.Must(template.New(_).Parse(_)) lends back a pointer its caller owns, and it is kept, so the object has two owners | gobyexample/text-templates |
| 1 | the address of _ is kept while _ is still used by name, so the object has two owners | gotour/moretypes/pointers |
| 1 | the clear built-in (Go N.N) is not supported in patina mode yet | gobyexample/maps |
| 1 | the copy built-in is not supported in patina mode yet | gobyexample/slices |
| 1 | the format %+_ with a value of type *main._ is not supported in patina mode yet | gotour/methods/methods-with-pointer-receivers |
| 1 | the format %T with a value of type bool is not supported in patina mode yet | gotour/basics/basic-types |
| 1 | the format %T with a value of type complex128 is not supported in patina mode yet | gotour/basics/basic-types |
| 1 | the format %T with a value of type int is not supported in patina mode yet | gotour/basics/type-inference |
| 1 | the format %T with a value of type uint64 is not supported in patina mode yet | gotour/basics/basic-types |
| 1 | the format %v with a value of type complex128 is not supported in patina mode yet | gotour/basics/basic-types |
| 1 | the results of Parse used here other than as v..., _ := Parse(...) or return Parse(...) | gobyexample/text-templates |
| 1 | time.NewTimer is not supported in patina mode yet | gobyexample/timers |
| 1 | type _: a map type is not supported in patina mode yet | gotour/concurrency/exercise-web-crawler |
| 1 | type _: an interface method with an error result is not supported in patina mode yet | gotour/concurrency/exercise-web-crawler |
| 1 | type complex128 is not supported in patina mode yet | gotour/basics/basic-types |
| 1 | type io.Reader is not supported in patina mode yet | gotour/solutions/rot13 |
| 1 | type sync/atomic.Uint64 is not supported in patina mode yet | gobyexample/atomic-counters |
| 1 | type time.Timer is not supported in patina mode yet | gobyexample/timers |
| 1 | type time.Weekday is not supported in patina mode yet | gotour/flowcontrol/switch-evaluation-order |
| 1 | uses the standard package crypto, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package crypto/fips140, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package crypto/internal/fips140, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package crypto/internal/fips140/aes, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package crypto/internal/fips140/aes/gcm, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package crypto/internal/fips140/alias, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package crypto/internal/fips140/check, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package crypto/internal/fips140/drbg, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package crypto/internal/fips140/hmac, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package crypto/internal/fips140/sha256, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package crypto/internal/fips140/sha3, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package crypto/internal/fips140/sha512, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package crypto/internal/fips140/subtle, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package crypto/internal/fips140deps/cpu, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package crypto/internal/fips140deps/godebug, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package crypto/internal/impl, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package crypto/sha256, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package encoding, which patina mode does not support yet | gobyexample/xml |
| 1 | uses the standard package encoding/base64, which patina mode does not support yet | gobyexample/base64-encoding |
| 1 | uses the standard package encoding/xml, which patina mode does not support yet | gobyexample/xml |
| 1 | uses the standard package hash, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package image, which patina mode does not support yet | gotour/methods/images |
| 1 | uses the standard package image/color, which patina mode does not support yet | gotour/methods/images |
| 1 | uses the standard package internal/byteorder, which patina mode does not support yet | gobyexample/sha256-hashes |
| 1 | uses the standard package internal/fmtsort, which patina mode does not support yet | gobyexample/text-templates |
| 1 | uses the standard package maps, which patina mode does not support yet | gobyexample/maps |
| 1 | uses the standard package math/cmplx, which patina mode does not support yet | gotour/basics/basic-types |
| 1 | uses the standard package net/netip, which patina mode does not support yet | gobyexample/text-templates |
| 1 | uses the standard package net/url, which patina mode does not support yet | gobyexample/text-templates |
| 1 | uses the standard package regexp, which patina mode does not support yet | gobyexample/regular-expressions |
| 1 | uses the standard package regexp/syntax, which patina mode does not support yet | gobyexample/regular-expressions |
| 1 | uses the standard package text/template, which patina mode does not support yet | gobyexample/text-templates |
| 1 | uses the standard package text/template/parse, which patina mode does not support yet | gobyexample/text-templates |
| 1 | uses the standard package unique, which patina mode does not support yet | gobyexample/text-templates |
| 1 | using fmt.Println as a value is not supported in patina mode yet | gobyexample/string-functions |
| 1 | using function _ as a value is not supported in patina mode yet | gobyexample/directories |
| 1 | using math.Pow as a value is not supported in patina mode yet | gotour/moretypes/function-values |

## Skipped (not counted)

| programs | reason |
|---|---|
| 6 | does not build with Go |
| 1 | does not finish within 10s with Go |
| 5 | goroutine scheduling order (skip.txt) |
| 1 | goroutine scheduling order (two sums received as they finish) (skip.txt) |
| 4 | imports golang.org/x/tour/pic, not in the standard library |
| 2 | imports golang.org/x/tour/reader, not in the standard library |
| 3 | imports golang.org/x/tour/tree, not in the standard library |
| 2 | imports golang.org/x/tour/wc, not in the standard library |
| 1 | log timestamps (skip.txt) |
| 4 | map iteration order (skip.txt) |
| 2 | pointer addresses (skip.txt) |
| 3 | random numbers (skip.txt) |
| 1 | runs other programs (os/exec) |
| 1 | temporary file names (skip.txt) |
| 8 | the clock (skip.txt) |
| 1 | the output of date (skip.txt) |
| 2 | uses the network (net) |
| 4 | uses the network (net/http) |
| 1 | waits for signals (os/signal) |
