package main

import "fmt"

func main() {
	s := "héllo, 世界! 😀"
	fmt.Println(len(s))
	for i, r := range s {
		fmt.Printf("%d %d %c\n", i, r, r)
	}
	for i := 0; i < len(s); i++ {
		fmt.Printf("%d:%x ", i, s[i])
	}
	fmt.Println()
	n := 0
	for range s {
		n++
	}
	fmt.Println(n)
	fmt.Printf("%c %c %c\n", 'é', 0x4e16, 128512)
	fmt.Printf("%5s|%-5s|%.2s\n", "世界", "é", "世界x")
	var r rune = 'a'
	r += 2
	fmt.Printf("%d %c\n", r, r)
	b := s[1]
	fmt.Printf("%d %c\n", b, rune(b))
}
