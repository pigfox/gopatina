// A pointer converted to an interface while the variable is still used:
// the interface owns the object (RFC 0017), so n would be a second owner.
package main

import "fmt"

type Node struct{ V int }

func (n *Node) Get() int { return n.V }

type Getter interface{ Get() int }

func main() {
	n := &Node{V: 1}
	var g Getter = n
	n.V++
	fmt.Println(g.Get(), n.V)
}
