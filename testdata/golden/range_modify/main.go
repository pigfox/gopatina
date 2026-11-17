package main

import "fmt"

func main() {
	s := []int{1, 2, 3, 4}
	for i, v := range s {
		s[len(s)-1-i] = v * 10
		fmt.Println(i, v, s)
	}
	t := []int{1, 2, 3}
	for _, v := range t {
		t = append(t, v)
	}
	fmt.Println(t)
	u := []int{1, 2, 3}
	for i, v := range u {
		v += 100
		u[i] += v
	}
	fmt.Println(u)
	w := []string{"a", "b", "c"}
	for i, x := range w {
		w[i] = x + x
		if i < 2 {
			w[i+1] = "Z"
		}
	}
	fmt.Println(w)
	n := 3
	for i := range n {
		n = 10
		fmt.Print(i, " ")
	}
	fmt.Println(n)
}
