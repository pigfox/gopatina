package rules

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

const shop = `# the shop's rules
discount bulk 10 5
discount sku coffee 12

shipping 499 free-over 5000
tax 8
`

func TestParse(t *testing.T) {
	rs, err := Parse(shop)
	if err != nil {
		t.Fatal(err)
	}
	want := []Rule{
		{Kind: "bulk", Min: 10, Percent: 5},
		{Kind: "sku", SKU: "coffee", Percent: 12},
		{Kind: "shipping", Cents: 499, FreeAt: 5000},
		{Kind: "tax", Percent: 8},
	}
	if !reflect.DeepEqual(rs, want) {
		t.Errorf("Parse = %+v", rs)
	}
}

func TestParseErrors(t *testing.T) {
	for _, tc := range []struct{ text, want string }{
		{"discount bulk ten 5", "line 1: rules: syntax error: bad number ten"},
		{"discount bulk 10 x", "line 1: rules: syntax error: bad number x"},
		{"\ndiscount sku tea 101", "line 2: rules: syntax error: percentage over 100: 101"},
		{"shipping 1234567890 free-over 5", "line 1: rules: syntax error: bad number 1234567890"},
		{"shipping 499 free-over -1", "line 1: rules: syntax error: bad number -1"},
		{"tax eight", "line 1: rules: syntax error: bad number eight"},
		{"refund all", "line 1: rules: syntax error: unknown rule refund all"},
	} {
		_, err := Parse(tc.text)
		if err == nil || err.Error() != tc.want || !errors.Is(err, ErrSyntax) {
			t.Errorf("Parse(%q) = %v, want %q", tc.text, err, tc.want)
		}
	}
}

func TestPrice(t *testing.T) {
	rs, _ := Parse(shop)
	q, err := Price([]Line{{"apple", 12, 50}, {"pear", 2, 80}}, rs)
	if err != nil {
		t.Fatal(err)
	}
	want := Quote{Subtotal: 760, Discount: 30, Shipping: 499, Tax: 58, Total: 1287, Applied: []string{"5% off apple"}}
	if !reflect.DeepEqual(q, want) {
		t.Errorf("Price = %+v", q)
	}
	// coffee: both rules apply, the larger wins; over the free-shipping line.
	q, _ = Price([]Line{{"coffee", 12, 1000}}, rs)
	if q.Discount != 1440 || q.Shipping != 0 || strings.Join(q.Applied, "; ") != "12% off coffee; free shipping" {
		t.Errorf("Price = %+v", q)
	}
	if _, err := Price(nil, rs); err != ErrEmpty {
		t.Errorf("Price of no lines = %v", err)
	}
}
