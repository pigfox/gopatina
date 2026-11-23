// Package wordstat counts words: a library crate with no errors in its API
// (RFC 0014).
package wordstat

import "strings"

// MaxWords is the most words Top returns.
const MaxWords = 3

// Count is one word and how often it occurs.
type Count struct {
	Word string
	N    int
}

// Words splits text on spaces, lower-cased, dropping empty words.
func Words(text string) []string {
	var out []string
	for _, w := range strings.Split(strings.ToLower(text), " ") {
		if w != "" {
			out = append(out, w)
		}
	}
	return out
}

// Top returns the most frequent words, most frequent first, at most
// MaxWords, ties in order of first appearance.
func Top(words []string) []Count {
	var counts []Count
	for _, w := range words {
		found := false
		for i := range counts {
			if counts[i].Word == w {
				counts[i].N++
				found = true
			}
		}
		if !found {
			counts = append(counts, Count{Word: w, N: 1})
		}
	}
	for i := 1; i < len(counts); i++ {
		for j := i; j > 0 && counts[j].N > counts[j-1].N; j-- {
			counts[j], counts[j-1] = counts[j-1], counts[j]
		}
	}
	var top []Count
	for i := 0; i < len(counts) && i < MaxWords; i++ {
		top = append(top, counts[i])
	}
	return top
}
