// Command pointer_conflict uses a variable directly while a pointer to it
// is still in use.
package main

import "fmt"

type Point struct{ X, Y int }

func main() {
	p := Point{1, 2}
	q := &p
	p.X = 10
	q.Y = 20
	fmt.Println(p.X, p.Y)
}
