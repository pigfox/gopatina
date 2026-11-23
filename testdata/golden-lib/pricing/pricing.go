// Package pricing quotes orders: a library crate GoPatina translates
// (RFC 0014), checked against the Go package by check/.
package pricing

import (
	"errors"
	"fmt"
)

// ErrEmpty is returned for an order with no lines.
var ErrEmpty = errors.New("pricing: empty order")

// ErrQuantity is wrapped for a line with a quantity below one.
var ErrQuantity = errors.New("pricing: bad quantity")

// Cents is an amount of money.
type Cents int

// Line is one line of an order.
type Line struct {
	SKU      string
	Quantity int
	Price    Cents
}

// Order is a list of lines with a discount rule.
type Order struct {
	Lines   []Line
	MinQty  int
	Percent int
	note    string
}

// Validate reports the first line with a bad quantity.
func (o Order) Validate() error {
	if len(o.Lines) == 0 {
		return ErrEmpty
	}
	for i, l := range o.Lines {
		if l.Quantity < 1 {
			return fmt.Errorf("line %d (%s): %w", i, l.SKU, ErrQuantity)
		}
	}
	return nil
}

// Add appends a line.
func (o *Order) Add(sku string, qty int, price Cents) {
	o.Lines = append(o.Lines, Line{SKU: sku, Quantity: qty, Price: price})
}

// Total returns the order's total, after the discount on lines of at least
// MinQty items.
func (o Order) Total() (Cents, error) {
	if err := o.Validate(); err != nil {
		return 0, err
	}
	var total Cents
	for _, l := range o.Lines {
		c := Cents(l.Quantity) * l.Price
		if l.Quantity >= o.MinQty {
			c -= c * Cents(o.Percent) / 100
		}
		total += c
	}
	return total, nil
}

// Label names a line.
func Label(l Line) string { return fmt.Sprintf("%s x%d at %d", l.SKU, l.Quantity, l.Price) }

// Bulk returns the lines of at least min items, in order.
func Bulk(lines []Line, min int) []Line {
	var out []Line
	for _, l := range lines {
		if l.Quantity >= min {
			out = append(out, l)
		}
	}
	return out
}

// Tag keeps a note on the order and returns its label.
func Tag(o Order, note string) string {
	o.note = note
	return note + ": " + fmt.Sprint(len(o.Lines))
}

// Reprice returns the lines with every price changed by deltaCents, and an
// error for a price that would fall below zero.
func Reprice(lines []Line, deltaCents Cents, minimumQuantity int, keepUnchanged bool) ([]Line, error) {
	var out []Line
	for _, l := range lines {
		if l.Quantity >= minimumQuantity || !keepUnchanged {
			l.Price += deltaCents
		}
		if l.Price < 0 {
			return nil, fmt.Errorf("%s: price below zero", l.SKU)
		}
		out = append(out, l)
	}
	return out, nil
}
