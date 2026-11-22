// i <= n needs n >= 0, which make([]int, n+1) does not prove for n itself,
// so the first loop is not specialized; make([]int, m) proves m >= 0, so
// the second one is (RFC 0006).
package main

import "fmt"

func main() {
	n := len("abcd")
	s := make([]int, n+1)
	for i := 0; i <= n; i++ {
		s[i] = i
	}
	m := len("abc")
	t := make([]int, m)
	for i := 0; i <= m; i++ {
		s[i] += i * 10
	}
	fmt.Println(s, t)
}
