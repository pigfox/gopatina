// Command owned_lent_to_walk lends fields and slice elements to a function
// that walks a list with a temporary borrow (p := l). The function only
// lends its parameter to that borrow, so it borrows it too, and the
// caller lends a field or an element in place: no copy is kept, and the
// list's pointers keep one owner (RFC 0017).
package main

import "fmt"

// Link is a list link.
type Link struct {
	V    int
	Next *Link
}

// Lists holds a list and a slice of lists.
type Lists struct {
	Main *Link
	All  []*Link
}

// count walks the list from l, keeping a borrow of the last odd link.
func count(l *Link) int {
	n := 0
	var odd *Link
	for p := l; p != nil; p = p.Next {
		n++
		if p.V%2 == 1 {
			odd = p
		}
	}
	if odd != nil {
		n += odd.V * 100
	}
	return n
}

// build returns a list of n links, the last first.
func build(n int) *Link {
	var h *Link
	for i := 0; i < n; i++ {
		x := &Link{V: i}
		x.Next = h
		h = x
	}
	return h
}

// newLists returns lists of 3, 2 and 4 links.
func newLists() *Lists {
	ls := &Lists{Main: build(3)}
	ls.All = append(ls.All, build(2))
	ls.All = append(ls.All, build(4))
	return ls
}

func main() {
	ls := newLists()
	fmt.Println(count(ls.Main), count(ls.All[0]), count(ls.All[1]))
	fmt.Println(count(ls.Main.Next), ls.Main.V)
}
