package main

import (
	"encoding/json"
	"encoding/xml"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
)

// withPosts is the site's files, with the given files under content/blog
// in place of the site's own posts.
func withPosts(t *testing.T, posts map[string]string) fstest.MapFS {
	t.Helper()
	m := fstest.MapFS{}
	fs.WalkDir(files, ".", func(name string, d fs.DirEntry, _ error) error {
		if !d.IsDir() && !strings.HasPrefix(name, "content/blog/") {
			b, _ := files.ReadFile(name)
			m[name] = &fstest.MapFile{Data: b}
		}
		return nil
	})
	for name, src := range posts {
		m["content/blog/"+name] = &fstest.MapFile{Data: []byte(src)}
	}
	return m
}

const (
	firstPost = "---\ntitle: The first post of the GoPatina blog, a test\ndescription: A post for the tests: it has a title, a description, a date and a body with a link and a figure.\n---\n# The first post\n\nIt links to [the evidence](/evidence) & says 3 < 4.\n"
	laterPost = "---\ntitle: A later post of the GoPatina blog, for the tests\ndescription: A later post for the tests, changed after its date, so its updated date is the feed's last change.\nupdated: 2026-11-23\n---\n# A later post\n\nMore.\n"
)

func TestBlogWithPosts(t *testing.T) {
	h, err := newHandler(withPosts(t, map[string]string{
		"2026-11-16-first-post.md": firstPost,
		"2026-11-22-later-post.md": laterPost,
		"README.txt":               "not a post",
		"drafts/x.md":              "a directory is not a post",
	}))
	if err != nil {
		t.Fatal(err)
	}
	index := body(t, h, "/blog")
	if strings.Contains(index, `name="robots"`) {
		t.Error("the blog with posts is noindex")
	}
	i, j := strings.Index(index, "/blog/2026-11-22-later-post"), strings.Index(index, "/blog/2026-11-16-first-post")
	if i < 0 || j < 0 || i > j {
		t.Errorf("the index does not list the posts newest first")
	}
	if !strings.Contains(body(t, h, "/"), `<a href="/blog">Blog</a>`) || !strings.Contains(index, "November 16, 2026.") {
		t.Error("the blog is not in the navigation, or a post's date is missing")
	}
	sitemap := body(t, h, "/sitemap.xml")
	for _, u := range []string{"blog</loc><lastmod>2026-11-23", "blog/2026-11-16-first-post</loc><lastmod>2026-11-16", "blog/2026-11-22-later-post</loc><lastmod>2026-11-23"} {
		if !strings.Contains(sitemap, siteURL+u) {
			t.Errorf("sitemap lacks %s", u)
		}
	}
	if !strings.Contains(body(t, h, "/llms.txt"), "/blog/2026-11-16-first-post.md") {
		t.Error("llms.txt lacks the post")
	}

	post := body(t, h, "/blog/2026-11-16-first-post")
	for _, want := range []string{
		`<a href="/blog">Blog</a> <span aria-hidden="true">›</span> <span aria-current="page">The first post of the GoPatina blog, a test</span>`,
		`By Peter Sjolin, <time datetime="2026-11-16">November 16, 2026</time>.`,
		aiNote,
		`<link rel="alternate" type="application/atom+xml" title="GoPatina blog" href="/blog/feed.xml">`,
	} {
		if !strings.Contains(post, want) {
			t.Errorf("the post lacks %q", want)
		}
	}
	ld := jsonLDOf(t, post)
	var posting, crumbs map[string]any
	for _, n := range ld["@graph"].([]any) {
		m := n.(map[string]any)
		switch m["@type"] {
		case "BlogPosting":
			posting = m
		case "BreadcrumbList":
			crumbs = m
		}
	}
	if posting == nil || posting["datePublished"] != "2026-11-16" || posting["headline"] != "The first post" {
		t.Errorf("BlogPosting = %v", posting)
	}
	if crumbs == nil || len(crumbs["itemListElement"].([]any)) != 3 {
		t.Errorf("BreadcrumbList = %v", crumbs)
	}
	if !strings.Contains(body(t, h, "/blog/2026-11-16-first-post.md"), "It links to [the evidence]") {
		t.Error("the post's Markdown is not served")
	}
	var blogLD map[string]any
	for _, n := range jsonLDOf(t, index)["@graph"].([]any) {
		if m := n.(map[string]any); m["@type"] == "Blog" {
			blogLD = m
		}
	}
	if blogLD == nil || len(blogLD["blogPost"].([]any)) != 2 {
		t.Errorf("Blog = %v", blogLD)
	}

	rec := get(t, h, http.MethodGet, "/blog/feed.xml", false)
	if ct := rec.Header().Get("Content-Type"); ct != "application/atom+xml; charset=utf-8" {
		t.Errorf("feed Content-Type %q", ct)
	}
	var feed struct {
		Updated string `xml:"updated"`
		Entries []struct {
			Title     string `xml:"title"`
			ID        string `xml:"id"`
			Published string `xml:"published"`
			Updated   string `xml:"updated"`
			Content   string `xml:"content"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal(rec.Body.Bytes(), &feed); err != nil {
		t.Fatalf("the feed is not XML: %v", err)
	}
	if feed.Updated != "2026-11-23T00:00:00Z" || len(feed.Entries) != 2 || feed.Entries[0].Updated != "2026-11-23T00:00:00Z" ||
		!strings.Contains(feed.Entries[1].Content, "3 &lt; 4") || !strings.Contains(feed.Entries[1].Content, aiNote) {
		t.Errorf("feed = %+v", feed)
	}
}

func TestBlogWithoutPosts(t *testing.T) {
	h, err := newHandler(withPosts(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	index := body(t, h, "/blog")
	if !strings.Contains(index, "No posts yet.") || !strings.Contains(index, `name="robots" content="noindex"`) {
		t.Error("the empty blog is not noindex, or does not say it has no posts")
	}
	if strings.Contains(body(t, h, "/"), `<a href="/blog">`) || strings.Contains(body(t, h, "/sitemap.xml"), "/blog") {
		t.Error("the empty blog is in the navigation or the sitemap")
	}
	var feed struct {
		Entries []struct{} `xml:"entry"`
	}
	if err := xml.Unmarshal([]byte(body(t, h, "/blog/feed.xml")), &feed); err != nil || len(feed.Entries) != 0 {
		t.Errorf("empty feed: %v, %d entries", err, len(feed.Entries))
	}
}

func TestBlogPostErrors(t *testing.T) {
	front := func(title, desc, extra string) string {
		return "---\ntitle: " + title + "\ndescription: " + desc + extra + "\n---\n# T\n"
	}
	title := "A title of the right length for a post"
	desc := "A description of the right length for a post of the blog, between seventy and 160."
	for name, c := range map[string]struct{ file, src, want string }{
		"bad name":    {"2026-11-16-Bad_Name.md", front(title, desc, ""), "the file name must be"},
		"bad date":    {"2026-13-40-x.md", front(title, desc, ""), "is not a date"},
		"no front":    {"2026-11-16-x.md", "# T\n", "no front matter"},
		"bad line":    {"2026-11-16-x.md", "---\ntitle\n---\n# T\n", "is not key: value"},
		"short title": {"2026-11-16-x.md", front("Too short", desc, ""), "not 30 to 60"},
		"short desc":  {"2026-11-16-x.md", front(title, "Too short", ""), "not 70 to 160"},
		"bad updated": {"2026-11-16-x.md", front(title, desc, "\nupdated: 2026-11-15"), "on or after"},
	} {
		_, err := newHandler(withPosts(t, map[string]string{c.file: c.src}))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", name, err, c.want)
		}
	}
	notDir := withPosts(t, nil)
	notDir["content/blog"] = &fstest.MapFile{Data: []byte("a file, not a directory")}
	if _, err := newHandler(notDir); err == nil {
		t.Error("content/blog as a file: no error")
	}
	unreadable := withPosts(t, map[string]string{"2026-11-16-x.md": front(title, desc, "")})
	unreadable["content/blog/2026-11-16-x.md"].Mode = fs.ModeSymlink // fstest cannot open it as a file
	if _, err := loadPosts(errFS{unreadable}); err == nil {
		t.Error("an unreadable post: no error")
	}
}

// errFS fails every ReadFile of a post, and serves everything else.
type errFS struct{ fstest.MapFS }

func (e errFS) Open(name string) (fs.File, error) {
	if strings.HasPrefix(name, "content/blog/") && strings.HasSuffix(name, ".md") {
		return nil, fs.ErrPermission
	}
	return e.MapFS.Open(name)
}

var ldRE = regexp.MustCompile(`(?s)<script type="application/ld\+json">\s*(.*?)\s*</script>`)

// jsonLDOf parses a page's JSON-LD.
func jsonLDOf(t *testing.T, page string) map[string]any {
	t.Helper()
	m := ldRE.FindStringSubmatch(page)
	var ld map[string]any
	if m == nil || json.Unmarshal([]byte(m[1]), &ld) != nil {
		t.Fatalf("no JSON-LD")
	}
	return ld
}
