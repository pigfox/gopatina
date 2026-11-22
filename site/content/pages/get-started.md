# Get started: build Go through Rust with GoPatina

GoPatina needs Go 1.25 or later; `-patina` and `gopatina verify` also need a Rust toolchain. Phase 1 is not released yet, so install it from `main`. The patina commands work on programs inside the [phase 1 subset](/how-it-works#subset).

## Steps {#steps}

1. Install the command: `go install github.com/pigfox/gopatina/cmd/gopatina@main`.
2. Build as plain Go, exactly like `go build`: `gopatina build ./...`.
3. Translate a program to a readable Rust crate: `gopatina build -emit-rust -o crate .`.
4. Build a native binary through Rust, with no garbage collector, and see every borrow-checker decision: `gopatina build -patina -explain -o app .`.
5. Check that the build is reproducible, byte for byte: `gopatina verify .`.

## All the commands {#commands}

```sh
go install github.com/pigfox/gopatina/cmd/gopatina@main

gopatina build ./...                        # runs go build ./...
gopatina build -emit-rust -o crate .        # writes a cargo crate
gopatina build -patina -explain -o app .    # native binary, with every decision
gopatina run -patina .                      # build and run it
gopatina build -patina=check .              # patina checks, then the Go compiler
gopatina verify .                           # reproducible: -patina == plain cargo
```

`-patina=check` runs the patina checks and then builds with the Go compiler, for machines where Rust cannot be installed; the output says which compiler built the binary.

## Examples {#examples}

The [examples](https://github.com/pigfox/gopatina/blob/main/EXAMPLES.md) show the three modes on three small programs: real output for the two that fit the subset, and the goroutine example's planned output, clearly labelled. Emitted Rust is byte-identical for the same input, and `gopatina verify` checks that a `-patina` binary has the same sha256 as a plain `cargo build --release --locked` of the emitted crate.

Next: [how it works](/how-it-works), or the [frequently asked questions](/faq).
