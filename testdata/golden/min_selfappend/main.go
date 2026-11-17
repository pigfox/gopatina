package main

import "fmt"

func main() {
	d := []int{1, 2}
	d = append(d, d...)
	fmt.Println(d)
}
