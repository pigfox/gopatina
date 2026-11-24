// Command static_args reads package-level variables that nothing changes
// (statics, RFC 0010) in every position a read can take: a call argument,
// an operand, an index, a receiver and a field's base. A static is a
// LazyLock read through *S, which needs parentheses only before a field, a
// method or an index; anywhere else rustc warns that they are unused.
package main

import "fmt"

type Point struct{ X, Y int }

func (p Point) Sum() int { return p.X + p.Y }

func double(n int) int { return 2 * n }

func greet(s string) string { return "hello " + s }

var limit = 7

var name = "go"

var origin = Point{X: 3, Y: 4}

var table = []int{10, 20, 30}

func main() {
	fmt.Println(double(limit), greet(name), -limit, limit*limit+1)
	fmt.Println(origin.Sum(), origin.X, table[1], len(table), table[limit%3])
	n := limit
	s := name
	fmt.Println(n, s, double(double(limit)))
}
