// Command panic_custom_error panics with a value of an error type of its
// own, and Go prints that value's Error().
package main

import "fmt"

type codeError struct {
	code int
}

func (e codeError) Error() string {
	return fmt.Sprintf("failed with code %d", e.code)
}

func main() {
	fmt.Println("start")
	panic(codeError{42})
}
