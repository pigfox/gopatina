package main

import (
	"fmt"
	"strings"
)

// strings.TrimSpace must use Go's unicode.White_Space, not Rust's
// is_whitespace, so the two binaries agree on every boundary below.
func main() {
	cases := []string{
		"\u1680go\u1680", // Ogham space mark
		"\u2000go\u200a", // en quad .. hair space
		"\u2028go\u2029", // line/paragraph separator
		"\u202fgo\u205f", // narrow no-break / medium mathematical space
		"\u3000go\u0085", // ideographic space / NEL
		"\u00a0go\u00a0", // no-break space
		"\u180ego\u180e", // Mongolian vowel separator: not White_Space
		"\ufeffgo\ufeff", // BOM: not White_Space
		" \u3000go\t\n",  // mixed
		"\u200bgo\u200b", // zero-width space: not White_Space
	}
	for _, s := range cases {
		fmt.Println(strings.TrimSpace(s))
	}
}
