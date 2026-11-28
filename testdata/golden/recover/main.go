// Command recover writes recover (RFC 0020): a deferred function literal
// that calls recover stops a panic, and its function returns as Go's does,
// its named results as they are. A recovered panic prints nothing.
package main

import (
	"errors"
	"fmt"
)

func mayPanic() {
	panic("a problem")
}

func inner(n int) int {
	if n > 2 {
		panic(fmt.Sprintf("too big: %d", n))
	}
	return n * 10
}

func safe(n int) (out int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recovered:", r)
		}
	}()
	out = 1
	out = inner(n) // a panic in a callee passes through it to here
	return out
}

func divide(a, b int) int {
	defer func() {
		r := recover()
		fmt.Println("divide recovered:", r != nil, r)
	}()
	return a / b
}

func pair(fail bool) (n int, ok bool) {
	defer func() {
		recover() // stops the panic, and its value is dropped
	}()
	n, ok = 7, true
	if fail {
		panic("pair failed")
	}
	return n, ok
}

func both(fail bool) (int, bool) {
	defer func() {
		r := recover()
		fmt.Println("both: nil?", r == nil)
	}()
	if fail {
		panic("both failed")
	}
	return 3, true
}

func label(fail bool) string {
	defer func() {
		recover()
	}()
	if fail {
		panic("no label")
	}
	return "fine"
}

func echo() {
	defer func() {
		fmt.Println("echo:", recover())
	}()
	panic("echoed")
}

func failing() {
	defer func() {
		if recover() != nil {
			fmt.Println("stopped an error panic")
		}
	}()
	panic(errors.New("broken"))
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered. Error:\n", r)
		}
	}()
	fmt.Println(safe(2), safe(5))
	fmt.Println(divide(7, 2), divide(1, 0))
	n, ok := pair(false)
	fmt.Println(n, ok)
	n, ok = pair(true)
	fmt.Println(n, ok)
	m, fine := both(false)
	fmt.Println(m, fine)
	m, fine = both(true)
	fmt.Println(m, fine)
	fmt.Printf("%q %q\n", label(false), label(true))
	echo()
	failing()
	mayPanic()
	fmt.Println("After mayPanic()")
}
