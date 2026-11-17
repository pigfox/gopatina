// Command slices exercises slices: make, append, len, indexing and range.
package main

import "fmt"

type Item struct {
	Name  string
	Price int
}

func total(items []Item) int {
	sum := 0
	for _, it := range items {
		sum += it.Price
	}
	return sum
}

func double(xs []int) {
	for i := range xs {
		xs[i] *= 2
	}
}

func squares(n int) []int {
	out := make([]int, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, i*i)
	}
	return out
}

func names(items []Item) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.Name)
	}
	return out
}

func main() {
	xs := []int{3, 1, 4, 1, 5}
	double(xs)
	fmt.Println(xs, len(xs))

	sq := squares(5)
	fmt.Println(sq)

	grid := make([]int, 4)
	grid[2] = 7
	fmt.Println(grid)

	items := []Item{{"pen", 3}, {"book", 12}}
	items = append(items, Item{Name: "lamp", Price: 30})
	fmt.Println(total(items), names(items))
	items[0].Price = 4
	fmt.Println(total(items))

	for i, x := range xs {
		if x > 5 {
			xs = append(xs, i)
		}
	}
	fmt.Println(xs)

	for i, x := range xs {
		xs[i] = x + 1
	}
	fmt.Println(xs)

	words := []string{"a", "b"}
	for _, w := range words {
		fmt.Print(w, " ")
	}
	fmt.Println()
	more := []string{"c", "d"}
	words = append(words, more...)
	fmt.Println(words, len(more))

	fs := []float64{1.5, 2, 1e7}
	bs := []bool{true, false}
	fmt.Println(fs, bs, []string{})
}
