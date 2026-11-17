package main

import "fmt"

type String struct{ v string }

func main() {
	s := String{"hi"}
	fmt.Println(s.v)
}
