// Command fmt_q prints strings with the verb %q, which Go's fmt prints as
// strconv.Quote does: in double quotes, escaped where a rune is not
// printable by Go's own Unicode tables, and padded to a width by runes.
package main

import "fmt"

// level prints through its String method, which %q quotes.
type level int

func (l level) String() string {
	if l > 2 {
		return "high\t!"
	}
	return "low"
}

const greeting = "héllo, 世界"

func main() {
	var i int
	var f float64
	var b bool
	var s string
	fmt.Printf("%v %v %v %q\n", i, f, b, s)

	for _, w := range []string{
		"plain", `quote " backslash \`, "tab\tnewline\ncr\rbell\a",
		"bs\b ff\f vt\v nul\x00 esc\x1b del\x7f",
		"latin \u00a0 nbsp \u00ad soft hyphen é",
		"zero width \u200b joiner \u200d bom \ufeff",
		"emoji \U0001F600 tag \U000E0001 private \ue000",
		"日本語", "",
	} {
		fmt.Printf("%q\n", w)
	}

	fmt.Printf("[%12q] [%-12q] [%3q]\n", "日本", "日本", "longer")
	fmt.Printf("%q %q\n", level(1), level(3))
	q := fmt.Sprintf("%q", greeting)
	fmt.Println(len(q), q)
	fmt.Printf("%q|%q\n", greeting, "const")
}
