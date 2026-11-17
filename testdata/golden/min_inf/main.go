package main

import "fmt"

func main() {
	z := 0.0
	fmt.Printf("%5.1f|%6.2f|%8f|%+f|%-7.1f|\n", 1/z, -1/z, 1/z, 1/z, 1/z)
	fmt.Printf("%5.1f|%06.1f|%5v|%+v\n", z/z, 1/z, 1/z, 1/z)
}
