// Command owned_address_kept keeps a variable's address while the variable
// is still used by name, as pointers_more did while such a variable lived
// in a shared cell (RFC 0013). The pointer and the name are two owners of
// one value, which one owner per object refuses (RFC 0017): in Rust the
// pointer borrows the variable, which is not used by name meanwhile.
// (A local borrow p := &x used beside x is refused by the borrow checker
// instead: testdata/reject/owned_local_borrow_aliased.)
package main

import "fmt"

func main() {
	cx := 5
	cp := &cx
	*cp = 7
	fmt.Println(*cp, cp == &cx) // cx used again while cp still is
	fmt.Println(cx)
}
