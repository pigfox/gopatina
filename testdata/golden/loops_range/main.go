package main

import "fmt"

func main() {
	for i := range 0 {
		fmt.Println("never", i)
	}
	for i := range -3 {
		fmt.Println("never", i)
	}
	var n int64 = 3
	for i := range n {
		fmt.Printf("%d ", i)
	}
	var m int32 = 2
	for i := range m {
		fmt.Printf("%d ", i)
	}
	fmt.Println()
	for range 2 {
		fmt.Print("x")
	}
	fmt.Println()
	s := []string{"a", "b"}
	for i := range s {
		fmt.Print(i, s[i])
	}
	for _, v := range s {
		fmt.Print(v)
	}
	for i, _ := range s {
		fmt.Print(i)
	}
	fmt.Println()
	i := 99
	for i = 0; i < 3; i++ {
	}
	fmt.Println(i)
	var k int
	for k < 3 {
		k++
	}
	for {
		k++
		if k > 10 {
			break
		}
	}
	fmt.Println(k)
}
