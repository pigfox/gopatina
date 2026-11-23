// An error checked in an if's init statement and handed straight back,
// if err := f(); err != nil { return ..., err }, is Rust's f()?.
package main

import (
	"errors"
	"fmt"
)

func validate(n int) error {
	if n < 0 {
		return errors.New("negative")
	}
	return nil
}

func half(n int) (int, error) {
	if n%2 != 0 {
		return 0, fmt.Errorf("%d is odd", n)
	}
	return n / 2, nil
}

func total(n int) (int, error) {
	if err := validate(n); err != nil {
		return 0, err
	}
	if _, err := half(n); err != nil {
		return 0, err
	}
	return n * 2, nil
}

func check(n int) error {
	if err := validate(n); err != nil {
		return err
	}
	return nil
}

func main() {
	for _, n := range []int{4, -2, 3} {
		v, err := total(n)
		if err != nil {
			fmt.Println("error:", err, check(n) == nil)
			continue
		}
		fmt.Println(v)
	}
}
