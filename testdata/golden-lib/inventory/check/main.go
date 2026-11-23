// Command check prints what the inventory package computes on fixed
// inputs; check/main.rs prints the same through the Rust crate.
package main

import (
	"fmt"

	"github.com/pigfox/gopatina/testdata/golden-lib/inventory"
)

func main() {
	items := []inventory.Item{{SKU: "bolt", Count: 10}, {SKU: "nut", Count: 2}}
	out, err := inventory.Apply(items, []string{"bolt", "nut", "bolt"}, []int{-3, 5, 4})
	fmt.Println(out[0].SKU, out[0].Count, out[1].Count, err == nil)
	_, err = inventory.Apply(items, []string{"nut", "nut"}, []int{-1, -2})
	fmt.Println("failed:", err)
	fmt.Println(inventory.Report(items, []string{"bolt"}, []int{-20}))
}
