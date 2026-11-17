package main

import "fmt"

func main() {
	total := 0
	for i, j := 0, 10; i < j; i, j = i+1, j-2 {
		if i%2 == 0 {
			continue
		}
		total += i * j
		fmt.Println(i, j, total)
	}
	k := 0
	for k = 0; k < 20; k += 3 {
		if k == 9 {
			continue
		}
		if k > 14 {
			break
		}
		fmt.Print(k, ",")
	}
	fmt.Println(k)
	n := 100
	for n > 1 {
		if n%2 == 0 {
			n /= 2
			continue
		}
		n = 3*n + 1
		fmt.Print(n, " ")
	}
	fmt.Println()
	for i := 10; i > 0; i -= 4 {
		fmt.Print(i, " ")
	}
	fmt.Println()
}
