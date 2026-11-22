package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"html"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
)

func handler(t *testing.T) http.Handler {
	t.Helper()
	h, err := newHandler(files)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func get(t *testing.T, h http.Handler, method, path string, gz bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if gz {
		req.Header.Set("Accept-Encoding", "gzip, br")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// body returns the page at path, which must be served with status 200.
func body(t *testing.T, h http.Handler, path string) string {
	t.Helper()
	rec := get(t, h, http.MethodGet, path, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d", path, rec.Code)
	}
	return rec.Body.String()
}

func TestRoutes(t *testing.T) {
	h := handler(t)
	tests := []struct {
		path, ctype, cache, contains string
	}{
		{"/", "text/html; charset=utf-8", cacheShort, "<h1>GoPatina: write Go, run it like Rust</h1>"},
		{"/evidence", "text/html; charset=utf-8", cacheShort, "No official test-server run has happened yet."},
		{"/index.md", "text/markdown; charset=utf-8", cacheShort, "# GoPatina: write Go"},
		{"/faq.md", "text/markdown; charset=utf-8", cacheShort, "### What is GoPatina?"},
		{"/llms.txt", "text/plain; charset=utf-8", cacheShort, "- [FAQ](https://gopatina.org/faq): "},
		{"/robots.txt", "text/plain; charset=utf-8", cacheShort, "Sitemap: https://gopatina.org/sitemap.xml"},
		{"/sitemap.xml", "application/xml; charset=utf-8", cacheShort, "<loc>https://gopatina.org/faq</loc><lastmod>" + lastUpdated + "</lastmod>"},
		{"/healthz", "text/plain; charset=utf-8", "no-store", "ok"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := get(t, h, http.MethodGet, tt.path, false)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			if got := rec.Header().Get("Content-Type"); got != tt.ctype {
				t.Errorf("Content-Type = %q, want %q", got, tt.ctype)
			}
			if got := rec.Header().Get("Cache-Control"); got != tt.cache {
				t.Errorf("Cache-Control = %q, want %q", got, tt.cache)
			}
			if !strings.Contains(rec.Body.String(), tt.contains) {
				t.Errorf("body does not contain %q", tt.contains)
			}
			for k, v := range securityHeaders {
				if got := rec.Header().Get(k); got != v {
					t.Errorf("%s = %q, want %q", k, got, v)
				}
			}
		})
	}
	if strings.Contains(body(t, h, "/llms.txt"), "{{pages}}") {
		t.Error("llms.txt still has its {{pages}} placeholder")
	}
}

func TestStaticAssetsAreHashedAndImmutable(t *testing.T) {
	h := handler(t)
	home := body(t, h, "/")
	for _, re := range []string{`href="(/static/style\.[0-9a-f]{10}\.css)"`, `href="(/static/favicon\.[0-9a-f]{10}\.svg)"`, `src="(/static/anchors\.[0-9a-f]{10}\.js)"`} {
		m := regexp.MustCompile(re).FindStringSubmatch(home)
		if m == nil {
			t.Fatalf("home page has no link matching %s", re)
		}
		rec := get(t, h, http.MethodGet, m[1], false)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d", m[1], rec.Code)
		}
		if got := rec.Header().Get("Cache-Control"); got != cacheImmutable {
			t.Errorf("%s Cache-Control = %q", m[1], got)
		}
	}
}

// TestScripts checks the one script: on the home page only, from the
// site itself, vanilla JavaScript, mapping each old #anchor to a page that
// exists.
func TestScripts(t *testing.T) {
	h := handler(t)
	for _, p := range pages {
		b := body(t, h, p.Path)
		n := strings.Count(b, "<script src=")
		if want := map[bool]int{true: 1, false: 0}[p.Path == "/"]; n != want {
			t.Errorf("%s has %d scripts, want %d", p.Path, n, want)
		}
		if strings.Contains(b, "<script>") {
			t.Errorf("%s has an inline script", p.Path)
		}
	}
	js, err := files.ReadFile("static/anchors.js")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(bytes.ToLower(js), []byte("jquery")) || bytes.Contains(js, []byte("$(")) {
		t.Error("anchors.js must be vanilla JavaScript")
	}
	for _, m := range regexp.MustCompile(`"#[a-z]+": "(/[a-z-]*)`).FindAllStringSubmatch(string(js), -1) {
		if rec := get(t, h, http.MethodGet, m[1], false); rec.Code != http.StatusOK {
			t.Errorf("anchors.js sends a link to %s, which is %d", m[1], rec.Code)
		}
	}
}

func TestGzip(t *testing.T) {
	h := handler(t)
	rec := get(t, h, http.MethodGet, "/", true)
	if rec.Header().Get("Content-Encoding") != "gzip" || rec.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatalf("headers = %v", rec.Header())
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != body(t, h, "/") {
		t.Error("gzipped body differs from the plain body")
	}
}

func TestHeadHasNoBody(t *testing.T) {
	rec := get(t, handler(t), http.MethodHead, "/", false)
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Errorf("HEAD / = %d with %d bytes", rec.Code, rec.Body.Len())
	}
}

