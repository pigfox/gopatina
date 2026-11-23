// Command quote prices an order under a rules file with the Go package
// go/rules: the Go original of the Rust application app/, which uses the
// crate GoPatina generates from the same package. The two print the same
// receipts and errors (interop_test.go).
//
// usage: quote <rules-file> <order-file>
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pigfox/gopatina/examples/rust-interop/go/rules"
)

var exit = os.Exit

func main() { exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: quote <rules-file> <order-file>")
		return 2
	}
	out, err := quote(args[0], args[1])
	if err != nil {
		fmt.Fprintln(stderr, "quote:", err)
		return 1
	}
	fmt.Fprint(stdout, out)
	return 0
}

func quote(rulesPath, orderPath string) (string, error) {
	rulesText, err := os.ReadFile(rulesPath)
	if err != nil {
		return "", err
	}
	orderText, err := os.ReadFile(orderPath)
	if err != nil {
		return "", err
	}
	rs, err := rules.Parse(string(rulesText))
	if err != nil {
		return "", err
	}
	order, err := parseOrder(string(orderText))
	if err != nil {
		return "", err
	}
	q, err := rules.Price(order, rs)
	if err != nil {
		return "", err
	}
	return receipt(q), nil
}

// parseOrder reads one <sku> <quantity> <cents> line per item, with blank
// lines and # comments ignored.
func parseOrder(text string) ([]rules.Line, error) {
	var lines []rules.Line
	for i, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		bad := fmt.Errorf("order line %d: expected <sku> <quantity> <cents>", i+1)
		if len(f) != 3 {
			return nil, bad
		}
		q, ok1 := number(f[1])
		c, ok2 := number(f[2])
		if !ok1 || !ok2 {
			return nil, bad
		}
		lines = append(lines, rules.Line{SKU: f[0], Quantity: q, Cents: c})
	}
	return lines, nil
}

// number reads a whole number of one to nine digits.
func number(s string) (int, bool) {
	if s == "" || len(s) > 9 {
		return 0, false
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

func money(cents int) string { return fmt.Sprintf("%d.%02d", cents/100, cents%100) }

func receipt(q rules.Quote) string {
	var b strings.Builder
	for _, row := range []struct {
		label string
		cents int
	}{{"subtotal", q.Subtotal}, {"discount", -q.Discount}, {"shipping", q.Shipping}, {"tax", q.Tax}, {"total", q.Total}} {
		amount := money(row.cents)
		if row.cents < 0 {
			amount = "-" + money(-row.cents)
		}
		fmt.Fprintf(&b, "%-10s%10s\n", row.label, amount)
	}
	applied := "none"
	if len(q.Applied) > 0 {
		applied = strings.Join(q.Applied, "; ")
	}
	fmt.Fprintf(&b, "applied: %s\n", applied)
	return b.String()
}
