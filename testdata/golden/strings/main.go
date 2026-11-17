// Command strings exercises Go strings: concatenation, bytes, runes and the
// supported functions of package strings.
package main

import (
	"fmt"
	"strings"
)

func shout(s string) string {
	return s + "!"
}

func greet(name string, times int) string {
	out := ""
	for i := 0; i < times; i++ {
		if i > 0 {
			out += " "
		}
		out += "hi " + name
	}
	return out
}

func main() {
	s := "héllo, wörld"
	fmt.Println(len(s), s[0], s[1], s[2])
	for i, r := range s {
		if r > 127 {
			fmt.Println(i, r)
		}
	}
	count := 0
	for range s {
		count++
	}
	fmt.Println("runes:", count)

	name := "Gopher"
	fmt.Println(shout(name), name)
	fmt.Println(greet(name, 3))

	a, b := "apple", "banana"
	fmt.Println(a < b, a == "apple", b != a, a > b)

	words := []string{"go", "on", "top"}
	joined := strings.Join(words, "-")
	fmt.Println(joined, strings.Contains(joined, "on"), strings.Index(joined, "top"), strings.Index(joined, "x"))
	fmt.Println(strings.HasPrefix(joined, "go"), strings.HasSuffix(joined, "go"))
	fmt.Println(strings.Repeat("ab", 3) + "|" + strings.TrimSpace("  \t padded \n") + "|")

	greeting := name
	greeting += ", welcome"
	fmt.Println(greeting, "/", name)

	bs := []byte("abc")
	bs[0] = 'A'
	bs = append(bs, "def"...)
	fmt.Println(len(bs), bs[0], bs[5])
	fmt.Printf("%s\n", bs)
}
