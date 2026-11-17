package main

import "fmt"

type C struct {
	N string
	T []int
}

func mk() C { return C{N: "a", T: nil} }

func main() {
	c := mk()
	c.T = nil
	fmt.Println(len(c.T), c.N)
}
