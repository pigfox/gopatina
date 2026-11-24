// Command owned_escape_box takes the address of local struct values and
// hands the pointer on: returned, appended, stored in a field, declared.
// Nothing uses a value after its &, so the pointer is its only name from
// there, and Rust moves the value into a new box at the &: Box::new(n)
// (RFC 0017). The value is a plain local until then.
package main

import "fmt"

// Item is a list item.
type Item struct {
	V    int
	Name string
	Next *Item
}

// Bag holds items.
type Bag struct {
	Items []*Item
	First *Item
}

// newItem fills a local value, then returns its address.
func newItem(v int, name string) *Item {
	it := Item{V: v}
	it.Name = name
	it.V *= 10
	return &it
}

// push builds an item in front of head.
func push(head *Item, v int) *Item {
	var it Item
	it.Next = head
	it.V = v
	return &it
}

// fill appends an item per value, each declared in the loop.
func fill(b *Bag, vs []int) {
	for _, v := range vs {
		it := Item{V: v, Name: "item"}
		b.Items = append(b.Items, &it)
	}
}

// length counts the items of a list.
func length(l *Item) int {
	n := 0
	for it := l; it != nil; it = it.Next {
		n++
	}
	return n
}

func main() {
	a := newItem(3, "a")
	l := push(push(a, 2), 1)
	fmt.Println(l.V, l.Next.V, l.Next.Next.V, l.Next.Next.Name, length(l))

	b := &Bag{}
	fill(b, []int{4, 5, 6})
	first := Item{V: 7, Name: "first"}
	b.First = &first
	fmt.Println(len(b.Items), b.Items[2].V, b.Items[0].Name, b.First.Name)

	last := Item{V: 8}
	p := &last
	p.V++
	fmt.Println(p.V, p.Next == nil)
}
