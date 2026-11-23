package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

func testdata(name string) string { return filepath.Join("..", "..", "..", "testdata", name) }

func TestRun(t *testing.T) {
	for _, tc := range []struct {
		args         []string
		code         int
		stdout, errs string
	}{
		{[]string{testdata("shop.rules"), testdata("small.order")}, 0,
			"subtotal        7.60\ndiscount       -0.30\nshipping        4.99\ntax             0.58\ntotal          12.87\napplied: 5% off apple\n", ""},
		{[]string{testdata("shop.rules"), testdata("empty.order")}, 1, "", "quote: rules: empty order\n"},
		{[]string{testdata("shop.rules"), testdata("bad.order")}, 1, "", "quote: order line 2: expected <sku> <quantity> <cents>\n"},
		{[]string{testdata("bad.rules"), testdata("small.order")}, 1, "", "quote: line 2: rules: syntax error: bad number eight\n"},
		{[]string{"only-one"}, 2, "", "usage: quote <rules-file> <order-file>\n"},
	} {
		var out, errs bytes.Buffer
		if code := run(tc.args, &out, &errs); code != tc.code || out.String() != tc.stdout || errs.String() != tc.errs {
			t.Errorf("run(%v) = %d, %q, %q", tc.args, code, out.String(), errs.String())
		}
	}
}

func TestReadErrors(t *testing.T) {
	if _, err := quote("missing.rules", testdata("small.order")); err == nil {
		t.Error("a missing rules file")
	}
	if _, err := quote(testdata("shop.rules"), "missing.order"); err == nil {
		t.Error("a missing order file")
	}
}

func TestOrderLines(t *testing.T) {
	for _, text := range []string{"apple 12", "apple 12 x", "apple 1234567890 5", "apple -1 5"} {
		if _, err := parseOrder(text); err == nil {
			t.Errorf("parseOrder(%q) accepted", text)
		}
	}
}

func TestMainExits(t *testing.T) {
	code := -1
	exit = func(c int) { code = c }
	main() // the test binary's own arguments: a usage error
	if code != 2 {
		t.Errorf("exit %d", code)
	}
}
