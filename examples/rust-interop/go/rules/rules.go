// Package rules prices an order under a small rules language: discounts on
// bulk lines and on particular products, a shipping charge waived above a
// threshold, and tax. It is written in Go and compiled by GoPatina into the
// Rust crate examples/rust-interop/rules, which a hand-written Rust
// application uses (RFC 0014).
//
// A rules text has one rule per line; blank lines and lines starting with
// # are ignored:
//
//	discount bulk <min-quantity> <percent>
//	discount sku <sku> <percent>
//	shipping <cents> free-over <cents>
//	tax <percent>
package rules

import (
	"errors"
	"fmt"
	"strings"
)

// ErrSyntax is wrapped by every error Parse returns.
var ErrSyntax = errors.New("rules: syntax error")

// ErrEmpty is returned by Price for an order with no lines.
var ErrEmpty = errors.New("rules: empty order")

// A Rule is one parsed rule.
type Rule struct {
	Kind    string // "bulk", "sku", "shipping" or "tax"
	SKU     string // the product of a "sku" discount
	Min     int    // the quantity of a "bulk" discount
	Percent int    // the percentage of a discount or of tax
	Cents   int    // the shipping charge
	FreeAt  int    // the subtotal from which shipping is free
}

// A Line is one line of an order.
type Line struct {
	SKU      string
	Quantity int
	Cents    int // the price of one item
}

// A Quote is the priced order, in cents.
type Quote struct {
	Subtotal int
	Discount int
	Shipping int
	Tax      int
	Total    int
	Applied  []string // the rules that changed the price, in order
}

// Parse reads a rules text.
func Parse(text string) ([]Rule, error) {
	var rules []Rule
	for i, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		r, err := parseLine(words(line))
		if err != nil {
			return nil, fmt.Errorf("line %d: %w: %s", i+1, ErrSyntax, err.Error())
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// words splits a line on spaces, dropping empty words.
func words(line string) []string {
	var out []string
	for _, w := range strings.Split(line, " ") {
		if w != "" {
			out = append(out, w)
		}
	}
	return out
}

func parseLine(w []string) (Rule, error) {
	switch {
	case len(w) == 4 && w[0] == "discount" && w[1] == "bulk":
		min, err := number(w[2])
		if err != nil {
			return Rule{}, err
		}
		pct, err := percent(w[3])
		if err != nil {
			return Rule{}, err
		}
		return Rule{Kind: "bulk", Min: min, Percent: pct}, nil
	case len(w) == 4 && w[0] == "discount" && w[1] == "sku":
		pct, err := percent(w[3])
		if err != nil {
			return Rule{}, err
		}
		return Rule{Kind: "sku", SKU: w[2], Percent: pct}, nil
	case len(w) == 4 && w[0] == "shipping" && w[2] == "free-over":
		cents, err := number(w[1])
		if err != nil {
			return Rule{}, err
		}
		free, err := number(w[3])
		if err != nil {
			return Rule{}, err
		}
		return Rule{Kind: "shipping", Cents: cents, FreeAt: free}, nil
	case len(w) == 2 && w[0] == "tax":
		pct, err := percent(w[1])
		if err != nil {
			return Rule{}, err
		}
		return Rule{Kind: "tax", Percent: pct}, nil
	}
	return Rule{}, errors.New("unknown rule " + strings.Join(w, " "))
}

// number reads a whole number of at most nine digits.
func number(s string) (int, error) {
	if s == "" || len(s) > 9 {
		return 0, errors.New("bad number " + s)
	}
	n := 0
	for i := 0; i < len(s); i++ {
		d := s[i]
		if d < '0' || d > '9' {
			return 0, errors.New("bad number " + s)
		}
		n = n*10 + int(d-'0')
	}
	return n, nil
}

// percent reads a percentage from 0 to 100.
func percent(s string) (int, error) {
	n, err := number(s)
	if err != nil {
		return 0, err
	}
	if n > 100 {
		return 0, errors.New("percentage over 100: " + s)
	}
	return n, nil
}

// Price prices the lines under the rules. Each line takes the largest
// discount that applies to it; shipping and tax apply to the discounted
// subtotal; amounts round down to whole cents.
func Price(lines []Line, rules []Rule) (Quote, error) {
	if len(lines) == 0 {
		return Quote{}, ErrEmpty
	}
	var q Quote
	for _, l := range lines {
		amount := l.Quantity * l.Cents
		q.Subtotal += amount
		best, why := 0, ""
		for _, r := range rules {
			applies := r.Kind == "bulk" && l.Quantity >= r.Min || r.Kind == "sku" && r.SKU == l.SKU
			if applies && r.Percent > best {
				best = r.Percent
				why = fmt.Sprintf("%d%% off %s", r.Percent, l.SKU)
			}
		}
		if best > 0 {
			q.Discount += amount * best / 100
			q.Applied = append(q.Applied, why)
		}
	}
	net := q.Subtotal - q.Discount
	for _, r := range rules {
		switch r.Kind {
		case "shipping":
			if net < r.FreeAt {
				q.Shipping += r.Cents
			} else {
				q.Applied = append(q.Applied, "free shipping")
			}
		case "tax":
			q.Tax += net * r.Percent / 100
		}
	}
	q.Total = net + q.Shipping + q.Tax
	return q, nil
}
