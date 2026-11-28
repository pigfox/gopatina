// Command panic_error_nil panics with a nil error, which Go reports as
// its runtime.PanicNilError: "panic called with nil argument".
package main

import "fmt"

func check() error {
	return nil
}

func main() {
	err := check()
	fmt.Println("checked:", err)
	panic(err)
}
