// Command decisions exercises borrow checker decisions that the other
// programs do not: copies forced by an earlier borrow in the same call,
// value receivers that change their own copy, and loops that are not
// simple counted ranges.
package main

import "fmt"

type Msg struct {
	To   string
	Body []byte
}

// Size changes its own copy of m and keeps nothing, so a copy handed to
// it behaves like Go's.
func Size(m Msg) int {
	m.To = "nobody"
	return len(m.Body) + len(m.To)
}

type Tag struct {
	Name string
}

// Renamed returns a changed copy of t.
func (t Tag) Renamed(suffix string) Tag {
	t.Name += suffix
	return t
}

func pair(a string, b string) []string {
	return []string{b, a}
}

func shout(s string) string {
	s = s + "!"
	return s
}

func addr(s string) int {
	p := &s
	*p = *p + "?"
	return len(s)
}

func double(n int) int {
	p := &n
	*p *= 2
	return n
}

type Point struct{ X, Y int }

func norm1(p *Point) int {
	return p.X + p.Y
}

func items() []int { return []int{4, 5} }

func main() {
	m := Msg{To: "ops", Body: []byte("hi")}
	fmt.Println(Size(m), Size(m), m.To)

	t := Tag{Name: "v"}
	u := t.Renamed("1")
	fmt.Println(t.Name, u.Name)

	name := "go"
	fmt.Println(pair(name, name), shout(name), addr(name), double(21))

	p := Point{3, -4}
	fmt.Println(norm1(&p), -p.X)

	const step = 2
	for i := 0; i < 7; i += step {
		fmt.Print(i, " ")
	}
	for i := 3; i > 0; i-- {
		fmt.Print(i, " ")
	}
	for i, j := 0, 10; i < j; i, j = i+1, j-1 {
		fmt.Print(i+j, " ")
	}
	for f := 0.5; f < 2; f++ {
		fmt.Print(f, " ")
	}
	n := 3
	for i := 0; 5 > i; i++ {
		fmt.Print(i)
	}
	for i := 0; i < 6; i++ {
		i++
		fmt.Print(i)
	}
	for i := 0; i < n*2; i++ {
		fmt.Print(i)
	}
	for i := 0; i < len(items()); i++ {
		fmt.Print(i)
	}
	fmt.Println()

	s := "héllo"
	for i, r := range s {
		if i == 0 {
			s = "changed"
		}
		fmt.Print(r, " ")
	}
	fmt.Println(s)

	useLabel()
	var a, b = divmod(17, 5)
	fmt.Println(a, b)
}

func divmod(x, y int) (int, int) { return x / y, x % y }

func label(prefix string, keep string) []string {
	fmt.Print(prefix)
	return []string{keep}
}

func useLabel() {
	word := "w"
	fmt.Println(label(word, word))
}
