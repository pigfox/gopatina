// Command interfaces is RFC 0012's example: interface types, conversions,
// method calls through an interface, a type switch, comparison, %v and %T,
// and a type assertion with ok. (error as an interface comes next.)
package main

import "fmt"

type Shape interface {
	Area() float64
	Name() string
}

type Circle struct{ R float64 }

func (c Circle) Area() float64 { return 3 * c.R * c.R }
func (c Circle) Name() string  { return "circle" }

type Square struct{ S float64 }

func (s Square) Area() float64  { return s.S * s.S }
func (s Square) Name() string   { return "square" }
func (s Square) String() string { return fmt.Sprintf("Square(%v)", s.S) }

func describe(x any) string {
	switch v := x.(type) {
	case nil:
		return "nil"
	case int:
		return fmt.Sprint("int ", v)
	case Shape:
		return "shape " + v.Name()
	default:
		return fmt.Sprintf("other %T", v)
	}
}

func main() {
	shapes := []Shape{Circle{1}, Square{2}}
	total := 0.0
	for _, s := range shapes {
		total += s.Area()
	}
	var a, b any = Square{2}, Square{2}
	fmt.Println(total, shapes[1], describe(3), describe(shapes[0]), describe(nil), describe("s"), a == b)
	fmt.Printf("%v %T %T %v\n", shapes[0], shapes[1], a, any(nil))
	c, ok := a.(Circle)
	fmt.Println(any(c), ok)
}
