package main

import (
	"errors"
	"fmt"
	"html"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// The blog: progress posts on real milestones. Each post is
// content/blog/<yyyy-mm-dd>-<slug>.md, a few lines of front matter and
// Markdown, served at /blog/<yyyy-mm-dd>-<slug>; /blog lists them, newest
// first, and /blog/feed.xml is their Atom feed. The blog is in the
// navigation and the sitemap once it has a post.

// aiNote ends every post: how it was written, and where its figures are
// from.
const aiNote = "Drafted with AI assistance; every figure links to its evidence."

// blogPage is the blog's index.
var blogPage = page{
	Path: "/blog", Kind: "blog", Crumb: "Blog",
	Title:       "GoPatina blog: progress compiling Go through Rust",
	Description: "Progress on GoPatina, the Go toolchain front end that compiles Go through Rust: each milestone as it lands, with links to the evidence.",
}

// postName is a post's file name, without .md: its date and its slug.
var postName = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-[a-z0-9]+(?:-[a-z0-9]+)*$`)

// post is one blog post: its page and its Markdown, after the front
// matter.
type post struct {
	page
	src string
}

// loadPosts reads every post under content/blog, newest first. A post
// whose name, date, title or description breaks the site's rules is an
// error, so it never reaches the site.
func loadPosts(fsys fs.FS) ([]post, error) {
	entries, err := fs.ReadDir(fsys, "content/blog")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ps []post
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".md")
		if e.IsDir() || !ok {
			continue
		}
		src, err := fs.ReadFile(fsys, "content/blog/"+e.Name())
		if err != nil {
			return nil, err
		}
		p, err := parsePost(name, string(src))
		if err != nil {
			return nil, fmt.Errorf("content/blog/%s: %v", e.Name(), err)
		}
		ps = append(ps, p)
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].Path > ps[j].Path })
	return ps, nil
}

// parsePost reads the post named name from src: front matter between
// "---" lines (title, description, and updated if it changed after its
// date), then Markdown.
func parsePost(name, src string) (post, error) {
	m := postName.FindStringSubmatch(name)
	if m == nil {
		return post{}, errors.New("the file name must be <yyyy-mm-dd>-<slug>.md, the slug of lowercase letters, digits and hyphens")
	}
	if _, err := time.Parse(time.DateOnly, m[1]); err != nil {
		return post{}, fmt.Errorf("the date %s is not a date", m[1])
	}
	rest, ok := strings.CutPrefix(src, "---\n")
	head, body, ok2 := strings.Cut(rest, "\n---\n")
	if !ok || !ok2 {
		return post{}, errors.New("no front matter: start with ---, the fields, then ---")
	}
	fields := map[string]string{}
	for _, line := range strings.Split(head, "\n") {
		k, v, ok := strings.Cut(line, ": ")
		if !ok {
			return post{}, fmt.Errorf("front matter line %q is not key: value", line)
		}
		fields[k] = v
	}
	title, desc := fields["title"], fields["description"]
	updated := m[1]
	if u, ok := fields["updated"]; ok {
		if _, err := time.Parse(time.DateOnly, u); err != nil || u < m[1] {
			return post{}, fmt.Errorf("updated %q is not a date on or after %s", u, m[1])
		}
		updated = u
	}
	if n := utf8.RuneCountInString(title); n < 30 || n > 60 {
		return post{}, fmt.Errorf("the title has %d characters, not 30 to 60", n)
	}
	if n := utf8.RuneCountInString(desc); n < 70 || n > 160 {
		return post{}, fmt.Errorf("the description has %d characters, not 70 to 160", n)
	}
	return post{page: page{
		Path: "/blog/" + name, Kind: "post", Title: title, Description: desc, Crumb: title,
		Updated: updated, Published: m[1], ParentPath: blogPage.Path, ParentName: blogPage.Crumb,
	}, src: body}, nil
}

// longDate is a date as the posts show it: November 22, 2026.
func longDate(date string) string {
	t, _ := time.Parse(time.DateOnly, date) // checked by parsePost
	return t.Format("January 2, 2006")
}

// blogIndex returns the Markdown of the blog's index for posts.
func blogIndex(posts []post) string {
	var b strings.Builder
	b.WriteString("# The GoPatina blog\n\n")
	b.WriteString("Progress on GoPatina as it happens: each feature as it is merged, each result as it is measured, and each defect found, ours or the Go compiler's. Every figure links to its evidence. Follow it with the [Atom feed](/blog/feed.xml).\n\n")
	if len(posts) == 0 {
		b.WriteString("No posts yet.\n")
	}
	for _, p := range posts {
		fmt.Fprintf(&b, "## [%s](%s)\n\n%s. %s\n\n", p.Title, p.Path, longDate(p.Published), p.Description)
	}
	return b.String()
}

// postHTML wraps a post's rendered body with its byline and the note on
// how it was written.
func postHTML(p page, body string) string {
	byline := fmt.Sprintf("<p class=\"meta\">By Peter Sjolin, <time datetime=\"%s\">%s</time>.</p>\n", p.Published, longDate(p.Published))
	return byline + body + "<p class=\"meta\">" + aiNote + "</p>\n"
}

// atomFeed returns the blog's Atom feed: every post, with its rendered
// body.
func atomFeed(posts []post, bodies map[string]string) string {
	updated := lastUpdated
	for _, p := range posts {
		updated = max(updated, p.Updated)
	}
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<feed xmlns=\"http://www.w3.org/2005/Atom\">\n")
	b.WriteString("  <title>GoPatina blog</title>\n")
	b.WriteString("  <subtitle>" + html.EscapeString(blogPage.Description) + "</subtitle>\n")
	b.WriteString("  <link rel=\"self\" type=\"application/atom+xml\" href=\"" + siteURL + "blog/feed.xml\"/>\n")
	b.WriteString("  <link rel=\"alternate\" type=\"text/html\" href=\"" + blogPage.url() + "\"/>\n")
	b.WriteString("  <id>" + blogPage.url() + "</id>\n")
	b.WriteString("  <updated>" + updated + "T00:00:00Z</updated>\n")
	b.WriteString("  <author><name>Peter Sjolin</name></author>\n")
	for _, p := range posts {
		b.WriteString("  <entry>\n")
		b.WriteString("    <title>" + html.EscapeString(p.Title) + "</title>\n")
		b.WriteString("    <link rel=\"alternate\" type=\"text/html\" href=\"" + p.url() + "\"/>\n")
		b.WriteString("    <id>" + p.url() + "</id>\n")
		b.WriteString("    <published>" + p.Published + "T00:00:00Z</published>\n")
		b.WriteString("    <updated>" + p.Updated + "T00:00:00Z</updated>\n")
		b.WriteString("    <summary>" + html.EscapeString(p.Description) + "</summary>\n")
		b.WriteString("    <content type=\"html\">" + html.EscapeString(bodies[p.Path]) + "</content>\n")
		b.WriteString("  </entry>\n")
	}
	b.WriteString("</feed>\n")
	return b.String()
}
