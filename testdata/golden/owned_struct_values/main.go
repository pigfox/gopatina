// Command owned_struct_values keeps owned pointers (RFC 0016) in struct
// values that are not behind a pointer (RFC 0017): the struct value owns
// them, as a Rust struct owns its Box fields. It is built by a literal,
// given new pointers in its fields, lent to functions, returned, and
// moved whole to another variable.
package main

import "fmt"

type Item struct {
	V    int
	Next *Item
}

// Owner holds a list and a slice of items, and some data.
type Owner struct {
	Name  string
	Head  *Item
	Items []*Item
	N     int
}

func newOwner(name string) Owner {
	o := Owner{Name: name, Head: &Item{V: 1}}
	o.Head.Next = &Item{V: 2}
	o.Items = append(o.Items, &Item{V: 10}, &Item{V: 20})
	return o
}

// total reads an owner it is lent.
func total(o *Owner) int {
	t := o.N
	for p := o.Head; p != nil; p = p.Next {
		t += p.V
	}
	for _, it := range o.Items {
		t += it.V
	}
	return t
}

// bump changes the owner it is lent.
func bump(o *Owner) {
	o.N++
	o.Head.V *= 10
}

func main() {
	a := newOwner("a")
	bump(&a)
	fmt.Println(a.Name, total(&a), a.Head.V, len(a.Items))
	b := a // the whole value moves: a is dead from here
	b.Items[1].V = 25
	fmt.Println(b.Name, total(&b))
	c := Owner{Head: &Item{V: 7}}
	c.Head.Next = &Item{V: 8}
	fmt.Println(total(&c), len(c.Items))
}
