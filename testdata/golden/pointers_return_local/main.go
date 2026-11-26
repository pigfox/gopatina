// A function that lends back a field of its input through a local
// variable, or through its named result, returns a borrow of the input,
// as return n.Next does (the parity corpus's next refusal: "n.Next is
// copied and kept in r, so its object has two owners").
package main

import "fmt"

type Node struct {
	V    int
	Next *Node
}

// second returns the node after n, through a local.
func second(n *Node) *Node {
	r := n.Next
	return r
}

// third returns the node two after n, through its named result.
func third(n *Node) (r *Node) {
	r = n.Next.Next
	return
}

// last walks from n to the last node, through its named result.
func last(n *Node) (r *Node) {
	for r = n; r.Next != nil; r = r.Next {
	}
	return
}

func main() {
	list := &Node{V: 1, Next: &Node{V: 2, Next: &Node{V: 3}}}
	fmt.Println(second(list).V, third(list).V, last(list).V)
	if s := second(list); s != nil {
		fmt.Println(s.V + third(list).V)
	}
	list.V = 10
	fmt.Println(list.V, last(list.Next).V)
}
