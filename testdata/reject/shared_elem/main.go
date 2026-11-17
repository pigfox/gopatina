// Command shared_elem copies a row out of a slice of slices; the copy
// shares the row's backing array.
package main

import "fmt"

func main() {
	grid := [][]int{{1, 2}, {3, 4}}
	row := grid[0]
	row[0] = 9
	fmt.Println(grid[0][0], row[0])
}
