// Command strings_provided exercises the first provided API of package
// strings (RFC 0011), including the corners where Go's behaviour differs
// from Rust's standard library: Unicode case mapping by Go's own tables
// (final sigma, dotted capital I, sharp s, Kelvin sign), simple case
// folding, and Split with an empty separator.
package main

import (
	"fmt"
	"strings"
)

func describe(s string) string {
	before, after, found := strings.Cut(s, "=")
	if !found {
		return "no key in " + s
	}
	return strings.ToUpper(strings.TrimSpace(before)) + " -> " + strings.TrimSpace(after)
}

func main() {
	for _, s := range []string{"Hello, Wörld", "ΟΔΟΣ", "İstanbul", "straße", "K", "ǅ", "ﬀ", ""} {
		fmt.Println(strings.ToLower(s), strings.ToUpper(s), len(strings.ToLower(s)))
	}
	fmt.Println(strings.EqualFold("Go", "GO"), strings.EqualFold("K", "k"), strings.EqualFold("ſ", "S"),
		strings.EqualFold("σ", "ς"), strings.EqualFold("ß", "SS"), strings.EqualFold("a", "ab"), strings.EqualFold("", ""))
	parts := strings.Split("a,b,,c", ",")
	fmt.Println(len(parts), parts, len(strings.Split("héllo", "")), strings.Split("", ","), len(strings.Split("", ",")))
	fmt.Println(strings.Split("abc", "abc"), len(strings.Split("abc", "abc")))
	fmt.Println(describe("name = gopher"), "|", describe("plain"), "|", describe("=x="))
	path := "srv/data/logs/"
	fmt.Println(strings.TrimPrefix(path, "srv"), strings.TrimSuffix(path, "/"), strings.TrimPrefix(path, "x"), strings.TrimSuffix("aaa", "a"))
	fmt.Println(strings.ReplaceAll("banana", "a", "o"), strings.ReplaceAll("abc", "", "-"), strings.ReplaceAll("", "", "x"), strings.ReplaceAll("aaaa", "aa", "b"))
	fmt.Println(strings.IndexByte("chicken", 'k'), strings.IndexByte("chicken", 'z'), strings.IndexByte("héllo", 'l'))
	b, a, ok := strings.Cut("key", "")
	fmt.Println(b, a, ok)
}
