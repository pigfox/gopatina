// Command ptrslice grows slices through pointers to them.
package main

import "fmt"

type Bag struct{ items []string }

func grow(xs *[]int) {
	*xs = append(*xs, len(*xs))
	(*xs)[0] = 9
}

func fill(b *Bag, n int) {
	for i := 0; i < n; i++ {
		b.items = append(b.items, fmt.Sprint(i))
	}
}

func main() {
	xs := []int{7}
	grow(&xs)
	grow(&xs)
	fmt.Println(xs)
	var b Bag
	fill(&b, 3)
	fmt.Println(b.items, len(b.items))
	p := &xs
	*p = append(*p, 5)
	fmt.Println(len(xs))
}
