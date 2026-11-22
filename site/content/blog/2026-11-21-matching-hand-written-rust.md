---
title: Matching hand-written Rust: proven integer ranges
description: How GoPatina's generated Rust came to run the same instructions as hand-written Rust: loops with proven integer ranges, and a TOST test at 5σ.
---
# Matching hand-written Rust

GoPatina's speed goal is to be as fast as the same program written in Rust by hand: equivalent within ±0.5%, shown with two one-sided tests (TOST) at 5σ. The hypotheses, the statistics and every run are on the [evidence page](/evidence).

## A test that can say "the same"

A [benchmark harness](https://github.com/pigfox/gopatina/pull/33) runs the GoPatina binary and the hand-written Rust alternately, after warm-up runs, and tests whether the mean log ratio of their costs lies within the margin. It also compares the machine code of each benchmark's hot functions, which timing noise cannot affect.

## Proven integer ranges

Go integers wrap on overflow, so a faithful translation checks for wrapping where hand-written Rust does not. [RFC 0006](https://github.com/pigfox/gopatina/pull/37) lets GoPatina prove, for a loop, that its integer arithmetic cannot wrap, and then [write the loop](https://github.com/pigfox/gopatina/pull/40) the way a Rust programmer would. The [generated tests](https://github.com/pigfox/gopatina/pull/41) produce those loop nests, some at exactly the thresholds the proof depends on, so a wrong proof would show up as a wrong result.

## The result, with its conditions

On a laptop on November 21, `fib` and `suffix-sum` ran the same number of user instructions as hand-written Rust within the margin at 5σ, while cycles and wall time were inconclusive. On the test server, measured on user instructions (the primary metric) with one rustc for both binaries, all three benchmarks were [equivalent at 6σ](/evidence#results), and the 5σ run agrees. On run time, the bar is not yet decided: wall time on that machine was too noisy to decide anything within ±0.5%, as the [benchmarks page](/benchmarks) shows with every interval.
