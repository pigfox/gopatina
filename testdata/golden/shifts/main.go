package main

import "fmt"

func main() {
	var a int = 1
	fmt.Println(a<<62, a<<63, a<<3)
	var b uint8 = 200
	fmt.Println(b<<1, b>>3, b<<7)
	var c int32 = -8
	fmt.Println(c>>1, c>>30, c<<29, c<<30)
	var d int64 = -1
	fmt.Println(d>>63, d<<63, uint8(d))
	var e uint8 = 255
	fmt.Println(int(e), int32(e)<<24)
	x := 1000
	fmt.Println(uint8(x), uint8(x>>2))
	var big int64 = 1 << 40
	fmt.Println(int32(big), int32(big+5), uint8(big+300))
}
