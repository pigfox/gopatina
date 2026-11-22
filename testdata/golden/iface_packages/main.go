// Command iface_packages uses an interface across packages (RFC 0012 with
// RFC 0009): types of both packages are converted to another package's
// interface, which that package's functions call and compare.
package main

import (
	"fmt"

	"example.com/ifacepkgs/shapes"
)

// Tri is a triangle, declared in main.
type Tri struct{ B, H int }

func (t Tri) Area() int { return t.B * t.H / 2 }

func (t Tri) Name() string { return "tri" }

func main() {
	all := []shapes.Shape{shapes.Rect{W: 2, H: 3}, shapes.Unit(4), Tri{B: 6, H: 5}}
	fmt.Println(shapes.Total(all))
	big := shapes.Biggest(all)
	fmt.Printf("%v %T %v\n", big, big, shapes.Biggest(nil))
	if u, ok := big.(shapes.Unit); ok {
		fmt.Println("unit", int(u))
	}
}
