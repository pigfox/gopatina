package main

import (
	"fmt"
	"os"
)

func show(s []int) {
	fmt.Println(s, len(s))
}

func showSuffix(s []int, i int) {
	(show)((s)[(i):])
}

func readA(s []int, depth int) {
	if depth > 0 {
		readB(s, depth-1)
		return
	}
	n := 0
	for _, x := range s {
		n += x
	}
	if len(s) != 0 {
		n += s[0]
	}
	fmt.Println(n)
}

func readB(s []int, depth int) {
	readA(s, depth)
}

func read(s []int) {
	readA((s), 2)
}

func forwardSuffix(s []int) {
	read((s[1:]))
}

func main() {
	s := []int{10, 20, 30}
	i := 1
	showSuffix(s, i)
	show(s[3:])
	s[1] = 99
	show(s[i:])
	fmt.Println(s)
	var empty []int
	show(empty[0:])
	show(empty[os.O_RDONLY:])
	const start byte = 1
	nums := []int{9, 5, 2, 0}
	forwardSuffix(nums[start:])
}
