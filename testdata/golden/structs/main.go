// Command structs exercises structs and methods with value and pointer
// receivers, and pointers used as borrows.
package main

import "fmt"

// Point holds only integers, so it is copied, as in Go.
type Point struct {
	X, Y int
}

// Scale changes p in place: an exclusive borrow.
func (p *Point) Scale(k int) {
	p.X *= k
	p.Y *= k
}

// Sum only reads p.
func (p Point) Sum() int {
	return p.X + p.Y
}

// Moved returns a moved copy and leaves p alone.
func (p Point) Moved(dx int) Point {
	p.X += dx
	return p
}

type Person struct {
	Name string
	Age  int
	Home Point
}

func (p *Person) Birthday() {
	p.Age++
}

func (p Person) Describe() string {
	return fmt.Sprintf("%s (%d) at %d,%d", p.Name, p.Age, p.Home.X, p.Home.Y)
}

func rename(p *Person, name string) {
	p.Name = name
}

func older(a, b Person) bool {
	return a.Age > b.Age
}

func bump(n *int) {
	*n = *n + 1
}

func main() {
	p := Point{X: 1, Y: 2}
	p.Scale(3)
	q := p.Moved(10)
	fmt.Println(p.X, p.Y, p.Sum(), q.X, q == p, Point{3, 6} == p)

	alice := Person{Name: "Alice", Age: 30, Home: Point{1, 1}}
	alice.Birthday()
	rename(&alice, "Alice B.")
	bob := Person{Name: "Bob"}
	bob.Age = 25
	fmt.Println(alice.Describe())
	fmt.Println(bob.Describe(), older(alice, bob))

	ptr := &alice
	ptr.Home.Scale(5)
	ptr.Birthday()
	fmt.Println(alice.Describe())

	counter := 0
	bump(&counter)
	bump(&counter)
	fmt.Println("counter:", counter)

	box := &Point{X: 7}
	box.Scale(2)
	fmt.Println(box.X, box.Y, box.Sum())

	var zero Person
	fmt.Println(zero.Name == "", zero.Age, zero.Home.X)
}
