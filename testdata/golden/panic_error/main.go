// Command panic_error panics with an error value, as Go programs do with
// panic(err): Go prints its Error(), and indents each line after the first.
package main

import (
	"errors"
	"fmt"
)

func open(name string) error {
	if name == "" {
		inner := errors.New("empty name\nsecond line")
		return fmt.Errorf("open: %w", inner)
	}
	return nil
}

func main() {
	fmt.Println("opening")
	if err := open(""); err != nil {
		panic(err)
	}
	fmt.Println("not reached")
}
