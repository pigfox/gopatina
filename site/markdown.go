package main

import (
	"html"
	"regexp"
	"strings"
)

// A small Markdown renderer for the site's own pages, so that each page is
// written once, in content/pages, and served both as HTML and as Markdown.
// It covers what the pages use and nothing more:
//
//   - one "# " title, "## " and "### " headings, with an optional {#id};
//   - paragraphs, with an optional {.class} at the end;
//   - "- " and "1. " lists, "| " tables and ``` code blocks;
//   - inline `code`, **strong**, *emphasis*, [links](url), and the HTML
//     tags <sub> and <sup>, kept as written; everything else is escaped.

// doc is a rendered page.
type doc struct {
	Title    string    // the text of the "# " heading
	HTML     string    // the body, without the title
	Sections []section // each "### " heading with the plain text after it
	Steps    []string  // the plain text of each item of the first "1. " list
}

// section is a "### " heading and the plain text of the paragraphs after
// it, up to the next heading: a question and its answer on the FAQ page.
type section struct {
	Heading, Text string
}

var (
	headingRE = regexp.MustCompile(`^(#{1,3}) (.*?)(?: \{#([a-z0-9-]+)\})?$`)
	classRE   = regexp.MustCompile(` \{\.([a-z-]+)\}$`)
	orderedRE = regexp.MustCompile(`^\d+\. `)
	linkRE    = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	codeRE    = regexp.MustCompile("`([^`]+)`")
	strongRE  = regexp.MustCompile(`\*\*(.+?)\*\*`)
	emRE      = regexp.MustCompile(`(^|[^\w*])\*([^*\s][^*]*?)\*([^\w*]|$)`)
	tagRE     = regexp.MustCompile(`&lt;(/?)(sub|sup)&gt;`)
	slugRE    = regexp.MustCompile(`[^a-z0-9]+`)
)

// render turns the Markdown source of a page into a doc.
func render(src string) doc {
	var d doc
	var b strings.Builder
	lines := strings.Split(strings.TrimRight(src, "\n"), "\n")
	var cur *section
	for i := 0; i < len(lines); {
		l := lines[i]
		switch {
		case strings.TrimSpace(l) == "":
			i++
		case headingRE.MatchString(l):
			m := headingRE.FindStringSubmatch(l)
			text := m[2]
			if m[1] == "#" {
				d.Title = plain(text)
				cur = nil
				i++
				continue
			}
			id := m[3]
			if id == "" {
				id = slug(plain(text))
			}
			tag := "h2"
			cur = nil
			if m[1] == "###" {
				tag = "h3"
				d.Sections = append(d.Sections, section{Heading: plain(text)})
				cur = &d.Sections[len(d.Sections)-1]
			}
			b.WriteString("<" + tag + ` id="` + id + `">` + inline(text) + "</" + tag + ">\n")
			i++
		case strings.HasPrefix(l, "```"):
			j := i + 1
			for j < len(lines) && !strings.HasPrefix(lines[j], "```") {
				j++
			}
			b.WriteString("<pre><code>" + html.EscapeString(strings.Join(lines[i+1:j], "\n")) + "</code></pre>\n")
			i = j + 1
		case strings.HasPrefix(l, "| "):
			j := i
			for j < len(lines) && strings.HasPrefix(lines[j], "| ") {
				j++
			}
			b.WriteString(table(lines[i:j]))
			i = j
		case strings.HasPrefix(l, "- ") || orderedRE.MatchString(l):
			ordered := !strings.HasPrefix(l, "- ")
			tag := "ul"
			if ordered {
				tag = "ol"
			}
			b.WriteString("<" + tag + ">\n")
			first := ordered && d.Steps == nil
			for i < len(lines) && (strings.HasPrefix(lines[i], "- ") && !ordered || ordered && orderedRE.MatchString(lines[i])) {
				item := strings.TrimPrefix(lines[i], "- ")
				if ordered {
					item = orderedRE.ReplaceAllString(lines[i], "")
				}
				if first {
					d.Steps = append(d.Steps, plain(item))
				}
				b.WriteString("<li>" + inline(item) + "</li>\n")
				i++
			}
			b.WriteString("</" + tag + ">\n")
		default:
			var para []string
			for i < len(lines) && strings.TrimSpace(lines[i]) != "" && !block(lines[i]) {
				para = append(para, lines[i])
				i++
			}
			text := strings.Join(para, " ")
			open := "<p>"
			if m := classRE.FindStringSubmatch(text); m != nil {
				text = strings.TrimSuffix(text, m[0])
				open = `<p class="` + m[1] + `">`
			}
			if cur != nil {
				cur.Text = strings.TrimSpace(cur.Text + " " + plain(text))
			}
			b.WriteString(open + inline(text) + "</p>\n")
		}
	}
	d.HTML = b.String()
	return d
}

