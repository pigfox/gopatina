package main

import "testing"

func TestIfaceCopy(t *testing.T) {
	if got := measure(Rect{W: 2, H: 3, Name: "ab"}); got != 8 {
		t.Errorf("measure = %d", got)
	}
	size = 2001
	main()
}
