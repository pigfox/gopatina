// Command print_structs prints struct values with fmt's %v, as Println,
// Print, Printf and Sprint do: {f1 f2 ...}, each field by its own %v, a
// nested struct in braces and a slice in brackets.
package main

import (
	"fmt"
	"math"
)

type point struct {
	X, Y int
}

type reading struct {
	Name  string
	Value float64
	Ok    bool
	Count uint8
	Delta int64
}

type segment struct {
	From, To point
	Label    string
}

type series struct {
	Name   string
	Values []float64
	Tags   []string
}

type small struct {
	F float32
	U uint
}

type empty struct{}

var calls int

func origin() point {
	calls++
	return point{0, 0}
}

func main() {
	p := point{1, -2}
	fmt.Println(p)
	fmt.Println("point:", p, "done")
	fmt.Printf("%v|\n", p)
	s := fmt.Sprint(p)
	fmt.Println(len(s), s)

	r := reading{"temp sensor", 21.5, true, 255, -7}
	fmt.Println(r)
	fmt.Println(reading{"", math.NaN(), false, 0, math.MinInt64})
	fmt.Println(reading{"big", 1e21, true, 1, 0}, reading{"small", 1e-7, false, 2, 1})
	fmt.Println(reading{"inf", math.Inf(-1), true, 3, 3})

	seg := segment{point{1, 2}, point{3, 4}, "a to b"}
	fmt.Println(seg)
	fmt.Println(seg.From, seg.To)

	sr := series{"empty", nil, []string{}}
	fmt.Println(sr)
	sr = series{"three", []float64{1, 2.5, -0.125}, []string{"x", "y z"}}
	fmt.Println(sr)

	fmt.Println(small{0.1, 7}, small{float32(math.Inf(1)), 0})
	fmt.Println(empty{})
	fmt.Println(origin(), calls)
}
