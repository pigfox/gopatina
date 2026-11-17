// Command errors exercises errors as return values, lowered to Result.
package main

import (
	"errors"
	"fmt"
	"os"
)

func parseDigit(c byte) (int, error) {
	if c < '0' || c > '9' {
		return 0, fmt.Errorf("not a digit: %c", c)
	}
	return int(c - '0'), nil
}

func parse(s string) (int, error) {
	if len(s) == 0 {
		return 0, errors.New("empty input")
	}
	n := 0
	for i := 0; i < len(s); i++ {
		d, err := parseDigit(s[i])
		if err != nil {
			return 0, err
		}
		n = n*10 + d
	}
	return n, nil
}

func check(n int) error {
	if n > 100 {
		return errors.New("too big")
	}
	return nil
}

func half(n int) (int, string, error) {
	if n%2 != 0 {
		return 0, "", fmt.Errorf("%d is odd", n)
	}
	return n / 2, "ok", nil
}

func both(s string) (int, error) {
	return parse(s)
}

func main() {
	for _, in := range []string{"42", "", "4x2", "1234"} {
		n, err := parse(in)
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		fmt.Println("parsed", n)
		if err := check(n); err != nil {
			fmt.Println("check:", err.Error())
		} else {
			fmt.Println("check ok")
		}
	}
	h, msg, err := half(10)
	if err != nil {
		fmt.Println("unexpected", err)
	}
	fmt.Println(h, msg)
	if v, _, err := half(7); err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(v)
	}
	ignored, _ := parse("x")
	fmt.Println("ignored:", ignored)
	check(500)
	n, err := both("77")
	if err != nil {
		os.Exit(1)
	}
	fmt.Println(n)
	_, err = both("bad")
	if err != nil {
		fmt.Println("fatal:", err)
		os.Exit(3)
	}
}
