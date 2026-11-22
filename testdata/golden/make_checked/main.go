// make with lengths and capacities only known at run time, of several
// integer types, within range, beside constant ones.
package main

import "fmt"

func size(k int) int { return k - 5 }

func main() {
	var k int32 = 3
	a := make([]int, k)
	b := make([]string, size(7), size(9))
	c := make([]byte, 0, size(10))
	c = append(c, 'x')
	d := make([]bool, 2, size(8))
	e := make([]int, 0, 4)
	e = append(e, 7)
	f := make([]int, 2, 4)
	fmt.Println(len(a), len(b), len(c), len(d), a, b, c, d, e, f)
}