func TestNotFoundAndMethodNotAllowed(t *testing.T) {
	h := handler(t)
	for _, path := range []string{"/nope", "/nope/", "/evidence.html"} {
		rec := get(t, h, http.MethodGet, path, false)
		b := rec.Body.String()
		if rec.Code != http.StatusNotFound || !strings.Contains(b, "<h1>Page not found</h1>") ||
			!strings.Contains(b, `<meta name="robots" content="noindex">`) || rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("GET %s = %d, Cache-Control %q", path, rec.Code, rec.Header().Get("Cache-Control"))
		}
	}
	rec := get(t, h, http.MethodPost, "/", false)
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Errorf("POST / = %d, Allow %q", rec.Code, rec.Header().Get("Allow"))
	}
}

func TestRedirects(t *testing.T) {
	h := handler(t)
	for from, to := range map[string]string{"/faq/": "/faq", "/evidence//": "/evidence", "/index.html": "/", "//": "/"} {
		rec := get(t, h, http.MethodGet, from, false)
		if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != to {
			t.Errorf("GET %s = %d to %q, want 301 to %q", from, rec.Code, rec.Header().Get("Location"), to)
		}
	}
}

func TestCanonicalPath(t *testing.T) {
	for in, want := range map[string]string{"/": "/", "/faq": "/faq", "/faq/": "/faq", "///": "/", "/index.html": "/"} {
		if got := canonicalPath(in); got != want {
			t.Errorf("canonicalPath(%q) = %q, want %q", in, got, want)
		}
	}
}

var (
	titleRE     = regexp.MustCompile(`<title>(.*?)</title>`)
	descRE      = regexp.MustCompile(`<meta name="description" content="([^"]*)">`)
	canonicalRE = regexp.MustCompile(`<link rel="canonical" href="([^"]*)">`)
	jsonLDRE    = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
	hrefRE      = regexp.MustCompile(`href="(/[^"#]*)`)
)

type ldNode struct {
	Type         any    `json:"@type"`
	ID           string `json:"@id"`
	DateModified string `json:"dateModified"`
	MainEntity   []struct {
		Name   string `json:"name"`
		Answer struct {
			Text string `json:"text"`
		} `json:"acceptedAnswer"`
	} `json:"mainEntity"`
	Step []struct {
		Text string `json:"text"`
	} `json:"step"`
	Citation []any `json:"citation"`
	Items    []any `json:"itemListElement"`
}

