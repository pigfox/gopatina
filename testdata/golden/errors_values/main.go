// Command errors_values uses errors as ordinary interface values (RFC
// 0012): kept, stored, compared, passed, printed, switched on and
// asserted, custom error types, and errors made by errors.New, each equal
// only to itself.
package main

import (
	"errors"
	"fmt"
)

// NotFound is a custom error type.
type NotFound struct {
	Name string
	Code int
}

func (e NotFound) Error() string { return fmt.Sprintf("%s not found (%d)", e.Name, e.Code) }

// ErrCode is implemented by NotFound only.
func (e NotFound) ErrCode() int { return e.Code }

// Coded is an interface besides error, which errors.New's values lack.
type Coded interface{ ErrCode() int }

// Retrier is implemented by every error type of the program, but not by
// errors.New's values.
type Retrier interface{ Retry() bool }

func (e NotFound) Retry() bool { return false }

func (t Temporary) Retry() bool { return true }

// two returns two values and an error.
func two(n int) (int, string, error) {
	if n < 0 {
		return 0, "", Temporary(n)
	}
	return n, fmt.Sprint(n), nil
}

// Three error types that are not comparable, with long names.
type UncomparableListOfStrings []string

func (u UncomparableListOfStrings) Error() string { return "list" }

type AnotherUncomparableSliceError []int

func (a AnotherUncomparableSliceError) Error() string { return "slice" }

type YetAnotherUncomparableError []bool

func (y YetAnotherUncomparableError) Error() string { return "bools" }

func (u UncomparableListOfStrings) Retry() bool { return true }

func (a AnotherUncomparableSliceError) Retry() bool { return true }

func (y YetAnotherUncomparableError) Retry() bool { return true }

// pass returns its error, which may be nil.
func pass(e error) error { return e }

// Temporary is an error type of another kind.
type Temporary int

func (t Temporary) Error() string { return fmt.Sprint("temporary ", int(t)) }

func lookup(name string) (int, error) {
	switch name {
	case "a":
		return 1, nil
	case "b":
		return 0, NotFound{Name: name, Code: 404}
	case "c":
		return 0, Temporary(3)
	}
	return 0, errors.New("bad name " + name)
}

func check(n int) error {
	if n > 2 {
		return fmt.Errorf("too big: %d", n)
	}
	return nil
}

// first returns the first error of errs, which may be nil.
func first(errs []error) (int, error) {
	var e error
	for _, x := range errs {
		if x != nil {
			e = x
			break
		}
	}
	return 0, e
}

type Result struct {
	Value int
	Err   error
}

func describe(err error) string {
	switch e := err.(type) {
	case nil:
		return "no error"
	case NotFound:
		return fmt.Sprint("missing ", e.Name, " ", e.Code)
	case Temporary:
		return fmt.Sprint("retry after ", int(e))
	case Coded:
		return fmt.Sprint("coded ", e.ErrCode())
	default:
		return "other: " + e.Error()
	}
}

func main() {
	sentinel := errors.New("same text")
	other := errors.New("same text")
	copied := sentinel
	fmt.Println(sentinel == copied, sentinel == other, sentinel != nil, sentinel.Error() == other.Error())

	var results []Result
	for _, name := range []string{"a", "b", "c", "d"} {
		v, err := lookup(name)
		results = append(results, Result{Value: v, Err: err})
		fmt.Println(name, v, err, describe(err))
	}
	for _, r := range results {
		fmt.Printf("%v|%s|%T|%12v|%-12s|\n", r.Err, r.Err, r.Err, r.Err, r.Err)
	}
	fmt.Println(results[0] == Result{Value: 1}, results[1] == Result{Err: NotFound{Name: "b", Code: 404}})

	nf, ok := results[1].Err.(NotFound)
	fmt.Println(nf.Name, nf.Code, ok)
	_, ok = results[2].Err.(NotFound)
	fmt.Println(ok)

	var errs []error
	for i := 0; i < 5; i++ {
		errs = append(errs, check(i))
	}
	fmt.Println(errs[0] == nil, errs[3], errs[3] == errs[4])
	n, err := first(errs)
	fmt.Println(n, err)
	n, err = first(nil)
	fmt.Println(n, err, err == nil)

	if v, err := lookup("a"); err == nil {
		fmt.Println("found", v)
	}
	v, err := lookup("zz")
	if err != nil {
		fmt.Println("value stays", v)
	}
	fmt.Println(err)
	fmt.Println(check(1), check(7))

	if v, err := lookup("b"); err != nil {
		fmt.Println("v is still", v)
	}
	if v, err := lookup("a"); err == nil {
		fmt.Println("ok", v)
	} else {
		fmt.Println("not ok", err)
	}
	kept := check(9)
	passed := pass(kept)
	fmt.Println(kept, passed, pass(nil) == nil)
	fmt.Printf("|%-12v|\n", kept)
	fmt.Print(kept, " ", sentinel, "\n")
	a, b, err2 := two(-2)
	fmt.Println(a, b, err2)
	a, b, err2 = two(4)
	fmt.Println(a, b, err2)
	var anything any = Temporary(1)
	switch anything.(type) {
	case error:
		fmt.Println("an error")
	}
	_, isRetrier := errors.New("y").(Retrier)
	r, isRetrier2 := results[2].Err.(Retrier)
	fmt.Println(isRetrier, isRetrier2, r.Retry())
	var list error = UncomparableListOfStrings{"x"}
	var slice error = AnotherUncomparableSliceError{1}
	var bools error = YetAnotherUncomparableError{true}
	fmt.Println(errors.Is(list, slice), errors.Is(bools, list), list, slice, bools)
	var c Coded = NotFound{Name: "x", Code: 7}
	fmt.Println(c.ErrCode(), describe(errors.New("plain")))
}
