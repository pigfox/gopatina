---
title: GoPatina phase 1: a Go subset with a borrow checker
description: GoPatina's first working pieces: a checker for the phase 1 subset of Go, and a borrow checker that decides ownership in Go terms before Rust.
---
# Phase 1: a Go subset and a borrow checker

GoPatina is a Go toolchain front end. Without a flag it is the installed Go toolchain, unchanged. With a patina flag it compiles a defined subset of Go through Rust, so the program runs with no garbage collector. [How it works](/how-it-works) describes the whole pipeline.

On November 16 the first two stages that make the patina modes possible were merged.

## The subset checker

The [subset checker](https://github.com/pigfox/gopatina/pull/18) loads a package with Go's own libraries and reports every construct outside the phase 1 subset, each with a Go-style error that names it. Nothing outside the subset is ever translated approximately: it is refused, with the reason. The subset itself is specified in [patina-subset.md](https://github.com/pigfox/gopatina/blob/main/docs/patina-subset.md).

## The borrow checker

The [borrow checker](https://github.com/pigfox/gopatina/pull/19) runs on the type-checked Go program before any Rust is written. It decides, in Go terms, who owns each value: what is moved, what is copied because Go's meaning needs a copy, and whether each parameter, receiver and pointer is a shared or an exclusive borrow. Every decision it makes on its own is reported with `-explain`, and when a program shares memory in a way no safe fix expresses, the error is in Go terms. Rust's own borrow checker then verifies the generated code.

## How GoPatina follows Go

The same day, [RFC 0003](https://github.com/pigfox/gopatina/pull/11) set how GoPatina follows Go and Rust toolchain releases and how it is versioned.

What phase 1 accepts, and what it does not yet, is on the [feature matrix](https://github.com/pigfox/gopatina/blob/main/docs/compat.md).
