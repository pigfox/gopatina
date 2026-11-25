package main

import "testing"

func TestIfaceKeep(t *testing.T) {
	if got := (Rect{W: 2, H: 3, Name: "ab"}).Area(); got != 8 {
		t.Errorf("Area = %d", got)
	}
	rounds, size = 2, 201
	main()
}
