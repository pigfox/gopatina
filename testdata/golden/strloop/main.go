package main

import (
	"fmt"
	"strings"
)

func main() {
	s := ""
	for i := 0; i < 5; i++ {
		s += fmt.Sprint(i)
		s = s + "-"
	}
	fmt.Println(s, len(s))
	a, b := "apple", "banana"
	fmt.Println(a < b, a > b, a == b, a != b, a <= "apple", "Z" < "a", "" < "a")
	a, b = b, a
	fmt.Println(a, b)
	t := strings.Repeat("ab", 3) + strings.TrimSpace("  \t x y \n")
	fmt.Println(t, strings.Index(t, "ba"), strings.Index(t, "zz"), strings.Contains(t, ""), strings.HasPrefix(t, "ab"), strings.HasSuffix(t, "y"))
	parts := []string{"x", "y", "z"}
	fmt.Println(strings.Join(parts, ", "), strings.Join(nil, ","))
}