// block reports whether l starts a block other than a paragraph.
func block(l string) bool {
	return headingRE.MatchString(l) || strings.HasPrefix(l, "```") || strings.HasPrefix(l, "| ") ||
		strings.HasPrefix(l, "- ") || orderedRE.MatchString(l)
}

// table renders a Markdown table as a table that stacks into cards on a
// phone: each cell carries its column name.
func table(rows []string) string {
	cells := func(r string) []string {
		parts := strings.Split(strings.Trim(strings.TrimSpace(r), "|"), "|")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}
	head := cells(rows[0])
	var b strings.Builder
	b.WriteString(`<div class="table-wrap"><table class="stack">` + "\n<thead><tr>")
	for _, h := range head {
		b.WriteString(`<th scope="col">` + inline(h) + "</th>")
	}
	b.WriteString("</tr></thead>\n<tbody>\n")
	for _, r := range rows[2:] {
		b.WriteString("<tr>")
		for i, c := range cells(r) {
			b.WriteString(`<td data-label="` + html.EscapeString(plain(head[i])) + `">` + inline(c) + "</td>")
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</tbody>\n</table></div>\n")
	return b.String()
}

// inline renders the inline Markdown of one line of text.
func inline(s string) string {
	// **strong** may span a link, so it is marked first, with two control
	// characters that escaping leaves alone. Then links, since their text
	// may hold code; then code spans, whose contents are kept literally.
	s = strongRE.ReplaceAllString(s, "\x01$1\x02")
	var out strings.Builder
	for {
		m := linkRE.FindStringSubmatchIndex(s)
		if m == nil {
			out.WriteString(spans(s))
			return strings.NewReplacer("\x01", "<strong>", "\x02", "</strong>").Replace(out.String())
		}
		out.WriteString(spans(s[:m[0]]))
		out.WriteString(`<a href="` + html.EscapeString(s[m[4]:m[5]]) + `">` + spans(s[m[2]:m[3]]) + "</a>")
		s = s[m[1]:]
	}
}

// spans renders code spans and emphasis in text without links.
func spans(s string) string {
	var out strings.Builder
	for {
		m := codeRE.FindStringSubmatchIndex(s)
		if m == nil {
			out.WriteString(marks(s))
			return out.String()
		}
		out.WriteString(marks(s[:m[0]]))
		out.WriteString("<code>" + html.EscapeString(s[m[2]:m[3]]) + "</code>")
		s = s[m[1]:]
	}
}

// marks escapes text and renders *emphasis*, <sub> and <sup>.
func marks(s string) string {
	s = html.EscapeString(s)
	s = tagRE.ReplaceAllString(s, "<$1$2>")
	return emRE.ReplaceAllString(s, "$1<em>$2</em>$3")
}

// plain returns the text of inline Markdown, without its marks: for titles,
// structured data and ids.
func plain(s string) string {
	s = linkRE.ReplaceAllString(s, "$1")
	s = strings.NewReplacer("**", "", "`", "", "<sub>", "", "</sub>", "", "<sup>", "^", "</sup>", "").Replace(s)
	return emRE.ReplaceAllString(s, "$1$2$3")
}

// slug makes a heading's id: lower case, words joined by hyphens.
func slug(s string) string {
	return strings.Trim(slugRE.ReplaceAllString(strings.ToLower(s), "-"), "-")
}
