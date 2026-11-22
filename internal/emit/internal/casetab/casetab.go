// Package casetab renders Go's case tables (package unicode) as Rust.
package casetab

import (
	"fmt"
	"strings"
	"unicode"
)

// Version is the Unicode version of the running toolchain's tables.
func Version() string { return unicode.Version }

// Rust returns the Rust constants CASE (unicode.CaseRanges: lo, hi, upper
// delta, lower delta, with UL for Go's UpperLower) and FOLD (the runes
// whose unicode.SimpleFold is not ToLower, or ToUpper when ToLower is the
// rune itself: Go's case orbits).
func Rust() string {
	var b strings.Builder
	fmt.Fprintf(&b, "/// Go's unicode.CaseRanges (Unicode %s): lo, hi, upper and lower\n", unicode.Version)
	b.WriteString("/// deltas; UL marks Go's UpperLower, alternating upper and lower pairs.\n")
	b.WriteString("const CASE: &[(u32, u32, i32, i32)] = &[\n")
	for _, r := range unicode.CaseRanges {
		fmt.Fprintf(&b, "    (0x%X, 0x%X, %s, %s),\n", r.Lo, r.Hi, delta(r.Delta[unicode.UpperCase]), delta(r.Delta[unicode.LowerCase]))
	}
	b.WriteString("];\n\n")
	fmt.Fprintf(&b, "/// Go's case orbits (Unicode %s): the runes whose unicode.SimpleFold\n", unicode.Version)
	b.WriteString("/// is neither its ToLower nor, when that is itself, its ToUpper.\n")
	b.WriteString("const FOLD: &[(u32, u32)] = &[\n")
	for r := rune(0); r <= unicode.MaxRune; r++ {
		f := unicode.ToLower(r)
		if f == r {
			f = unicode.ToUpper(r)
		}
		if s := unicode.SimpleFold(r); s != f {
			fmt.Fprintf(&b, "    (0x%X, 0x%X),\n", r, s)
		}
	}
	b.WriteString("];\n")
	return b.String()
}

func delta(d rune) string {
	if d == unicode.MaxRune+1 {
		return "UL"
	}
	return fmt.Sprint(d)
}
