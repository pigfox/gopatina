// A pointer in an interface (RFC 0013) whose value methods would run on a
// copy that does not share its elements, and a pointer fmt would print by
// its address.
package main

import "fmt"

type Bag struct{ items []int }

func (b Bag) Len() int { return len(b.items) }

type Sizer interface{ Len() int }

type Pt struct{ X int }

func keep() *Pt { return &Pt{X: 1} }

func main() {
	var s Sizer = &Bag{items: []int{1}}
	fmt.Println(s.Len())
	fmt.Println(keep())
	var a any = keep()
	fmt.Println(a)
}
