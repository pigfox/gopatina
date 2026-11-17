package main

import (
	"fmt"
	"os"
)

func check(n int) {
	if n > 2 {
		panic("too big: " + fmt.Sprint(n))
	}
}

func main() {
	fmt.Fprintln(os.Stderr, "start")
	for i := 0; i < 10; i++ {
		fmt.Println(i)
		check(i)
	}
}
