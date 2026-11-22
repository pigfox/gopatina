package main

import "testing"

func TestBinaryTrees(t *testing.T) {
	if got := bottomUp(3).check(); got != 15 {
		t.Errorf("check of depth 3 = %d", got)
	}
	maxDepth = 4
	main()
}
