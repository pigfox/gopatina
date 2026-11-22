---
title: Interfaces and errors as values, compiled to Rust enums
description: GoPatina now compiles Go interfaces and errors as values: one Rust enum of a program's dynamic types, Go's panics, and the errors package's API.
---
# Interfaces and errors as values

GoPatina compiles a whole program at once, so it knows every type that is ever stored in an interface. That made a simple design possible for [RFC 0012](https://github.com/pigfox/gopatina/pull/73): an interface value is one Rust enum of those types, and a method call through an interface is a `match` on it. There are no trait objects, no `Any` and no `unsafe`, and a program that uses no interface pays nothing.

## Interfaces

[Interfaces](https://github.com/pigfox/gopatina/pull/78) are supported with conversions, method calls, comparisons, type assertions, type switches and printing, and with Go's exact panics: a method call on a nil interface, a failed assertion, a missing method, and comparing values of a type that cannot be compared.

## Errors as values

`error` is now an [ordinary interface value](https://github.com/pigfox/gopatina/pull/79): kept, stored, compared, printed and switched on, while functions still return Rust's `Result`. The [errors API](https://github.com/pigfox/gopatina/pull/81) follows Go's errors package: `fmt.Errorf` with `%w`, `errors.Unwrap`, `errors.Is` and `errors.As`. [Sentinel errors](https://github.com/pigfox/gopatina/pull/83) such as `var ErrNotFound = errors.New("not found")` work across packages.

## Where translation stops

[RFC 0011](https://github.com/pigfox/gopatina/pull/71) draws the line between the standard library GoPatina translates from source and the packages it provides in Rust with Go's behaviour. The first provided APIs are [the common `strings` functions](https://github.com/pigfox/gopatina/pull/74), whose case mapping uses Go's own Unicode tables (Rust's differ), checked against Go on every code point, and [`time.Duration`](https://github.com/pigfox/gopatina/pull/76).

## Defects found on the way

Writing the tests for these features turned up defects, and each was fixed in its own pull request with golden programs and generator cases: a false rejection by the borrow checker ([#80](https://github.com/pigfox/gopatina/pull/80)), calls of another package's error functions that did not build ([#82](https://github.com/pigfox/gopatina/pull/82)), and the translation of `time` from source ([#76](https://github.com/pigfox/gopatina/pull/76)). The [correctness page](/correctness#defects) lists every defect found so far.
