// Command sorting sorts slices of integers and strings with package slices
// and package sort: Rust's sort gives Go's order, since equal integers or
// strings are the same. Go by Example's sorting.
package main

import (
	"fmt"
	"slices"
	"sort"
)

type celsius int

func sortedCopy(xs []int) []int {
	out := slices.Clone(xs)
	slices.Sort(out)
	return out
}

func main() {
	strs := []string{"c", "a", "b"}
	slices.Sort(strs)
	fmt.Println("Strings:", strs)

	ints := []int{7, 2, 4}
	slices.Sort(ints)
	fmt.Println("Ints:   ", ints)

	s := slices.IsSorted(ints)
	fmt.Println("Sorted: ", s)

	words := []string{"pear", "Apple", "fig", "apple", ""}
	sort.Strings(words)
	fmt.Println(words, len(words[0]), sort.StringsAreSorted(words))

	nums := []int{3, -1, 3, 0, -10}
	fmt.Println(sort.IntsAreSorted(nums), sortedCopy(nums), nums)
	sort.Ints(nums)
	fmt.Println(nums, sort.IntsAreSorted(nums))

	temps := []celsius{30, -5, 12}
	slices.Sort(temps)
	bytes := []byte("zebra")
	slices.Sort(bytes)
	fmt.Println(temps, slices.IsSorted(temps), bytes)
}
