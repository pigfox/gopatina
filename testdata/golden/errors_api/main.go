// Command errors_api exercises the provided errors API (RFC 0012):
// fmt.Errorf with %w, errors.Unwrap, errors.Is and errors.As, through
// Go's own error types and the program's, with their Unwrap and Is
// methods, as Go's errors package defines them.
package main

import (
	"errors"
	"fmt"
)

// Op is an error that wraps another, with its own Unwrap.
type Op struct {
	Name string
	Err  error
}

func (o Op) Error() string { return fmt.Sprint(o.Name, ": ", o.Err) }

func (o Op) Unwrap() error { return o.Err }

// Code is an error that says it is any Code with the same N.
type Code struct{ N int }

func (c Code) Error() string { return fmt.Sprint("code ", c.N) }

func (c Code) Is(target error) bool {
	t, ok := target.(Code)
	return ok && t.N == c.N
}

// Path is an error type that is not comparable.
type Path []string

func (p Path) Error() string { return fmt.Sprint("path ", len(p)) }

// UncomparableListOfStrings is another error type that is not comparable.
type UncomparableListOfStrings []string

func (u UncomparableListOfStrings) Error() string { return "list" }

// Never is an error type no value of which becomes an error.
type Never struct{ X int }

func (n Never) Error() string { return "never" }

// Coded is implemented by Code only.
type Coded interface{ Get() int }

func (c Code) Get() int { return c.N }

func main() {
	base := errors.New("base")
	wrapped := fmt.Errorf("read %s: %w", "f", base)
	twice := fmt.Errorf("open: %w", wrapped)
	var none error
	nilWrapped := fmt.Errorf("nil: %w", none)
	fmt.Println(wrapped, twice, nilWrapped)
	fmt.Printf("%T %T %v\n", wrapped, nilWrapped, errors.Unwrap(nilWrapped) == nil)
	fmt.Println(errors.Unwrap(twice) == wrapped, errors.Unwrap(wrapped) == base, errors.Unwrap(base) == nil)
	fmt.Println(errors.Is(twice, base), errors.Is(base, twice), errors.Is(nil, nil), errors.Is(base, nil), errors.Is(nil, base))

	op := Op{Name: "op", Err: Code{N: 7}}
	var opErr error = op
	var err error = fmt.Errorf("outer: %w", opErr)
	fmt.Println(err, errors.Is(err, Code{N: 7}), errors.Is(err, Code{N: 8}), errors.Is(err, op))
	fmt.Println(errors.Unwrap(op), errors.Unwrap(errors.Unwrap(err)) == Code{N: 7})

	var c Code
	fmt.Println(errors.As(err, &c), c.N)
	var o Op
	fmt.Println(errors.As(err, &o), o.Name)
	var cd Coded
	fmt.Println(errors.As(err, &cd), cd.Get())
	var n Never
	fmt.Println(errors.As(err, &n), n.X, errors.As(nil, &c))

	p := Path{"a", "b"}
	var perr error = p
	fmt.Println(errors.Is(perr, base), errors.Is(fmt.Errorf("x: %w", perr), Code{N: 1}))
	var pp Path
	fmt.Println(errors.As(fmt.Errorf("y: %w", perr), &pp), len(pp))
	var list error = UncomparableListOfStrings{"x"}
	fmt.Println(errors.Is(list, base), errors.Is(fmt.Errorf("z: %w", list), list), list)
}
