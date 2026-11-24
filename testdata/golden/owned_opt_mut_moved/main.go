// Command owned_opt_mut_moved changes through pointer parameters that may
// be nil, tested with if p != nil. In Rust each is an Option<&mut T>. One
// that is not used after the if moves into the if let binding, as clippy
// asks (needless_option_as_deref); one used again is reborrowed by
// as_deref_mut.
package main

import "fmt"

// Acc accumulates.
type Acc struct{ N int }

func newAcc() *Acc { return &Acc{} }

// addIf adds v when a is not nil: a is not used after the if.
func addIf(a *Acc, v int) {
	if a != nil {
		a.N += v
	}
}

// addTwice adds v, then reads a again after the if.
func addTwice(a *Acc, v int) int {
	if a != nil {
		a.N += v
	}
	if a == nil {
		return -1
	}
	return a.N
}

// addEach adds each value, testing a in a loop.
func addEach(a *Acc, vs []int) {
	for _, v := range vs {
		if a != nil {
			a.N += v
		}
	}
}

// addPositive adds v only when it is positive.
func addPositive(a *Acc, v int) {
	if a != nil && v > 0 {
		a.N += v
	}
}

func main() {
	a := newAcc()
	addIf(a, 1)
	addIf(nil, 1)
	fmt.Println(addTwice(a, 2), addTwice(nil, 2))
	addEach(a, []int{3, 4})
	addEach(nil, []int{5})
	addPositive(a, -9)
	addPositive(a, 10)
	addPositive(nil, 10)
	fmt.Println(a.N)
}
