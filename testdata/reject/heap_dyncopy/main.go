// A pointer in an interface (RFC 0013) whose value methods would run on a
// copy that does not share its elements.
package main

import "fmt"

type Bag struct{ items []int }

func (b Bag) Len() int { return len(b.items) }

type Sizer interface{ Len() int }

func main() {
	var s Sizer = &Bag{items: []int{1}}
	var t Sizer = &Bag{}
	fmt.Println(s.Len(), t.Len())
}
