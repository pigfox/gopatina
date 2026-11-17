package main

import "fmt"

func empty() []int { return nil }

func take(s []int) int { return len(s) }

func main() {
	var a []int
	c := empty()
	a = append(a, 1)
	fmt.Println(len(a), len(c))
	var b []int
	c = append(c, b...)
	fmt.Println(len(c), take(nil), take(c))
	var s []string
	s = nil
	fmt.Println(len(s))
	s = append(s, "x", "y")
	s = nil
	fmt.Println(len(s))
	d := make([]int, 3)
	d[1] = 5
	d = append(d, 7)
	fmt.Println(d, len(d))
}
