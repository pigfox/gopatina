package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed templates/page.html content static
var files embed.FS

// siteFS is where the server reads its files from; replaceable in tests.
var siteFS fs.FS = files

// Site-wide facts, used by the pages, their Markdown and the sitemap.
const (
	siteURL     = "https://gopatina.org/"
	lastUpdated = "2026-11-21"
)

// githubSponsorsURL is GoPatina's GitHub Sponsors page (live since
// 2026-11-22): /sponsor links to it, or, set to "", says it is coming soon.
var githubSponsorsURL = "https://github.com/sponsors/pigfox"

// sponsorLine is the /sponsor line {{github-sponsors}} stands for.
func sponsorLine() string {
	if githubSponsorsURL == "" {
		return "GitHub Sponsors: coming soon."
	}
	return "Sponsor through [GitHub Sponsors](" + githubSponsorsURL + ")."
}

// resource is a response prepared once at startup: the body, its gzip
// encoding, and its headers.
type resource struct {
	body        []byte
	gzipped     []byte
	contentType string
	cache       string
}

const (
	cacheShort     = "public, max-age=300"
	cacheImmutable = "public, max-age=31536000, immutable"
)

// securityHeaders are sent with every response. HSTS is left to the TLS
// proxy in front of the server.
var securityHeaders = map[string]string{
	"Content-Security-Policy":      "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
	"X-Content-Type-Options":       "nosniff",
	"X-Frame-Options":              "DENY",
	"Referrer-Policy":              "strict-origin-when-cross-origin",
	"Permissions-Policy":           "camera=(), microphone=(), geolocation=(), interest-cohort=()",
	"Cross-Origin-Opener-Policy":   "same-origin",
	"Cross-Origin-Resource-Policy": "same-origin",
}

// newResource prepares body for serving.
func newResource(body []byte, contentType, cache string) *resource {
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression) // the level is valid
	zw.Write(body)
	zw.Close()
	return &resource{body: body, gzipped: buf.Bytes(), contentType: contentType, cache: cache}
}

// hashedName returns name with a short content hash before its extension,
// e.g. style.css -> style.3f2a9c1b04.css.
func hashedName(name string, body []byte) string {
	sum := sha256.Sum256(body)
	i := strings.LastIndex(name, ".")
	return name[:i] + "." + hex.EncodeToString(sum[:5]) + name[i:]
}

// pageData is what the page template needs.
type pageData struct {
	page
	URL, Markdown, CSS, NoJSCSS, Favicon, Script, NavScript, OGType string
	H1                                                              string
	Body                                                            template.HTML
	JSONLD                                                          template.JS
	Nav                                                             []navItem
}

// navItem is one link of the site navigation.
type navItem struct {
	Path, Name string
	Current    bool
	Class      string // "sponsor" for the Sponsor item, which stands out
}

