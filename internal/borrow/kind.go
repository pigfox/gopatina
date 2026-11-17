// Package borrow is GoPatina's borrow checker (stage 3). It runs on the
// type-checked Go program before any Rust is written, and decides, in Go
// terms, who owns each value: where a value is moved, where it is copied
// (cloned) because Go's meaning requires a copy, and whether each
// parameter, receiver and pointer is a shared or an exclusive borrow. Every
// decision it makes on its own is recorded for -explain. When the program
// shares memory in a way that no safe fix can express, it reports a
// Go-style error instead of guessing.
//
// The rules are in docs/patina-subset.md. rustc's own borrow checker then
// verifies the generated code as the final proof.
package borrow

import "go/types"

// A Kind says how values of a Go type behave under ownership.
type Kind int

const (
	// Copy values (numbers, booleans, structs made only of them) are
	// copied, as in Go and as with Rust's Copy.
	Copy Kind = iota
	// Value values (strings, and structs of strings and Copy values) have
	// one owner. Copying one is always safe, because nothing inside can be
	// changed through another copy: Go strings are immutable.
	Value
	// Shared values hold a slice, directly or in a field. Two Go copies
	// share the slice's backing array, so a change through one is seen
	// through the other. A Rust copy would not share it, so GoPatina never
	// copies these on its own.
	Shared
	// PointerKind values are borrows of another variable.
	PointerKind
	// Writer is io.Writer, always an exclusive borrow.
	Writer
	// Error is the error type, lowered to Result.
	Error
)

// KindOf returns the ownership kind of t, a type inside the subset.
func KindOf(t types.Type) Kind {
	switch t := t.(type) {
	case *types.Slice:
		return Shared
	case *types.Pointer:
		return PointerKind
	case *types.Named:
		if t.Obj().Pkg() == nil {
			return Error
		}
		if t.Obj().Pkg().Path() == "io" {
			return Writer
		}
		st := t.Underlying().(*types.Struct)
		k := Copy
		for i := 0; i < st.NumFields(); i++ {
			k = max(k, KindOf(st.Field(i).Type()))
		}
		return k
	}
	if b, ok := t.(*types.Basic); ok && b.Info()&types.IsString != 0 {
		return Value
	}
	return Copy
}
