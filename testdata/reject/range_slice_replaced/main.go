package main

import "fmt"

func replace(p *[]int) {
	*p = []int{9, 8, 7}
}

func forward(p *[]int) {
	replace((p))
}

func values() {
	s := []int{1, 2, 3}
	for _, v := range s {
		fmt.Print(v)
		replace(&s)
	}
	fmt.Println()
}

func indices() {
	s := []int{1, 2, 3}
	for i := range s {
		fmt.Print(i)
		forward(&s)
	}
	fmt.Println()
}

func localPointer() {
	s := []int{1, 2, 3}
	for i := range s {
		fmt.Print(i)
		p := &s
		forward(p)
	}
	fmt.Println()
}

func main() {
	values()
	indices()
	localPointer()
}
