// Command sentinel_errors uses package-level error variables (RFC 0012):
// sentinel errors of two packages, compared, wrapped and checked with
// errors.Is, initialized in Go's order (one by a call that prints).
package main

import (
	"errors"
	"fmt"

	"example.com/sentinels/store"
)

func made(name string) error {
	fmt.Println("making", name)
	return errors.New(name)
}

var ErrFirst = made("first")

var ErrSecond = made("second")

// Default is an interface value of another kind.
var Default any = 42

func find(key string) error {
	if key == "first" {
		return ErrFirst
	}
	_, err := store.Get(key)
	return err
}

func main() {
	fmt.Println("main starts", ErrFirst, ErrSecond, Default)
	for _, key := range []string{"a", "first", "l", "zz"} {
		err := find(key)
		fmt.Println(key, err, err == ErrFirst, err == store.ErrMissing,
			errors.Is(err, store.ErrMissing), errors.Is(err, store.ErrLocked))
	}
	fmt.Println(ErrFirst == ErrFirst, ErrFirst == ErrSecond, errors.Unwrap(store.ErrLocked) == store.ErrMissing)
	fmt.Printf("%v|%T|%v\n", ErrSecond, store.ErrLocked, Default)
}
