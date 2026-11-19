package main

import "testing"

// The benchmark harness compares the program's exact output across all
// three compilers; here we only need the case to stay buildable and covered.
func TestChecksum(t *testing.T) {
	main()
}
