package main

import "fmt"

func classify(n int) string {
	switch {
	case n < 0:
		return "neg"
	case n == 0:
		return "zero"
	case n < 10, n == 100:
		return "small-or-100"
	default:
		return "big"
	}
}

func grade(c byte) int {
	switch c {
	default:
		return -1
	case 'a', 'e', 'i', 'o', 'u':
		return 1
	case 'y':
		return 2
	}
}

func main() {
	for _, n := range []int{-5, 0, 5, 100, 50} {
		fmt.Println(classify(n))
	}
	for i := 0; i < 5; i++ {
		switch i {
		case 0:
			fmt.Println("zero")
		case 1, 2:
			fmt.Println("one-two")
			if i == 2 {
				break
			}
			fmt.Println("one only")
		default:
			fmt.Println("other", i)
		}
	}
	s := "hey"
	for i := 0; i < len(s); i++ {
		fmt.Println(grade(s[i]))
	}
	switch x := 5; x {
	case 5:
		x := "shadow"
		fmt.Println(x)
	}
	switch s {
	case "hey":
		fmt.Println("matched")
	case "hi":
	}
}
