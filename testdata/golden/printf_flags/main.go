package main

import "fmt"

func main() {
	fmt.Printf("[%5d][%-5d][%05d][%+d][%+5d]\n", 42, 42, 42, 42, 42)
	fmt.Printf("[%5d][%-5d][%05d][%+d][%+5d]\n", -42, -42, -42, -42, -42)
	fmt.Printf("[%x][%X][%-8x|]\n", 255, 255, 255)
	fmt.Printf("[%x][%X]\n", -255, -255)
	fmt.Printf("[%10s][%-10s][%.3s][%10.3s]\n", "hello", "hello", "hello", "hello")
	fmt.Printf("[%t][%5t][%-7t]\n", true, false, true)
	fmt.Printf("[%c][%5c][%-3c]\n", 'x', 'y', 'z')
	fmt.Printf("[%%]\n")
	fmt.Printf("[%v][%5v][%-5v]\n", 7, 7, 7)
	fmt.Printf("[%v][%10v][%-10v]\n", "s", "s", "s")
	fmt.Printf("[%v][%v]\n", true, 'a')
	fmt.Printf("[%d][%d]\n", int64(-9223372036854775808), uint8(200))
}
