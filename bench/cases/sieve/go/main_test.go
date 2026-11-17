package main

import "testing"

func TestCountPrimes(t *testing.T) {
	if got := countPrimes(100); got != 25 {
		t.Errorf("countPrimes(100) = %d", got)
	}
	main()
}
