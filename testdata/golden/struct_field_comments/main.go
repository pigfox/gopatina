// Trailing comments on struct fields are aligned the way rustfmt aligns
// them: one space after the widest field of each run of commented fields.
package main

import "fmt"

type Rule struct {
	Kind    string // "bulk" or "sku"
	SKU     string // the product of a "sku" rule
	Min     int    // the quantity of a "bulk" rule
	Percent int    // the percentage off
}

type Broken struct {
	Kind string // a run of one
	N    int
	SKU  string // a new run
	Min  int    // in it
}

type Documented struct {
	Kind string // before the doc comment
	// LongerPrice is documented: one space, and not counted.
	LongerPrice int    // after it
	Q           string // aligned with Kind
}

type Plain struct {
	A int
	B int // the only comment
}

type Wide struct {
	K                   int    // a comment long enough that aligning it would pass the line width rustfmt keeps to
	MuchLongerFieldName string // so the run stays as it is
}

func main() {
	r := Rule{Kind: "bulk", Min: 10, Percent: 5}
	b := Broken{Kind: "k", N: 1, SKU: "s", Min: 2}
	d := Documented{Kind: "d", LongerPrice: 3, Q: "q"}
	p := Plain{A: 1, B: 2}
	w := Wide{K: 4, MuchLongerFieldName: "w"}
	fmt.Println(r.Kind, r.SKU == "", r.Min, r.Percent)
	fmt.Println(b.Kind, b.N, b.SKU, b.Min, d.Kind, d.LongerPrice, d.Q, w.K, w.MuchLongerFieldName, p.A+p.B)
}
