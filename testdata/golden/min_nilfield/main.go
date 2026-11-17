package main

import "fmt"

type C struct {
	Name string
	Tags []string
}

func main() {
	c := C{Name: "x", Tags: nil}
	fmt.Println(c.Name, len(c.Tags))
}
