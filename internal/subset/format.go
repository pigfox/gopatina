package subset

import (
	"fmt"
	"strconv"
)

// A Verb is one formatting directive of a fmt format string, or a run of
// literal text when Verb is 0.
type Verb struct {
	Text  string // literal text (Verb == 0)
	Spec  string // the directive as written, such as "%-8.2f"
	Verb  rune   // 'd', 's', 'v', ..., '%' for a literal percent sign
	Minus bool   // '-' flag: pad on the right
	Plus  bool   // '+' flag: always print a sign
	Zero  bool   // '0' flag: pad with leading zeros
	Space bool   // ' ' flag
	Sharp bool   // '#' flag
	Width int    // -1 if none
	Prec  int    // -1 if none
}

// ParseFormat splits a fmt format string into literal text and verbs. It
// accepts the directives patina mode supports: flags, a decimal width and
// precision, and a verb letter. Argument indexes and '*' are rejected.
func ParseFormat(format string) ([]Verb, error) {
	var out []Verb
	text := ""
	for i := 0; i < len(format); {
		if format[i] != '%' {
			text += format[i : i+1]
			i++
			continue
		}
		if text != "" {
			out = append(out, Verb{Text: text, Width: -1, Prec: -1})
			text = ""
		}
		start := i
		i++
		v := Verb{Width: -1, Prec: -1}
	flags:
		for ; i < len(format); i++ {
			switch format[i] {
			case '-':
				v.Minus = true
			case '+':
				v.Plus = true
			case '0':
				v.Zero = true
			case ' ':
				v.Space = true
			case '#':
				v.Sharp = true
			default:
				break flags
			}
		}
		v.Width, i = number(format, i)
		if i < len(format) && format[i] == '.' {
			i++
			v.Prec, i = number(format, i)
			if v.Prec < 0 {
				v.Prec = 0
			}
		}
		if i >= len(format) {
			return nil, fmt.Errorf("%q ends in the middle of a directive", format)
		}
		switch c := format[i]; {
		case c == '*' || c == '[':
			return nil, fmt.Errorf("%q: '%c' in a directive is not supported in patina mode yet", format, c)
		case c >= 0x80:
			return nil, fmt.Errorf("%q: unknown verb in %q", format, format[start:i+1])
		}
		v.Verb = rune(format[i])
		i++
		v.Spec = format[start:i]
		switch v.Verb {
		case 'v', 'd', 's', 't', 'f', 'F', 'x', 'X', 'c', 'T':
		case '%':
			if v.Spec != "%%" {
				return nil, fmt.Errorf("%q: flags on %%%% are not supported", format)
			}
			v = Verb{Text: "%", Width: -1, Prec: -1}
		default:
			return nil, fmt.Errorf("%q: the verb %s is not supported in patina mode yet", format, v.Spec)
		}
		out = append(out, v)
	}
	if text != "" {
		out = append(out, Verb{Text: text, Width: -1, Prec: -1})
	}
	return mergeText(out), nil
}

// mergeText joins adjacent runs of literal text.
func mergeText(vs []Verb) []Verb {
	var out []Verb
	for _, v := range vs {
		if v.Verb == 0 && len(out) > 0 && out[len(out)-1].Verb == 0 {
			out[len(out)-1].Text += v.Text
			continue
		}
		out = append(out, v)
	}
	return out
}

// number parses a decimal number at s[i:], returning -1 if there is none,
// and the index after it.
func number(s string, i int) (int, int) {
	j := i
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	if j == i {
		return -1, i
	}
	n, err := strconv.Atoi(s[i:j])
	if err != nil || n > 1000 {
		return 1000, j
	}
	return n, j
}
