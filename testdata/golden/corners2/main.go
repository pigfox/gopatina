// Command corners2 exercises more corners of the subset.
package main

import (
	"errors"
	"fmt"
)

const _ = 1

// Stack holds numbers.
//
// The second paragraph of its documentation is kept too.
type Stack struct {
	items []int
	Name  string
}

func (s *Stack) Combine(firstArgumentName int, secondArgumentName int, thirdArgumentName int) int {
	s.items = append(s.items, firstArgumentName, secondArgumentName, thirdArgumentName)
	return len(s.items)
}

func (s *Stack) Merge(other *Stack, n int) {
	other.items = append(other.items, n)
	s.items = append(s.items, len(other.items))
}

type Point struct{ X, Y int }

func area(p *Point) int { return p.X * p.Y }

func divmod(a, b int) (int, int) { return a / b, a % b }

func check(n int) (int, error) {
	if n < 0 {
		return 0, errors.New("negative")
	}
	return n, nil
}

func setLast(xs *[]int, v int) {
	(*xs)[len(*xs)-1] = v
}

func main() {
	var s Stack
	n := s.Combine(10000000000, 20000000000, 30000000000) + s.Combine(1, 2, 3) + len(s.items)
	fmt.Println(n, s.items)
	s.items[len(s.items)-1] = 99
	fmt.Println(s.items[len(s.items)-1])
	other := Stack{Name: "other"}
	s.Merge(&other, other.items2())
	fmt.Println(len(s.items), len(other.items))

	p := Point{3, 4}
	q := &p
	fmt.Println(area(q), area(&p))

	a, b := 0, 0
	a, _ = divmod(17, 5)
	_, b = divmod(17, 5)
	fmt.Println(a, b)

	xs := []int{1, 2, 3}
	setLast(&xs, 7)
	fmt.Println(xs)

	x := 2
	y := 3
	switch x {
	case y:
		fmt.Println("same")
	case 2:
		fmt.Println("two")
	}

	for _, r := range s.Name + "ab" {
		fmt.Print(r, " ")
	}
	for i := range other.Name {
		fmt.Print(i)
	}
	fmt.Println()

	v, err := check(-1)
	if err != nil {
	}
	fmt.Println(v)
	z, err := check(-4)
	if err != nil {
		// keep the zero value
	}
	fmt.Println(z)
	fallback := 0
	w, err := check(-2)
	if err != nil {
		fallback = 42
	}
	fmt.Println(w, fallback)
	if _, err := check(-3); err != nil {
		if len(xs) > 100 {
			panic("unreachable: " + err.Error())
		}
	}
	u, err := check(5)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(u)
}

func (s *Stack) items2() int { return 5 }
