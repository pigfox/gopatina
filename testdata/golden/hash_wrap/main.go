package main

import "fmt"

func fnv(s string) int64 {
	var h int64 = 1469598103
	for i := 0; i < len(s); i++ {
		h ^= int64(s[i])
		h *= 1099511628211
	}
	return h
}

func lcg(seed int32) int32 {
	return seed*1103515245 + 12345
}

func main() {
	fmt.Println(fnv("hello"), fnv("world"), fnv(""))
	var s int32 = 42
	for i := 0; i < 5; i++ {
		s = lcg(s)
		fmt.Println(s, s&0x7fffffff, s>>16, s|1, s^s>>3, s&^0xff)
	}
	var u uint8 = 17
	for i := 0; i < 6; i++ {
		u = u*u + 1
		fmt.Print(u, " ")
	}
	fmt.Println()
	var x uint8 = 0xAB
	fmt.Printf("%d %x %X %d\n", ^x, x, x, x)
}
