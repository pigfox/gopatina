// Command double_borrow passes the same variable twice to a function
// that changes both.
package main

import "fmt"

type Acc struct{ N int }

func move(from, to *Acc) {
	from.N--
	to.N++
}

func main() {
	a := Acc{N: 1}
	move(&a, &a)
	fmt.Println(a.N)
}
