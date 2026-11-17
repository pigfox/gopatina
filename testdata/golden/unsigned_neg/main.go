package main

import "fmt"

func main() {
	var a uint8 = 5
	b := -a
	fmt.Println(b, ^a, a-6, a*100)
	var c int = 5
	var d uint8 = uint8(c) - 10
	fmt.Println(d)
	var e byte = 'a'
	e -= 'b'
	fmt.Println(e)
	var x int32 = 1 << 30
	fmt.Println(x*4, x*2, x+x+x)
	var big int64 = 1 << 62
	fmt.Println(big*2, big*4, big+big+big)
	fmt.Println(int64(int32(big+3)), int32(int64(x)*8))
	var neg int = -1
	fmt.Println(uint8(neg), int32(neg), int64(neg), uint8(neg)+1)
	var f float64 = -1.5
	fmt.Println(int(f), int32(f*2), int64(f*1e3))
}
