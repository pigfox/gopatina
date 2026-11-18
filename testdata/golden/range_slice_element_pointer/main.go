package main

import "fmt"

func set(p *[]int, i int) {
	(*p)[i] = 9
}

func forward(p *[]int, i int) {
	set(p, i)
}

func main() {
	s := []int{1, 2, 3}
	for i, v := range s {
		fmt.Println(v)
		if i+1 < len(s) {
			set(&s, i+1)
		}
	}
	for i := range s {
		forward(&s, i)
		fmt.Println(i, s[i])
	}
}
