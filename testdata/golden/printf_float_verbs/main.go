package main

import "fmt"

func main() {
	vals := []float64{3.0, 3.5, -0.5, 1234567.891, 0.000123, 1e100, -1e-100}
	for _, v := range vals {
		fmt.Printf("%v %6.1f |%8.2f|%-8.2f|%+.1f|%08.2f\n", v, v, v, v, v, v)
	}
	var f32 float64 = 16777216.5
	fmt.Println(f32, 1.5)
}
