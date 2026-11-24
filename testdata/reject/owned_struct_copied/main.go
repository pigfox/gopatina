// Command owned_struct_copied copies a struct value that owns a pointer
// (RFC 0016) and uses both: in Go the copy shares the pointer, which one
// owner per object refuses (RFC 0017).
package main

import "fmt"

type Item struct{ V int }

type Owner struct{ Item *Item }

func main() {
	a := Owner{Item: &Item{V: 1}}
	b := a
	b.Item.V = 2
	fmt.Println(a.Item.V, b.Item.V)
}
