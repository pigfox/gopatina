// A nest whose fast copy would be over 60 lines is not specialized
// (RFC 0006, decided 2026-11-21); it keeps the general copy only.
package main

import "fmt"

func work(n int) int {
	s := make([]int, n)
	total := 0
	for i := 0; i < n; i++ {
		s[i] += 1
		s[i] += 2
		s[i] += 3
		s[i] += 4
		s[i] += 5
		s[i] += 6
		s[i] += 7
		s[i] += 8
		s[i] += 9
		s[i] += 10
		s[i] += 11
		s[i] += 12
		s[i] += 13
		s[i] += 14
		s[i] += 15
		s[i] += 16
		s[i] += 17
		s[i] += 18
		s[i] += 19
		s[i] += 20
		s[i] += 21
		s[i] += 22
		s[i] += 23
		s[i] += 24
		s[i] += 25
		s[i] += 26
		s[i] += 27
		s[i] += 28
		s[i] += 29
		s[i] += 30
		s[i] += 31
		s[i] += 32
		s[i] += 33
		s[i] += 34
		s[i] += 35
		s[i] += 36
		s[i] += 37
		s[i] += 38
		s[i] += 39
		s[i] += 40
		s[i] += 41
		s[i] += 42
		s[i] += 43
		s[i] += 44
		s[i] += 45
		s[i] += 46
		s[i] += 47
		s[i] += 48
		s[i] += 49
		s[i] += 50
		s[i] += 51
		s[i] += 52
		s[i] += 53
		s[i] += 54
		s[i] += 55
		s[i] += 56
		s[i] += 57
		s[i] += 58
		s[i] += 59
		s[i] += 60
		total += s[i]
	}
	return total
}

func main() {
	fmt.Println(work(4))
}
