// Command owned_moves moves owned pointers (RFC 0016) from one owner to
// the next, one owner per object (RFC 0017, step 4): b := a moves the
// list from a to b, x.Next = head moves head into a field, and a new
// node's literal takes the list it is given. A list built this way, in a
// loop, can be longer than any call depth, so its type frees a chain one
// link at a time.
package main

import "fmt"

type List struct {
	V    int
	Name string
	Next *List
}

// push builds a list of n nodes in a loop, each new node taking the list.
func push(n int) *List {
	var head *List
	for i := 0; i < n; i++ {
		x := &List{V: i}
		x.Next = head
		head = x
	}
	return head
}

// cons gives the list to a new node, by its literal.
func cons(v int, rest *List) *List {
	n := &List{V: v, Name: "cons"}
	n.Next = nil
	return n
}

// Tree has two children: built by moves too, it frees its nodes in a
// loop, with a stack of the links still to free.
type Tree struct {
	V           int
	Left, Right *Tree
}

// spine builds a tree whose left children make a chain of n nodes.
func spine(n int) *Tree {
	var root *Tree
	for i := 0; i < n; i++ {
		t := &Tree{V: i, Right: &Tree{V: -i}}
		t.Left = root
		root = t
	}
	return root
}

// sum reads the list, recursively.
func sum(l *List) int {
	s := 0
	if l != nil {
		s = l.V + sum(l.Next)
	}
	return s
}

// length reads the list in a loop of calls.
func length(l *List) int {
	if l == nil {
		return 0
	}
	return 1 + length(l.Next)
}

func main() {
	a := push(5)
	b := a // the list moves to b; a is dead from here
	fmt.Println(sum(b), b.V, length(b))
	c := &List{V: 9, Next: b} // b moves into c's literal
	fmt.Println(sum(c), c.Name == "")
	c.Next.Next = &List{V: 100} // a new node replaces the rest of the list
	fmt.Println(sum(c), length(c))
	a = push(3) // a assigned again: alive
	fmt.Println(sum(a), cons(1, nil).Name)
	big := push(1000000)
	fmt.Println(big.V, big.Next.V)
	var keep = big // a declaration moves it too
	keep.V = -1
	fmt.Println(keep.V)
	tall := spine(500000)
	fmt.Println(tall.V, tall.Left.V, tall.Right.V)
}
