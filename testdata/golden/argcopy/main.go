// Command argcopy passes a field to a function together with an
// argument that changes the same struct: the field is copied first.
package main

import "fmt"

type Box struct {
	Name string
	N    int
}

func bump(b *Box) int {
	b.N++
	return b.N
}

func show(name string, n int) {
	fmt.Println(name, n)
}

func main() {
	b := Box{Name: "box"}
	show(b.Name, bump(&b))
}
