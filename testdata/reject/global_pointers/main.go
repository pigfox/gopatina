// Package-level variables holding pointers in a slice or a struct: a
// static cannot hold reference-counted cells (RFC 0013), so patina mode
// rejects them instead of writing a crate that does not build.
package main

import "fmt"

type Tag struct{ Label string }

var tags = []*Tag{{Label: "even"}, {Label: "odd"}}

type Index struct {
	Count int
	First *Tag
}

var index Index

func main() {
	fmt.Println(tags[0].Label, index.Count)
}
