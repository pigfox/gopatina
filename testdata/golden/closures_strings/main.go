// Command closures_strings writes closures over strings (RFC 0018): string
// parameters, strings a closure uses or changes from its enclosing
// function, and string results, local, passed and returned.
package main

import (
	"fmt"
	"strings"
)

func each(words []string, f func(string)) {
	for _, w := range words {
		f(w)
	}
}

func mapWords(words []string, f func(string) string) []string {
	out := []string{}
	for _, w := range words {
		out = append(out, f(w))
	}
	return out
}

func greeter(greeting string) func(string) string {
	return func(name string) string {
		return greeting + ", " + name + "!"
	}
}

func main() {
	sep := " | "
	join := func(a, b string) string {
		return a + sep + b
	}
	fmt.Println(join("left", "right"))

	log := ""
	record := func(s string) {
		log += s + ";"
	}
	record("start")
	record("stop")
	fmt.Println(log)

	total := 0
	each([]string{"go", "rust", "c"}, func(w string) {
		total += len(w)
	})
	fmt.Println(total)

	shout := func(w string) string {
		return strings.ToUpper(w) + "!"
	}
	fmt.Println(mapWords([]string{"hey", "you"}, shout))

	id := func(s string) string {
		return s
	}
	fmt.Println(id("same"), mapWords([]string{"p", "q"}, id))
	get := func() string {
		return sep
	}
	fmt.Println(get(), get())
	hello := greeter("Hello")
	fmt.Println(hello("Ann"), hello("Bob"))
}
