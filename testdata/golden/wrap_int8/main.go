package main

import "fmt"

func main() {
	var a uint8 = 250
	for i := 0; i < 10; i++ {
		a += 3
		fmt.Println(a)
	}
	var b int32 = 2147483647
	b++
	fmt.Println(b)
	b = -b
	fmt.Println(b)
	var c int32 = -2147483648
	c = -c
	fmt.Println(c, c-1, c*2)
	var d int64 = -9223372036854775808
	d = -d
	fmt.Println(d)
	d--
	fmt.Println(d)
	var e uint8 = 0
	e--
	fmt.Println(e, -e)
	var m int = 9223372036854775807
	m += 1
	fmt.Println(m, m*m, m*3)
}
