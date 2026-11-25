// Command owned_lend_through_nil lends a field of an owned pointer that may
// be nil to a call whose later argument changes the same pointer. Rust
// unwraps the pointer whole to lend its field (q.as_mut().unwrap().n), so
// nothing else may change it until the call: refused (RFC 0017). A later
// argument that only reads is computed first instead
// (testdata/golden/owned_field_lends).
package main

import "fmt"

// T holds a count.
type T struct{ n int }

func addTo(x *int, d int) { *x += d }

func bump(t *T) int {
	t.n++
	return t.n
}

func main() {
	var q *T // may be nil: an Option
	if len("go") > 1 {
		q = &T{n: 1}
	}
	addTo(&q.n, bump(q))
	fmt.Println(q.n)
}
