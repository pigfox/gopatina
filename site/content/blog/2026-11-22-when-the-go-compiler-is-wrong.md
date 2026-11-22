---
title: When the oracle is wrong: a Go compiler miscompile
description: GoPatina's differential tests found programs where gc's optimized build is wrong: one amd64 rewrite rule, now reported as golang/go#83292.
---
# When the oracle is wrong

GoPatina checks itself against Go: every generated program is built with `go build` and with GoPatina, and both must print the same output and exit the same way. The method is on the [correctness page](/correctness).

## Two disagreements where GoPatina was right

The generated tests reported two programs where GoPatina's output differed from `go build`'s. Both times GoPatina was right: the optimized gc build on amd64 printed a result that contradicts the language specification and a hand calculation, while gc with optimization and inlining off agreed with GoPatina. A test whose oracle can be wrong measures the oracle too, so the oracle [became Go's reference build](https://github.com/pigfox/gopatina/pull/50), `go build -gcflags=all=-N -l`. Every batch is still built optimized as well, and a program whose two Go builds disagree is kept for review as a Go compiler disagreement: never counted as a GoPatina defect, never silently passed.

## One rule, three programs

A third disagreement, found on November 22, led to the cause of all three. Reduced to twenty lines, it adds 2³¹ to a variable twice; optimized gc drops 2³² from the sum. The cause is [one rewrite rule](https://github.com/pigfox/gopatina/pull/75) in gc's amd64 back end, which folds two subtractions of a constant and checks the result's range in 32 bits: for −2³¹ the check overflows and `x + 2³¹ + 2³¹` becomes `x + 0`. A gc built with only that check fixed gets all three programs right. It is wrong in every release tested, Go 1.21.0 to 1.27.1.

It is reported upstream as [golang/go#83292](https://github.com/golang/go/issues/83292). The reduced programs are golden tests in GoPatina, which must match the reference build.
