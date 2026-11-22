// Package parse returns errors to another package.
package parse

import (
	"errors"
	"fmt"
)

// Digit returns the value of a decimal digit.
func Digit(c byte) (int, error) {
	if c < '0' || c > '9' {
		return 0, fmt.Errorf("not a digit: %c", c)
	}
	return int(c - '0'), nil
}

// Number parses a string of digits, passing Digit's errors on.
func Number(s string) (int, error) {
	if s == "" {
		return 0, errors.New("empty")
	}
	n := 0
	for i := 0; i < len(s); i++ {
		d, err := Digit(s[i])
		if err != nil {
			return 0, err
		}
		n = n*10 + d
	}
	return n, nil
}

// Check reports whether n is small.
func Check(n int) error {
	if n > 99 {
		return fmt.Errorf("too big: %d", n)
	}
	return nil
}

// Parser parses with a limit.
type Parser struct{ Limit int }

// Parse parses s and checks it against the limit.
func (p Parser) Parse(s string) (int, error) {
	n, err := Number(s)
	if err != nil {
		return 0, err
	}
	if n > p.Limit {
		return 0, fmt.Errorf("over %d", p.Limit)
	}
	return n, nil
}
