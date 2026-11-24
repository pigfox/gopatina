// Command owned_graph_cycle builds a graph whose vertices point at each
// other through their edges: a cycle, in which each vertex has two names,
// its variable and the edge to it. One owner per object refuses it (RFC
// 0017); a graph keeps its vertices in a slice and links them by index.
package main

import "fmt"

type Edge struct {
	To   *Vertex
	Cost int
}

type Vertex struct {
	Name string
	Out  []*Edge
}

func main() {
	a := &Vertex{Name: "a"}
	b := &Vertex{Name: "b"}
	a.Out = append(a.Out, &Edge{To: b, Cost: 2})
	b.Out = append(b.Out, &Edge{To: a, Cost: 3})
	fmt.Println(a.Out[0].To.Name, a.Out[0].Cost+b.Out[0].Cost)
}
