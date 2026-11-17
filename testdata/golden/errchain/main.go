package main

import (
	"errors"
	"fmt"
	"os"
)

func level3(n int) (int, error) {
	if n%7 == 0 {
		return 0, errors.New("multiple of seven")
	}
	return n * 2, nil
}

func level2(n int) (int, error) {
	v, err := level3(n + 1)
	if err != nil {
		return 0, fmt.Errorf("level2(%d): %v", n, err)
	}
	return v + 1, nil
}

func level1(n int) (string, error) {
	v, err := level2(n * 3)
	if err != nil {
		return "", fmt.Errorf("level1: %v", err)
	}
	return fmt.Sprintf("<%d>", v), nil
}

func main() {
	for i := 0; i < 10; i++ {
		s, err := level1(i)
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		fmt.Println(s)
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	_, err := level1(2)
	if err != nil {
		return err
	}
	return nil
}
