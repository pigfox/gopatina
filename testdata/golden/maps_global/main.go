// Command maps_global writes package-level maps that nothing changes
// (RFC 0019, step 2), with struct values: a static, made once, and read.
// Printing sorts the keys, so no output depends on the hash table's order.
package main

import "fmt"

type Vertex struct {
	Lat, Long float64
}

var m = map[string]Vertex{
	"Bell Labs": {40.68433, -74.39967},
	"Google":    {37.42202, -122.08408},
}

var codes = map[int]string{404: "not found", 200: "ok"}

var empty, limit = map[string]bool{}, 3

type point struct {
	x, y int
	on   bool
}

func main() {
	fmt.Println(m)
	fmt.Println(m["Google"], m["nowhere"], len(m))
	fmt.Println(m["Bell Labs"].Lat)
	if v, ok := m["Google"]; ok {
		fmt.Printf("%.2f %.2f\n", v.Lat, v.Long)
	}
	fmt.Println(codes[404], codes[500] == "", codes)
	fmt.Println(empty, len(empty), empty["x"], limit)

	grid := map[string]point{"a": {1, 2, true}}
	grid["b"] = point{3, 4, false}
	p := grid["a"]
	p.x += 10
	fmt.Println(grid, p, grid["c"])
}
