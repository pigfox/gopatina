// A field's pointer converted to an interface: the field still holds the
// object the interface would own (RFC 0017).
package main

import "fmt"

type Kid struct{ V int }

func (k *Kid) Get() int { return k.V }

type Getter interface{ Get() int }

type Node struct{ Kid *Kid }

func main() {
	n := &Node{Kid: &Kid{V: 1}}
	var g Getter = n.Kid
	fmt.Println(g.Get(), n.Kid.V)
}
