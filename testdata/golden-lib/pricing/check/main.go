// Command check prints what the pricing package computes on fixed inputs;
// check/main.rs prints the same through the Rust crate GoPatina generates.
package main

import (
	"errors"
	"fmt"

	"github.com/pigfox/gopatina/testdata/golden-lib/pricing"
)

func main() {
	var o pricing.Order
	o.MinQty, o.Percent = 10, 5
	o.Add("apple", 12, 50)
	o.Add("pear", 2, 80)
	total, err := o.Total()
	fmt.Println(int(total), err == nil)
	fmt.Println(pricing.Label(o.Lines[0]), int(pricing.Cents(-1234)))
	fmt.Println(len(pricing.Bulk(o.Lines, 10)), pricing.Tag(o, "rush"))
	_, err = pricing.Order{}.Total()
	fmt.Println(err, err == pricing.ErrEmpty)
	re, err := pricing.Reprice(o.Lines, 5, 10, true)
	fmt.Println(len(re), int(re[0].Price), int(re[1].Price), err == nil)
	_, err = pricing.Reprice(o.Lines, -60, 0, false)
	fmt.Println(err)
	o.Add("plum", 0, 30)
	_, err = o.Total()
	fmt.Println(err, errors.Is(err, pricing.ErrQuantity), errors.Unwrap(err) == pricing.ErrQuantity)
}
