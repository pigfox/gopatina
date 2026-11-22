---
title: Packages, variables and every integer type in GoPatina
description: Four RFCs in two days: named types, every integer width and float32, programs of several packages, and package-level variables in Go's order.
---
# Packages, variables and every integer type

Each sizeable feature of GoPatina starts as an RFC, is merged in reviewable pull requests, and is held to Go's behaviour by golden programs and by the generated differential tests. Four of them landed on November 21 and 22.

## Named types and aliases (RFC 0007)

`type Op int` and `type Path []int`, with methods, conversions and printing through `String` and `Error` as `fmt` does, and type aliases ([#55](https://github.com/pigfox/gopatina/pull/55)).

## Every integer type and float32 (RFC 0008)

`int8` to `uint64`, `uintptr` and `float32`, with Go's wrapping arithmetic in each width ([#60](https://github.com/pigfox/gopatina/pull/60)). Where Go leaves a choice to the compiler, GoPatina follows gc, measured: a float converted to an integer out of range gives [gc's result on amd64](https://github.com/pigfox/gopatina/pull/56), not Rust's saturated one.

## Programs of several packages (RFC 0009)

A program's own packages, those of other modules and much of the standard library are [translated from their Go source](https://github.com/pigfox/gopatina/pull/64), each into a Rust module, and initialized in Go's order. Only what `main` reaches is translated.

## Package-level variables (RFC 0010)

[Package-level variables](https://github.com/pigfox/gopatina/pull/69), initialized in Go's order. Go's specification leaves open whether, in one statement, a call that changes a variable runs before a read of it; gc runs the calls first, and GoPatina does the same, with the generator mixing such reads and calls in every program so the tests hold it to gc.

What each of these covers, and what is still refused, is on the [feature matrix](https://github.com/pigfox/gopatina/blob/main/docs/compat.md).
