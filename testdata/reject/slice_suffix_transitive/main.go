package main

import "slices"

func read(s []int) {
	next(s[1:])
}

func next(s []int) {
	// A whole slice clone is legal Phase 1 syntax, but not an audited use
	// of the temporary view in this first increment.
	_ = slices.Clone(s)
}

func main() {
	s := []int{10, 20, 30}
	read(s[0:])
}
