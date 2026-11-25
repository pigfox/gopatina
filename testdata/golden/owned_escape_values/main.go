// Command owned_escape_values returns the addresses of local values that
// nothing uses after: each moves into a new Box at its & (RFC 0017), as
// Rust's Box::new(x). Two values declared together each take their own
// box, and so does a slice; a slice through its pointer is indexed and
// measured in place.
package main

import "fmt"

// Ints is a slice of numbers.
type Ints []int

// Label is a named string.
type Label string

func pair() (*string, *string) {
	a, b := "a", "b"
	return &a, &b
}

func labels(n int) (*Label, *Label) {
	var x, y Label = "x", Label(fmt.Sprint(n))
	return &x, &y
}

func mk(n int) *Ints {
	ns := Ints{n, n + 1, n + 2}
	return &ns
}

func words() *[]string {
	w := []string{"go", "rust"}
	return &w
}

func main() {
	a, b := pair()
	*a, *b = *b, *a
	*a += "!"
	fmt.Println(*a, *b)
	x, y := labels(7)
	*x += *y
	fmt.Println(*x, *y, len(*x))
	p := mk(4)
	(*p)[0] = 10
	fmt.Println(len(*p), (*p)[0], (*p)[2], len(*mk(1)))
	w := words()
	fmt.Println(len(*w), (*w)[1])
}
