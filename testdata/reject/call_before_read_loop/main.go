// Command call_before_read_loop reads a variable in a loop condition and
// a switch case before a call that changes it. gc makes the call first;
// patina mode writes that order by computing the calls into locals, which
// a condition evaluated again each time round, and a case evaluated only
// when the cases before it do not match, cannot hold yet, so both are
// refused for now.
package main

import "fmt"

func addTo(p *int, v int) int {
	*p += v
	return *p
}

func main() {
	x := 1
	for x+addTo(&x, 1) < 20 {
		fmt.Println(x)
	}
	switch {
	case x+addTo(&x, 1) > 100:
		fmt.Println("big")
	}
	for addTo(&x, 1) < 30 { // the call reads x itself: no earlier read
		fmt.Println(x)
	}
}
