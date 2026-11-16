// Package gopatina is the root of the GoPatina module.
//
// GoPatina is a design-stage project: a Go toolchain front end whose default
// mode delegates to the standard go command, and whose opt-in patina mode aims
// to run a defined subset of Go with no garbage collector and Rust-grade
// memory safety, with Rust under the hood. How patina mode is built is an open
// decision. The design is described
// in docs/rfcs/0001-gopatina.md. The command-line tool lives in cmd/gopatina.
//
// This package has no exported API.
package gopatina
