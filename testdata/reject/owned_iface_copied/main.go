// An interface value holding a pointer copied while the original is still
// used: Go's copy shares the object, which would have two owners (RFC 0017).
package main

import (
	"errors"
	"fmt"
)

type NotFound struct{ Key string }

func (e *NotFound) Error() string { return "not found: " + e.Key }

func find(k string) error { return &NotFound{Key: k} }

func main() {
	base := find("k")
	err := fmt.Errorf("lookup: %w", base)
	fmt.Println(err, base, errors.Is(err, base))
}
