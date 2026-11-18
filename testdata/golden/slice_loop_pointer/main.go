package main

import "fmt"

func clearSlice(p *[]int) {
	reset(p)
}

func reset(p *[]int) {
	*p = nil
}

func markFirst(s []int) {
	s[0] = 9
}

func show(s []int) {
	fmt.Println(s)
}

func main() {
	s := []int{1, 2, 3}
	for i := 0; i < len(s); i++ {
		show(s[i:])
		clearSlice(&s)
	}
	fmt.Println("done")

	t := []int{1, 2}
	for i := 0; i < len(t); i++ {
		markFirst(t)
		show(t[i:])
	}
}
