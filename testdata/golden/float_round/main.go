package main

import "fmt"

func main() {
	fmt.Printf("%.0f %.0f %.0f %.0f\n", 0.5, 1.5, 2.5, 3.5)
	fmt.Printf("%.1f %.1f %.2f %.2f\n", 0.25, 0.35, 1.005, 2.675)
	fmt.Printf("%.10f\n", 0.1)
	fmt.Printf("%.20f\n", 0.1)
	fmt.Printf("%6.2f|%-6.2f|%06.2f|%+6.2f\n", 3.14159, 3.14159, 3.14159, 3.14159)
	var f float64 = 16777217
	fmt.Println(f, f+1, float64(int64(f)))
	g := 1e15
	fmt.Println(g, g*10, g*1e6, g/1e20)
	fmt.Println(float64(1<<53), float64(1<<53)+1)
	ff := 3.99
	i := int(ff)
	j := int(-ff)
	fmt.Println(i, j, int64(1e18))
	h := 0.1 + 0.2
	fmt.Println(h, h == 0.3)
}
