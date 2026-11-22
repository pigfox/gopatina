// Command errors_packages calls another package's functions and methods
// that return errors, in every form patina mode lowers: checked at once,
// passed on with ?, in an if statement's init, kept as a value, and as a
// value of an expression. Calls of another package were once left out of
// the error plan, and their Results destructured as tuples.
package main

import (
	"fmt"

	"example.com/errpkgs/parse"
)

func total(ss []string) (int, error) {
	t := 0
	for _, s := range ss {
		n, err := parse.Number(s)
		if err != nil {
			return 0, err
		}
		t += n
	}
	return t, nil
}

func main() {
	for _, s := range []string{"42", "", "4x2", "123"} {
		n, err := parse.Number(s)
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		fmt.Println(n, parse.Check(n))
	}
	t, err := total([]string{"1", "2", "3"})
	fmt.Println(t, err)
	_, err = total([]string{"1", "b"})
	fmt.Println(err)
	if v, err := parse.Digit('7'); err == nil {
		fmt.Println("digit", v)
	}
	p := parse.Parser{Limit: 50}
	for _, s := range []string{"12", "77", "z"} {
		v, err := p.Parse(s)
		fmt.Println(v, err)
	}
	if err := parse.Check(500); err != nil {
		fmt.Println("checked:", err)
	}
}
