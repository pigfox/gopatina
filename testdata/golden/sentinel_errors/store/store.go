// Package store declares sentinel errors and returns them.
package store

import (
	"errors"
	"fmt"
)

// ErrMissing is returned for a key that is not there.
var ErrMissing = errors.New("missing")

// ErrLocked wraps ErrMissing, as a store might.
var ErrLocked = fmt.Errorf("locked: %w", ErrMissing)

// Get returns the value of key, or one of the sentinel errors.
func Get(key string) (int, error) {
	switch key {
	case "a":
		return 1, nil
	case "l":
		return 0, ErrLocked
	}
	return 0, ErrMissing
}
