// Command interfaces_more exercises RFC 0012 beyond its example: methods
// with parameters and several results on struct and non-struct types,
// embedded interfaces, interfaces in fields, slices, parameters and
// results, assertions of both kinds, type switches with several types per
// case, and %v and %T of every kind of dynamic value.
package main

import "fmt"

type Named interface {
	Name() string
}

type Scaler interface {
	Named
	Scale(by int, label string) (int, string)
}

type Box struct {
	W, H int
}

func (b Box) Name() string { return "box" }

func (b Box) Scale(by int, label string) (int, string) {
	return b.W * b.H * by, label + "!"
}

type Level int

func (l Level) Name() string { return fmt.Sprintf("level%d", int(l)) }

func (l Level) Scale(by int, label string) (int, string) {
	return int(l) * by, label
}

func (l Level) String() string { return fmt.Sprintf("L%d", int(l)) }

type Pair struct {
	A int
	B string
}

// Name changes its copy of the receiver, so the copy is its own.
func (p Pair) Name() string {
	p.B += "!"
	return p.B
}

// Tag only reads its receiver, which it borrows.
type Tag struct{ S string }

func (t Tag) Name() string { return "tag " + t.S }

// box is another type whose Go name differs from Box only in case.
type box int

func (b box) Name() string { return "small box" }

// Sizer and Grower share a method name with different signatures.
type Sizer interface{ Size() int }

type Grower interface{ Size(scale int) int }

func (b Box) Size() int { return b.W + b.H }

func (t Tag) Size(scale int) int { return len(t.S) * scale }

type Inner struct{ X float64 }

type Outer struct {
	I Inner
	N uint8
}

type Empty struct{}

type Path []int

type Holder struct {
	Item Named
	Tag  string
}

func pick(i int) Named {
	if i%2 == 0 {
		return Box{W: i, H: 2}
	}
	return Level(i)
}

func kind(x any) string {
	switch v := x.(type) {
	case nil:
		return "nil"
	case int, int8:
		return fmt.Sprintln("small int", v)
	case Scaler:
		n, s := v.Scale(3, "s")
		return fmt.Sprintln("scaler", n, s, v.Name())
	case string:
		v += "?"
		return "string " + v
	case Pair:
		if v.A <= 100 {
			return fmt.Sprintln("pair", v.A, v.B)
		}
		break
	}
	return "other"
}

// sort reports which group a value falls in: an interface case beside
// concrete types in one clause.
func sort(x any) string {
	switch x.(type) {
	case Sizer, bool:
		return "sizer or bool"
	case Grower:
		return "grower"
	}
	return "neither"
}

func main() {
	var named []Named = []Named{Pair{A: 1, B: "p"}, Tag{S: "t"}, box(1), Box{W: 1, H: 2}}
	for _, nm := range named {
		fmt.Println(nm.Name(), nm)
	}
	var sz Sizer = Box{W: 4, H: 5}
	var gr Grower = Tag{S: "abc"}
	fmt.Println(sz.Size(), gr.Size(2), sort(sz), sort(true), sort(gr), sort(1))
	var items []Named
	for i := 0; i < 4; i++ {
		items = append(items, pick(i))
	}
	for _, it := range items {
		fmt.Println(it.Name(), it)
	}
	var sc Scaler = Level(7)
	n, label := sc.Scale(2, "x")
	fmt.Println(n, label, sc.Name())
	var asNamed Named = sc
	fmt.Println(asNamed.Name(), asNamed == sc, asNamed != nil)

	h := Holder{Item: Box{W: 2, H: 3}, Tag: "t"}
	h2 := h
	fmt.Println(h.Item.Name(), h == h2, h.Item == Box{W: 2, H: 3}, h.Item == Box{W: 3, H: 2})

	zero := 0.0
	values := []any{
		int8(-3), uint(7), float32(1.5), 2.25, zero / zero, true, "s", 'r',
		Pair{A: 1, B: "b"}, Outer{I: Inner{X: 0.5}, N: 9}, Empty{}, Path{1, 2}, []int{3, 4},
		Level(2), Box{W: 1, H: 1}, nil, 1000000.0, 1e21,
	}
	for _, v := range values {
		fmt.Println(v)
		fmt.Printf("%v|%T\n", v, v)
		fmt.Print(kind(v))
		fmt.Println()
	}
	fmt.Println(values[4] == values[4], values[0] == any(int8(-3)), values[1] == any(7), values[15] == nil)
	fmt.Println(kind(Pair{A: 200}), kind(Level(1)))

	b, ok := values[14].(Box)
	fmt.Println(b.W, ok)
	_, ok = values[0].(Box)
	l, ok2 := values[13].(Named)
	fmt.Println(ok, ok2, l.Name())
	m, ok3 := values[0].(Named)
	fmt.Println(ok3, m == nil)
	sc2 := values[13].(Scaler)
	n2, s2 := sc2.Scale(1, "y")
	fmt.Println(n2, s2)
	fmt.Println(values[2].(float32), values[6].(string), values[8].(Pair).B)
	var never any = 1
	o, okNever := never.(Outer)
	fmt.Println(o.N, okNever)
}
