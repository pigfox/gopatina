// Command check prints what the wordstat package computes on fixed inputs;
// check/main.rs prints the same through the Rust crate.
package main

import (
	"fmt"

	"github.com/pigfox/gopatina/testdata/golden-lib/wordstat"
)

func main() {
	words := wordstat.Words("the cat and the dog and the bird")
	fmt.Println(len(words), words[0], wordstat.MaxWords)
	for _, c := range wordstat.Top(words) {
		fmt.Println(c.Word, c.N)
	}
}
