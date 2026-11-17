package main

import "fmt"

func main() {
	x := 1
	fmt.Println(x)
	{
		x := x + 1
		fmt.Println(x)
		{
			x := "str"
			fmt.Println(x)
			x += "!"
			fmt.Println(x)
		}
		x++
		fmt.Println(x)
	}
	if x := x * 10; x > 5 {
		x := x - 1
		fmt.Println(x)
	} else {
		fmt.Println("no", x)
	}
	for x := 0; x < 2; x++ {
		x := x * 100
		fmt.Println(x)
	}
	fmt.Println(x)
	y, err := 1, 2
	fmt.Println(y, err)
	if y, z := 2, 3; y < z {
		fmt.Println(y, z)
	}
	fmt.Println(y)
}
