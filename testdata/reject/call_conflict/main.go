// Command call_conflict reads a field and changes it in the same call; Go
// does not specify which happens first.
package main

import "fmt"

type Counter struct{ N int }

func (c *Counter) Next() int {
	c.N++
	return c.N
}

func main() {
	var c Counter
	fmt.Println(c.N, c.Next())
}