// structured returns a page's JSON-LD nodes and the set of their types.
func structured(t *testing.T, b string) ([]ldNode, map[string]bool) {
	t.Helper()
	m := jsonLDRE.FindStringSubmatch(b)
	if m == nil {
		t.Fatal("no JSON-LD")
	}
	var doc struct {
		Graph []ldNode `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(m[1]), &doc); err != nil {
		t.Fatalf("JSON-LD does not parse: %v", err)
	}
	types := map[string]bool{}
	for _, n := range doc.Graph {
		switch v := n.Type.(type) {
		case string:
			types[v] = true
		case []any:
			for _, s := range v {
				types[s.(string)] = true
			}
		}
	}
	return doc.Graph, types
}

// TestEveryPageSEO checks each page's head, heading, structured data and
// links.
func TestEveryPageSEO(t *testing.T) {
	h := handler(t)
	titles, descriptions := map[string]string{}, map[string]string{}
	for _, p := range append(pages, notFoundPage) {
		t.Run(p.Path, func(t *testing.T) {
			var b string
			if p == notFoundPage {
				b = get(t, h, http.MethodGet, "/no-such-page", false).Body.String()
			} else {
				b = body(t, h, p.Path)
			}
			title := html.UnescapeString(titleRE.FindStringSubmatch(b)[1])
			if title != p.Title || len(title) < 30 || len(title) > 60 {
				t.Errorf("title %q (%d characters) must be the page's, 30 to 60 characters", title, len(title))
			}
			desc := html.UnescapeString(descRE.FindStringSubmatch(b)[1])
			if n := len([]rune(desc)); desc != p.Description || n < 70 || n > 160 {
				t.Errorf("description has %d characters, want 70 to 160: %q", n, desc)
			}
			if other, ok := titles[title]; ok {
				t.Errorf("title shared with %s", other)
			}
			if other, ok := descriptions[desc]; ok {
				t.Errorf("description shared with %s", other)
			}
			titles[title], descriptions[desc] = p.Path, p.Path
			if n := strings.Count(b, "<h1"); n != 1 {
				t.Errorf("%d h1 elements, want 1", n)
			}
			if got := canonicalRE.FindStringSubmatch(b)[1]; got != p.url() {
				t.Errorf("canonical %q, want %q", got, p.url())
			}
			for _, want := range []string{
				`<meta property="og:title" content="`, `<meta property="og:description" content="`,
				`<meta property="og:url" content="` + p.url() + `">`, `<meta name="twitter:card" content="summary">`,
				`<time datetime="` + p.Updated + `">`, "GoPatina is an experiment by", "built with gopatina's default mode",
			} {
				if !strings.Contains(b, want) {
					t.Errorf("page does not contain %q", want)
				}
			}
			if noindex := strings.Contains(b, `<meta name="robots" content="noindex">`); noindex != p.NoIndex {
				t.Errorf("noindex = %t, want %t", noindex, p.NoIndex)
			}
			// Markdown link syntax left in the HTML shows a raw URL, which
			// cannot wrap and widens the page on a phone.
			if strings.Contains(b, "](http") || strings.Contains(b, "](/") || strings.Contains(b, "**") {
				t.Error("page contains Markdown that was not rendered")
			}
			// Headings go down one level at a time: h1, then h2, then h3.
			last := 1
			for _, m := range regexp.MustCompile(`<h([1-6])`).FindAllStringSubmatch(b, -1) {
				level := int(m[1][0] - '0')
				if level > last+1 {
					t.Errorf("an h%d follows an h%d", level, last)
				}
				last = level
			}
			nodes, types := structured(t, b)
			for _, want := range []string{"Organization", "WebSite", "WebPage"} {
				if !types[want] {
					t.Errorf("JSON-LD has no %s", want)
				}
			}
			if types["BreadcrumbList"] != (p.Path != "/") {
				t.Errorf("BreadcrumbList = %t on %s", types["BreadcrumbList"], p.Path)
			}
			for _, n := range nodes {
				if n.DateModified != "" && n.DateModified != p.Updated {
					t.Errorf("dateModified = %q, want %q", n.DateModified, p.Updated)
				}
			}
			// Every link within the site leads to a page.
			for _, m := range hrefRE.FindAllStringSubmatch(b, -1) {
				if rec := get(t, h, http.MethodGet, m[1], false); rec.Code != http.StatusOK {
					t.Errorf("link to %s is %d", m[1], rec.Code)
				}
			}
			if !p.NoIndex {
				text := regexp.MustCompile(`(?s)<script.*?</script>|<[^>]+>`).ReplaceAllString(b, " ")
				if n := len(strings.Fields(text)); n < 200 {
					t.Errorf("%d words of text, want at least 200", n)
				}
			}
		})
	}
}

// TestSitemap checks that every sitemap URL is an indexable page that
// returns 200 and names itself as canonical, and that every indexable page
// is in the sitemap.
func TestSitemap(t *testing.T) {
	h := handler(t)
	locs := regexp.MustCompile(`<loc>([^<]+)</loc><lastmod>(\d{4}-\d{2}-\d{2})</lastmod>`).FindAllStringSubmatch(body(t, h, "/sitemap.xml"), -1)
	seen := map[string]bool{}
	for _, m := range locs {
		path := "/" + strings.TrimPrefix(m[1], siteURL)
		b := body(t, h, path)
		if got := canonicalRE.FindStringSubmatch(b)[1]; got != m[1] {
			t.Errorf("%s names %s as canonical", m[1], got)
		}
		if strings.Contains(b, "noindex") {
			t.Errorf("%s is noindex but in the sitemap", m[1])
		}
		seen[path] = true
	}
	for _, p := range pages {
		if seen[p.Path] == p.NoIndex {
			t.Errorf("%s: in sitemap %t, noindex %t", p.Path, seen[p.Path], p.NoIndex)
		}
	}
}

// TestStructuredDataMatchesPages checks that the structured data says what
// the pages show: the FAQ's questions and answers, the steps, the citations.
func TestStructuredDataMatchesPages(t *testing.T) {
	h := handler(t)
	src, err := files.ReadFile("content/pages/faq.md")
	if err != nil {
		t.Fatal(err)
	}
	faq := body(t, h, "/faq")
	nodes, types := structured(t, faq)
	if !types["FAQPage"] {
		t.Fatal("/faq has no FAQPage")
	}
	visible := html.UnescapeString(faq[strings.Index(faq, "<body>"):])
	questions := 0
	for _, n := range nodes {
		for _, q := range n.MainEntity {
			questions++
			for _, s := range []string{q.Name, q.Answer.Text} {
				if !strings.Contains(visible, s) || !bytes.Contains(src, []byte(s)) {
					t.Errorf("FAQ text not on the page and in faq.md: %q", s)
				}
			}
		}
	}
	if questions < 8 {
		t.Errorf("FAQPage has %d questions", questions)
	}

	start := body(t, h, "/get-started")
	nodes, types = structured(t, start)
	steps := 0
	for _, n := range nodes {
		for _, s := range n.Step {
			steps++
			if !strings.Contains(html.UnescapeString(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(start, "")), s.Text) {
				t.Errorf("HowTo step not on the page: %q", s.Text)
			}
		}
	}
	if !types["HowTo"] || steps != 5 {
		t.Errorf("/get-started: HowTo %t with %d steps", types["HowTo"], steps)
	}

	nodes, types = structured(t, body(t, h, "/evidence"))
	cited := 0
	for _, n := range nodes {
		cited += len(n.Citation)
	}
	if !types["TechArticle"] || cited != 3 {
		t.Errorf("/evidence: TechArticle %t with %d citations", types["TechArticle"], cited)
	}

	_, types = structured(t, body(t, h, "/"))
	for _, want := range []string{"SoftwareSourceCode", "SoftwareApplication"} {
		if !types[want] {
			t.Errorf("home page JSON-LD has no %s", want)
		}
	}
}

func TestNewHandlerErrors(t *testing.T) {
	good := fstest.MapFS{}
	fs.WalkDir(files, ".", func(name string, d fs.DirEntry, _ error) error {
		if !d.IsDir() {
			b, _ := files.ReadFile(name)
			good[name] = &fstest.MapFile{Data: b}
		}
		return nil
	})
	without := func(name string) fstest.MapFS {
		m := fstest.MapFS{}
		for k, v := range good {
			if k != name {
				m[k] = v
			}
		}
		return m
	}
	badTemplate := without("templates/page.html")
	badTemplate["templates/page.html"] = &fstest.MapFile{Data: []byte("{{.NoSuchField}}")}

	if _, err := newHandler(good); err != nil {
		t.Fatalf("complete file set: %v", err)
	}
	for name, fsys := range map[string]fstest.MapFS{
		"missing stylesheet": without("static/style.css"),
		"missing template":   without("templates/page.html"),
		"broken template":    badTemplate,
		"missing page":       without("content/pages/faq.md"),
		"missing 404 page":   without("content/pages/404.md"),
		"missing llms.txt":   without("content/llms.txt"),
	} {
		if _, err := newHandler(fsys); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestHashedName(t *testing.T) {
	if got := hashedName("style.css", []byte("x")); !regexp.MustCompile(`^style\.[0-9a-f]{10}\.css$`).MatchString(got) {
		t.Errorf("hashedName = %q", got)
	}
}

func TestRender(t *testing.T) {
	d := render("# A `title` [link](/x)\n\nintro {.lead}\n\n## Head {#custom}\n\n### Why *not*?\n\nOne\nline.\n\nTwo.\n\n" +
		"- a [`code` link](/y)\n- b **bold** <sub>x</sub> & <b>\n- **bold [across](/z) a link**\n\n1. first\n2. second\n\n1. other list\n\n" +
		"| Col `a` | B |\n| --- | --- |\n| 1 | *e* |\n\n```sh\n<tag> & x\n```\n\n```\nunterminated")
	if d.Title != "A title link" {
		t.Errorf("title %q", d.Title)
	}
	for _, want := range []string{
		`<p class="lead">intro</p>`,
		`<h2 id="custom">Head</h2>`,
		`<h3 id="why-not">Why <em>not</em>?</h3>`,
		`<p>One line.</p>`,
		`<li>a <a href="/y"><code>code</code> link</a></li>`,
		`<li>b <strong>bold</strong> <sub>x</sub> &amp; &lt;b&gt;</li>`,
		`<li><strong>bold <a href="/z">across</a> a link</strong></li>`,
		"<ol>\n<li>first</li>\n<li>second</li>\n</ol>",
		`<th scope="col">Col <code>a</code></th>`,
		`<td data-label="Col a">1</td><td data-label="B"><em>e</em></td>`,
		"<pre><code>&lt;tag&gt; &amp; x</code></pre>",
		"<pre><code>unterminated</code></pre>",
	} {
		if !strings.Contains(d.HTML, want) {
			t.Errorf("HTML lacks %q:\n%s", want, d.HTML)
		}
	}
	if len(d.Sections) != 1 || d.Sections[0].Heading != "Why not?" || d.Sections[0].Text != "One line. Two." {
		t.Errorf("sections %+v", d.Sections)
	}
	if len(d.Steps) != 2 || d.Steps[0] != "first" {
		t.Errorf("steps %q", d.Steps)
	}
	if got := plain("x<sup>2</sup> **y**"); got != "x^2 y" {
		t.Errorf("plain = %q", got)
	}
}

// --- main.go -------------------------------------------------------------------

func stubLog(t *testing.T) {
	t.Helper()
	old := logf
	logf = func(string, ...any) {}
	t.Cleanup(func() { logf = old })
}

func stubListen(t *testing.T, f func(*http.Server) error) {
	t.Helper()
	old := listen
	listen = f
	t.Cleanup(func() { listen = old })
}

func TestAddr(t *testing.T) {
	t.Setenv(envAddr, "")
	if addr() != defaultAddr {
		t.Errorf("default addr = %q", addr())
	}
	t.Setenv(envAddr, "0.0.0.0:9000")
	if addr() != "0.0.0.0:9000" {
		t.Errorf("addr = %q", addr())
	}
}

func TestRunServes(t *testing.T) {
	stubLog(t)
	t.Setenv(envAddr, "127.0.0.1:1234")
	var got *http.Server
	stubListen(t, func(srv *http.Server) error { got = srv; return nil })
	if code := run(nil, io.Discard); code != 0 {
		t.Fatalf("run = %d", code)
	}
	if got.Addr != "127.0.0.1:1234" || got.ReadHeaderTimeout == 0 {
		t.Errorf("server = %+v", got)
	}
	rec := httptest.NewRecorder()
	got.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("/healthz = %d", rec.Code)
	}
}

