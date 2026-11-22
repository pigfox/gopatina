// Named non-struct types and type aliases (RFC 0007): methods, constants,
// conversions, assignability, printing through String and Error, and the
// underlying type's exact arithmetic.
package main

import "fmt"

// Op is an enumeration with a String method.
type Op int

const (
	Add Op = iota
	Sub
	Mul
)

func (o Op) String() string {
	switch o {
	case Add:
		return "+"
	case Sub:
		return "-"
	}
	return fmt.Sprintf("Op(%d)", int(o))
}

// Apply is a value-receiver method with parameters.
func (o Op) Apply(a, b int) int {
	switch o {
	case Add:
		return a + b
	case Sub:
		return a - b
	}
	return a * b
}

type Small int32

type Octet uint8

type Big int64

type Celsius float64

func (c Celsius) Fahrenheit() float64 { return float64(c)*9/5 + 32 }

type Name string

func (n Name) Greet(other Name) string { return "hi " + string(other) + ", I am " + string(n) }

// Shout changes its own copy of the receiver.
func (n Name) Shout() Name {
	n += "!"
	return n
}

// Path is a named slice with a pointer-receiver method.
type Path []int

func (p *Path) Push(x int) { *p = append(*p, x) }

func (p Path) Sum() int {
	s := 0
	for _, x := range p {
		s += x
	}
	return s
}

// Code has both Error and String; fmt prints Error.
type Code int

func (c Code) Error() string  { return "code error" }
func (c Code) String() string { return "code string" }

// Lazy has a pointer-receiver String, which fmt does not use for a value.
type Lazy int

func (l *Lazy) String() string { return "lazy" }

type Point struct{ X, Y int }

// Pair has Point's fields and none of its methods.
type Pair Point

func (p Pair) Swap() Pair { return Pair{X: p.Y, Y: p.X} }

type Count = int

type Ints = []int

type Label = Name

type Meters Big

func total(p Path) int { return p.Sum() }

func double(b Big) Big { return b * 2 }

func newPath() Path { return []int{7, 8} }

func main() {
	fmt.Println(Add, Sub, Mul, Op(7))
	fmt.Printf("%v|%s|%d|%5v|%-4s|\n", Sub, Sub, Sub, Add, Add)
	fmt.Println(Mul.Apply(6, 7), Op(1).Apply(6, 7))
	op := Sub
	po := &op
	fmt.Println(po.Apply(1, 2), (&op).Apply(3, 4))
	fmt.Printf("%.1s|%4.2v|\n", Mul, Mul)

	var s Small = 2147483647
	s++
	var o Octet = 250
	o += 10
	var b Big = -9223372036854775807
	b -= 2
	neg := Small(-1)
	fmt.Println(s, o, b, s/neg, o>>2, Big(3)%Big(-2))

	c := Celsius(100)
	nan := Celsius(0)
	nan = nan / nan
	fmt.Println(c.Fahrenheit(), c, nan == nan, nan < c, Celsius(-40).Fahrenheit())

	n := Name("Ada")
	var l Label = "Bob"
	fmt.Println(n.Greet(l), len(n), n[0], n+"!", n < l, len([]byte(n)))
	pn := &n
	fmt.Println(pn.Shout(), n)
	for i, r := range n {
		fmt.Print(i, r, " ")
	}
	fmt.Println()

	var p Path
	p.Push(3)
	p.Push(4)
	pp := &p
	pp.Push(5)
	fmt.Println(pp.Sum())
	fmt.Println(p, len(p), p[1], p.Sum(), total(p), total([]int{1, 2}))
	q := newPath()
	q = append(q, 9)
	fmt.Println(q)
	var ints Ints = q
	fmt.Println(ints, Path(ints).Sum())

	fmt.Println(Code(3), Lazy(4))
	fmt.Printf("%v %s %d\n", Code(3), Code(3), Code(3))

	pr := Pair(Point{X: 1, Y: 2}).Swap()
	pt := Point(pr)
	fmt.Println(pr.X, pr.Y, pt.X, pt.Y)

	var k Count = 5
	m := Meters(double(Big(k)))
	fmt.Println(k, m, double(10), Big(m)+1)
}