// newHandler builds every response from the embedded files and returns the
// site's handler.
func newHandler(fsys fs.FS) (http.Handler, error) {
	routes := map[string]*resource{}

	static := map[string]string{}
	for _, f := range []struct{ name, ctype string }{
		{"style.css", "text/css; charset=utf-8"},
		{"nojs.css", "text/css; charset=utf-8"},
		{"favicon.svg", "image/svg+xml"},
		{"anchors.js", "text/javascript; charset=utf-8"},
		{"nav.js", "text/javascript; charset=utf-8"},
	} {
		body, err := fs.ReadFile(fsys, "static/"+f.name)
		if err != nil {
			return nil, err
		}
		path := "/static/" + hashedName(f.name, body)
		static[f.name] = path
		routes[path] = newResource(body, f.ctype, cacheImmutable)
	}

	tmpl, err := template.ParseFS(fsys, "templates/page.html")
	if err != nil {
		return nil, err
	}
	posts, err := loadPosts(fsys)
	if err != nil {
		return nil, err
	}
	blog := blogPage
	blog.NoIndex = len(posts) == 0 // in the navigation and sitemap once it has a post
	blog.Updated = lastUpdated
	for _, p := range posts {
		blog.Updated = max(blog.Updated, p.Updated)
	}
	var nav []navItem
	for _, q := range append(append([]page{}, pages...), blog) {
		if !q.NoIndex && q.Path != "/" { // the logo links home on every page
			n := navItem{Path: q.Path, Name: q.Crumb}
			if q.Path == "/sponsor" {
				n.Class = "sponsor"
			}
			nav = append(nav, n)
		}
	}
	bodies := map[string]string{} // each post's HTML, for the feed
	// build renders one page from its Markdown, and returns it with the
	// Markdown.
	build := func(p page, src []byte) (*resource, []byte, error) {
		d := render(string(src))
		switch p.Kind {
		case "post":
			d.HTML = postHTML(p, d.HTML)
			bodies[p.Path] = d.HTML
		case "blog":
			for _, q := range posts {
				d.Posts = append(d.Posts, q.page)
			}
		}
		data := pageData{
			page: p, URL: p.url(), Markdown: p.markdownPath(), CSS: static["style.css"], NoJSCSS: static["nojs.css"], Favicon: static["favicon.svg"], NavScript: static["nav.js"],
			OGType: "article", H1: d.Title, Body: template.HTML(d.HTML), JSONLD: p.jsonLD(d),
		}
		if p.Path == "/" {
			// The home page has no breadcrumb, and sends old #links on.
			data.OGType, data.Script, data.page.Crumb = "website", static["anchors.js"], ""
		}
		if p == notFoundPage {
			data.Markdown = "" // the 404 page has no Markdown version
		}
		for _, n := range nav {
			n.Current = n.Path == p.Path
			data.Nav = append(data.Nav, n)
		}
		var b bytes.Buffer
		if err := tmpl.Execute(&b, data); err != nil {
			return nil, nil, err
		}
		return newResource(b.Bytes(), "text/html; charset=utf-8", cacheShort), src, nil
	}

	var sitemap, llms strings.Builder
	sitemap.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	type source struct {
		p   page
		src []byte
	}
	var all []source
	for _, p := range pages {
		src, err := fs.ReadFile(fsys, "content/pages/"+p.File+".md")
		if err != nil {
			return nil, err
		}
		src = []byte(strings.ReplaceAll(string(src), "{{github-sponsors}}", sponsorLine()))
		all = append(all, source{p, src})
	}
	all = append(all, source{blog, []byte(blogIndex(posts))})
	for _, q := range posts {
		all = append(all, source{q.page, []byte(q.src)})
	}
	for _, s := range all {
		p := s.p
		res, src, err := build(p, s.src)
		if err != nil {
			return nil, err
		}
		routes[p.Path] = res
		routes[p.markdownPath()] = newResource(src, "text/markdown; charset=utf-8", cacheShort)
		if !p.NoIndex {
			sitemap.WriteString("  <url><loc>" + p.url() + "</loc><lastmod>" + p.Updated + "</lastmod></url>\n")
			llms.WriteString("- [" + p.Crumb + "](" + p.url() + "): " + p.Description + " As Markdown: " + siteURL + p.markdownPath()[1:] + "\n")
		}
	}
	sitemap.WriteString("</urlset>\n")
	routes["/blog/feed.xml"] = newResource([]byte(atomFeed(posts, bodies)), "application/atom+xml; charset=utf-8", cacheShort)
	routes["/sitemap.xml"] = newResource([]byte(sitemap.String()), "application/xml; charset=utf-8", cacheShort)

	head, err := fs.ReadFile(fsys, "content/llms.txt")
	if err != nil {
		return nil, err
	}
	routes["/llms.txt"] = newResource([]byte(strings.Replace(string(head), "{{pages}}\n", llms.String(), 1)), "text/plain; charset=utf-8", cacheShort)
	routes["/robots.txt"] = newResource([]byte("User-agent: *\nAllow: /\n\nSitemap: "+siteURL+"sitemap.xml\n"),
		"text/plain; charset=utf-8", cacheShort)
	routes["/healthz"] = newResource([]byte("ok\n"), "text/plain; charset=utf-8", "no-store")
	// The funding manifest (fundingjson.org), which the repository's
	// .well-known/funding-manifest-urls names.
	funding, err := fs.ReadFile(fsys, "content/funding.json")
	if err != nil {
		return nil, err
	}
	routes["/funding.json"] = newResource(funding, "application/json", cacheShort)
	// The Early Sponsor badge, at a fixed address sponsors can link to
	// (content/pages/sponsor.md).
	badge, err := fs.ReadFile(fsys, "static/early-sponsor.svg")
	if err != nil {
		return nil, err
	}
	routes["/badges/early-sponsor.svg"] = newResource(badge, "image/svg+xml", cacheShort)

	src404, err := fs.ReadFile(fsys, "content/pages/"+notFoundPage.File+".md")
	if err != nil {
		return nil, err
	}
	// The template has rendered every other page by now, so it renders
	// this one too.
	notFound, _, _ := build(notFoundPage, src404)
	notFound.cache = "no-store"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		for k, v := range securityHeaders {
			h.Set(k, v)
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			h.Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := r.URL.Path
		// One address per page: no trailing slash, and / for /index.html.
		if target := canonicalPath(path); target != path && routes[target] != nil {
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		res, ok := routes[path]
		status := http.StatusOK
		if !ok {
			res, status = notFound, http.StatusNotFound
		}
		serve(w, r, res, status)
	}), nil
}

// canonicalPath returns the one address of a page that path may also be
// written as: without trailing slashes, and / for /index.html.
func canonicalPath(path string) string {
	if path == "/index.html" {
		return "/"
	}
	if trimmed := strings.TrimRight(path, "/"); trimmed != "" {
		return trimmed
	}
	return "/"
}

// serve writes res, gzipped when the client accepts it.
func serve(w http.ResponseWriter, r *http.Request, res *resource, status int) {
	h := w.Header()
	h.Set("Content-Type", res.contentType)
	h.Set("Cache-Control", res.cache)
	h.Set("Vary", "Accept-Encoding")
	body := res.body
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		h.Set("Content-Encoding", "gzip")
		body = res.gzipped
	}
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		w.Write(body)
	}
}
