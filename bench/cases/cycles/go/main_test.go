package main

import "testing"

func TestCycles(t *testing.T) {
	if got := walk(build(4)); got != 6-(0+0+1+1) {
		t.Errorf("walk of 4 = %d", got)
	}
	rounds, size = 2, 3
	main()
}
