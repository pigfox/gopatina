// Command append_other assigns the result of append to another variable;
// in Go the two may share a backing array.
package main

import "fmt"

func main() {
	s := make([]int, 1, 4)
	t := append(s, 2)
	fmt.Println(len(s), len(t))
}
