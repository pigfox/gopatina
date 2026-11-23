// fmt verbs and flags: %x of strings and []byte, the 0 and # flags of %x,
// slices printed through String and Error, and a width on a slice, which Go
// applies to each element (GoPatina padded the whole slice until
// 2026-11-22).
package main

import "fmt"

type Op int

func (o Op) String() string {
	switch o {
	case 0:
		return "+"
	case 1:
		return "-"
	}
	return "*"
}

type E int

func (e E) Error() string { return "e!" }

// Pt has a String method on a struct.
type Pt struct{ X int }

func (p Pt) String() string { return fmt.Sprint("pt", len("x")) }

type Name string

func (n Name) String() string { return "name " + string(n) }

func main() {
	a, b, c := 255, -255, 0
	var u uint8 = 10
	fmt.Printf("|%08x|%08X|%08x|%8x|%-8x|%#x|%#X|%#08x|%#x|%#8x|%08x|%#x|%#x|%03x|\n", a, a, b, b, b, a, a, a, b, b, u, u, c, b)
	fmt.Printf("|%-#8x|%0-8x|%#-8X|\n", a, b, b)
	s := "hié"
	bs := []byte("hi")
	fmt.Printf("|%x|%X|%8x|%-8x|%x|%X|%08x|%#x|%#X|\n", s, s, "ab", "ab", bs, bs, "ab", "ab", bs)
	fmt.Printf("|%x|%5x|%-5X|%x|\n", Op(1), Op(2), Op(0), Name("é"))
	empty := ""
	fmt.Printf("|%#x|%#08x|%#-8x|%08x|%#08X|%#20x|%x|%#x|\n", empty, "ab", "ab", empty, "ab", "ab", []byte{}, []byte{})
	// []byte(s) in hex lends s's bytes; nothing is copied.
	word := "gö"
	fmt.Printf("|%x|%#X|%6x|%x|\n", []byte(word), []byte(word), []byte("ab"), []byte(empty))
	pt := Pt{X: 2}
	fmt.Println([]Op{0, 1, 2}, []E{1, 2}, []Name{"a"}, []Pt{{X: 1}}, pt)
	fmt.Printf("%v|%s|%6v|%-3v|\n", []Op{0, 1}, []Op{2}, []Op{0}, []E{3})
	fmt.Print([]Op{1}, []Op{}, "\n")
	xs := []int{1, -20}
	ss := []string{"a", "bc"}
	fs := []float64{1.5, 2}
	fmt.Printf("|%6v|%-5v|%3v|%5v|%4v|\n", xs, xs, ss, fs, []bool{true})
	fmt.Println(fmt.Sprintf("%8v|%-4v|", []bool{false}, []string{"x"}))
}
