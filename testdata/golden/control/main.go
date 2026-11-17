// Command control exercises if, for, switch, break and continue.
package main

import "fmt"

func classify(n int) string {
	switch {
	case n < 0:
		return "negative"
	case n == 0:
		return "zero"
	case n%2 == 0:
		return "even"
	default:
		return "odd"
	}
}

func dayName(d int) string {
	name := ""
	switch d {
	case 0, 6:
		name = "weekend"
	case 1:
		name = "monday"
	default:
		name = "weekday"
	case 5:
		name = "friday"
	}
	return name
}

func color(c string) int {
	switch c {
	case "red":
		return 1
	case "green":
		return 2
	}
	return 0
}

func main() {
	for i := -1; i < 4; i++ {
		fmt.Println(i, classify(i))
	}
	for d := 0; d < 7; d++ {
		fmt.Print(dayName(d), " ")
	}
	fmt.Println()
	fmt.Println(color("red"), color("green"), color("blue"))

	sum := 0
	for i := 0; i < 20; i++ {
		if i%3 == 0 {
			continue
		}
		if i > 15 {
			break
		}
		sum += i
	}
	fmt.Println("sum:", sum)

	n := 27
	steps := 0
	for n != 1 {
		if n%2 == 0 {
			n /= 2
		} else {
			n = 3*n + 1
		}
		steps++
	}
	fmt.Println("collatz steps:", steps)

	i := 0
	for {
		i++
		if i*i > 50 {
			break
		}
	}
	fmt.Println("first square over 50:", i*i)

	for j := 10; j > 0; j -= 3 {
		if j == 4 {
			continue
		}
		fmt.Print(j, " ")
	}
	fmt.Println()

	for k := 0; k < 3; k++ {
		switch k {
		case 1:
			fmt.Println("one")
			break
		default:
			fmt.Println("other", k)
		}
	}

	if x := 5; x > 3 {
		fmt.Println("big", x)
	} else if x > 1 {
		fmt.Println("medium")
	} else {
		fmt.Println("small")
	}

	for i := range 3 {
		fmt.Print(i)
	}
	fmt.Println()

	limit := 4
	for i := 0; i < limit; i++ {
		limit--
		fmt.Print(i, limit, " ")
	}
	fmt.Println()
}
