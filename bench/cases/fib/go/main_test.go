package main

import "testing"

func TestFib(t *testing.T) {
	if got := fib(20); got != 6765 {
		t.Errorf("fib(20) = %d", got)
	}
	main()
}
