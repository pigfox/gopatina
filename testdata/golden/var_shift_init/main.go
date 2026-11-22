// Shifts by a count known only at run time, and init functions, which Go
// runs in source order before main. A negative count panics.
package main

import "fmt"

func init() { fmt.Println("init 1") }

func note(x int) int {
	fmt.Println("index", x)
	return x
}

func count(x int) int {
	fmt.Println("count", x)
	return x
}

func main() {
	var x int8 = -5
	var u uint16 = 3
	s := 3
	var big uint64 = 1 << 40
	var w int32 = 1
	fmt.Println(x<<s, x>>s, x<<9, x>>100, u<<u, u>>big, 1<<s, w<<big, w>>big, -w>>big)
	var c int32 = 1
	c <<= s
	var d uint64 = 1 << 63
	d >>= uint8(s)
	vs := []int{1, 2, 4}
	vs[note(1)] <<= count(4)
	vs[note(2)] >>= count(70)
	fmt.Println(c, d, vs)
	n := -1
	fmt.Println(x << n)
}

func init() { fmt.Println("init 2") }
