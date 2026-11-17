// Command shared_range_value changes a range value that shares memory
// with the element it copies.
package main

import "fmt"

type Row struct {
	Cells []int
}

func main() {
	rows := []Row{{Cells: []int{1}}, {Cells: []int{2}}}
	for _, r := range rows {
		r.Cells[0] = 0
		r = Row{}
		fmt.Println(len(r.Cells))
	}
	fmt.Println(rows[0].Cells[0])
}
