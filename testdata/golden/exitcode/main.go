// Command exitcode ends with os.Exit after output without a final newline.
package main

import (
	"fmt"
	"os"
)

func main() {
	total := 0
	for i := 1; i <= 4; i++ {
		total += i
	}
	fmt.Print("total=", total)
	if total > 5 {
		os.Exit(4)
	}
	fmt.Println("unreachable")
}
