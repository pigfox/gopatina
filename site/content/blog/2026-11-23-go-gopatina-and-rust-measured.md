---
title: Go, GoPatina and Rust, measured: where Go wins
description: The first three-way benchmark: GoPatina matches hand-written Rust on three programs, and plain Go beats both on the two that allocate pointers.
---
# Go, GoPatina and Rust, measured

Every benchmark program is now built three ways: with `gopatina build -patina`, with plain `go build`, and as the same program written by hand in idiomatic Rust. On November 23 the test server ran all three on all five benchmarks, interleaved, 100 rounds each, with both Rust binaries built by the same rustc. The [raw results and run notes](https://github.com/pigfox/gopatina/blob/main/bench/results/2026-11-23-server-d207c32-three-arm.md) have every interval; the [benchmarks page](/benchmarks#results) has the tables.

## Where GoPatina matches Rust

On `fib`, `sieve` and `suffix-sum`, GoPatina executes the same user instructions as the hand-written Rust, to within 0.001%. `fib` is a recursive `fib(42)`, nearly 867 million calls and no allocation, so it compares code generation: plain Go executes 1.48 times the instructions and takes 1.72 times Rust's wall time, and GoPatina takes [0.53 times Go's](/benchmarks#results). On `sieve` and `suffix-sum` no two of the three can be told apart on wall time at this precision.

## Where Go wins

The two pointer benchmarks go the other way, and plain Go is the fastest of the three on both:

| Benchmark | GoPatina / Rust, wall time | GoPatina / Go, wall time | Go / Rust, wall time |
| --- | --- | --- | --- |
| `binary-trees` | 1.50 [1.42, 1.57] | 1.59 [1.52, 1.66] | 0.94 [0.90, 0.98] |
| `cycles` | 2.00 [1.89, 2.11] | 2.70 [2.53, 2.88] | 0.74 [0.70, 0.78] |

Each ratio has its 5σ interval; every interval lies wholly away from 1, so each difference is real at that confidence ([run notes](https://github.com/pigfox/gopatina/blob/main/bench/results/2026-11-23-server-d207c32-three-arm.md)).

`binary-trees` allocates and frees millions of small trees. GoPatina represents a pointer that escapes as a shared, reference-counted cell, `Rc<RefCell<T>>`, where the hand-written Rust uses a `Box`. For this benchmark's node, a `Box` allocates 16 bytes, which the C library's `malloc` serves from a 32-byte chunk; the cell allocates 40 bytes (the node plus two reference counts and a borrow flag), served from a 48-byte chunk. Measured on a workstation, the hand-written Rust takes 6.00 s with `Rc<RefCell>` against 5.03 s with `Box`, and 5.81 s with the `Box` node padded to the cell's 40 bytes, so size alone costs most of the difference ([RFC 0016](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0016-owned-pointers-and-cycle-collection.md#motivation)). Go's collector, meanwhile, does its work concurrently with the program.

`cycles` builds doubly linked lists and drops them. GoPatina executes about as many instructions as the Rust, but never frees a list, because a reference count cannot reclaim a cycle ([RFC 0013](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0013-pointers.md#drawbacks)'s known limitation). Its peak memory is 185 MiB against Rust's 3 MiB and Go's 7.6 MiB, and the time goes to that fresh memory. The hand-written Rust, changed to leak every list in the same way, matches GoPatina's memory and time ([RFC 0016](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0016-owned-pointers-and-cycle-collection.md#motivation)).

## What comes next

[RFC 0016](https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0016-owned-pointers-and-cycle-collection.md), accepted on November 23, makes two changes, both in safe Rust. A pointer type that is never shared becomes a `Box`, which is exact for a tree. The types that can form cycles get a cycle collector, which frees what no pointer can reach. The next test-server run will show whether the two cases close.
