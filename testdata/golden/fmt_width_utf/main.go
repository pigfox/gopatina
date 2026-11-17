package main

import "fmt"

func main() {
	for _, s := range []string{"a", "é", "日本", "😀", ""} {
		fmt.Printf("[%4s][%-4s][%.1s][%3.2s]\n", s, s, s, s)
		fmt.Printf("[%4v][%-4v]\n", s, s)
	}
	fmt.Printf("[%3c][%-3c]\n", '日', 'é')
	fmt.Printf("[%5t]\n", true)
	fmt.Printf("%s|%v|%d\n", "x", "y", 5)
}
