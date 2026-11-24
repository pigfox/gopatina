// Command method_expr_addr lends a local's address through a method
// expression, (*P).Inc(&q), with pointers of that type escaping elsewhere.
// Method expressions are not in the subset: the program is refused with
// that reason, and GoPatina does not crash on the way (it did, finding
// what the address is lent to).
package main

import "fmt"

type P struct{ X int }

func (p *P) Inc() int {
	p.X++
	return p.X
}

func mk() *P { return &P{} }

func main() {
	_ = mk()
	var q P
	fmt.Println((*P).Inc(&q), q.X)
}
