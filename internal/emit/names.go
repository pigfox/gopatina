package emit

import (
	"strings"
	"unicode"
)

// Rust keywords, which need a raw identifier (r#type) or, for the few that
// cannot be raw, a trailing underscore.
var rustKeywords = map[string]bool{
	"as": true, "async": true, "await": true, "box": true, "break": true, "const": true,
	"continue": true, "crate": true, "dyn": true, "else": true, "enum": true, "extern": true,
	"false": true, "fn": true, "for": true, "gen": true, "if": true, "impl": true, "in": true,
	"let": true, "loop": true, "match": true, "mod": true, "move": true, "mut": true,
	"pub": true, "ref": true, "return": true, "self": true, "Self": true, "static": true,
	"struct": true, "super": true, "trait": true, "true": true, "type": true, "unsafe": true,
	"use": true, "where": true, "while": true, "abstract": true, "become": true, "do": true,
	"final": true, "macro": true, "override": true, "priv": true, "typeof": true,
	"unsized": true, "virtual": true, "yield": true, "try": true,
}

// Names of types in Rust's prelude and in the generated code, which a Go
// type must not take.
var reservedTypes = map[string]bool{
	"String": true, "Vec": true, "Option": true, "Some": true, "None": true, "Result": true,
	"Ok": true, "Err": true, "Box": true, "Default": true, "Clone": true, "Copy": true,
	"Send": true, "Sync": true, "Sized": true, "Drop": true, "Fn": true, "FnMut": true,
	"FnOnce": true, "Iterator": true, "IntoIterator": true, "Extend": true, "PartialEq": true,
	"PartialOrd": true, "Eq": true, "Ord": true, "ToString": true, "ToOwned": true,
	"AsRef": true, "AsMut": true, "Into": true, "From": true, "Debug": true, "Self": true,
}

// ident makes name safe to use as a Rust identifier.
func ident(name string) string {
	switch name {
	case "self", "Self", "super", "crate":
		return name + "_"
	}
	if rustKeywords[name] {
		return "r#" + name
	}
	return name
}

// snake converts a Go name to Rust's snake_case: ScaleBy and scaleBy become
// scale_by, HTTPServer becomes http_server.
func snake(name string) string {
	rs := []rune(name)
	var b strings.Builder
	for i, r := range rs {
		if unicode.IsUpper(r) {
			prevLower := i > 0 && (unicode.IsLower(rs[i-1]) || unicode.IsDigit(rs[i-1]))
			nextLower := i > 0 && i+1 < len(rs) && unicode.IsLower(rs[i+1]) && unicode.IsUpper(rs[i-1])
			if prevLower || nextLower {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// screaming converts a Go constant name to Rust's SCREAMING_SNAKE_CASE.
func screaming(name string) string { return strings.ToUpper(snake(name)) }

// camel converts a Go type name to Rust's CamelCase: point becomes Point.
func camel(name string) string {
	rs := []rune(name)
	rs[0] = unicode.ToUpper(rs[0])
	var b strings.Builder
	up := false
	for _, r := range rs {
		if r == '_' {
			up = true
			continue
		}
		if up {
			r = unicode.ToUpper(r)
			up = false
		}
		b.WriteRune(r)
	}
	if b.Len() == 0 {
		return "T"
	}
	return b.String()
}

// A namer hands out distinct Rust names within one namespace.
type namer struct {
	taken map[string]string // Rust name -> the Go name that has it
	sep   string            // between a name and its number: "_", or "" for types
}

func newNamer(reserved ...string) *namer {
	n := &namer{taken: map[string]string{}, sep: "_"}
	for _, r := range reserved {
		n.taken[r] = ""
	}
	return n
}

// name returns a Rust name for goName, converted by conv, distinct from
// the names already given to other Go names.
func (n *namer) name(goName string, conv func(string) string) string {
	base := ident(conv(goName))
	for i := 1; ; i++ {
		cand := base
		if i > 1 {
			cand = strings.TrimPrefix(base, "r#") + n.sep + itoa(i)
		}
		owner, ok := n.taken[cand]
		if !ok {
			n.taken[cand] = goName
			return cand
		}
		if owner == goName && goName != "" {
			return cand
		}
	}
}

func itoa(i int) string {
	const digits = "0123456789"
	if i < 10 {
		return digits[i : i+1]
	}
	return itoa(i/10) + digits[i%10:i%10+1]
}