func TestRunListenError(t *testing.T) {
	stubLog(t)
	stubListen(t, func(*http.Server) error { return errors.New("address in use") })
	var stderr bytes.Buffer
	if code := run(nil, &stderr); code != 1 || !strings.Contains(stderr.String(), "address in use") {
		t.Errorf("run = %d, stderr %q", code, stderr.String())
	}
}

func TestRunBadFiles(t *testing.T) {
	old := siteFS
	siteFS = fstest.MapFS{}
	t.Cleanup(func() { siteFS = old })
	var stderr bytes.Buffer
	if code := run(nil, &stderr); code != 1 || !strings.HasPrefix(stderr.String(), "site:") {
		t.Errorf("run = %d, stderr %q", code, stderr.String())
	}
}

func TestRunUsage(t *testing.T) {
	var stderr bytes.Buffer
	if code := run([]string{"serve", "now"}, &stderr); code != 2 || !strings.Contains(stderr.String(), "usage") {
		t.Errorf("run = %d, stderr %q", code, stderr.String())
	}
}

func TestHealthcheck(t *testing.T) {
	ok := httptest.NewServer(handler(t))
	defer ok.Close()
	t.Setenv(envAddr, strings.TrimPrefix(ok.URL, "http://"))
	if code := run([]string{"healthcheck"}, io.Discard); code != 0 {
		t.Errorf("healthcheck against a healthy server = %d", code)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	var stderr bytes.Buffer
	if code := healthcheck(strings.TrimPrefix(bad.URL, "http://"), &stderr); code != 1 || !strings.Contains(stderr.String(), "500") {
		t.Errorf("healthcheck against a failing server = %d, %q", code, stderr.String())
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := ln.Addr().String()
	ln.Close()
	stderr.Reset()
	if code := healthcheck(closed, &stderr); code != 1 || !strings.HasPrefix(stderr.String(), "healthcheck:") {
		t.Errorf("healthcheck against a closed port = %d, %q", code, stderr.String())
	}
}

func TestMainExitsWithRunStatus(t *testing.T) {
	oldExit, oldArgs, oldStderr := exit, os.Args, os.Stderr
	t.Cleanup(func() { exit, os.Args, os.Stderr = oldExit, oldArgs, oldStderr })
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { devNull.Close() })
	os.Stderr = devNull

	got := -1
	exit = func(code int) { got = code }
	os.Args = []string{"site", "bogus"}
	main()
	if got != 2 {
		t.Errorf("main exited with %d, want 2", got)
	}
}

func TestDefaultListenReportsErrors(t *testing.T) {
	if err := listen(&http.Server{Addr: "127.0.0.1:-1"}); err == nil {
		t.Error("listen on an invalid address returned no error")
	}
}
