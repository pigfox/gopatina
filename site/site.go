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
	var nav []navItem
	for _, q := range pages {
		if !q.NoIndex {
			nav = append(nav, navItem{Path: q.Path, Name: q.Crumb})
		}
	}
	// build renders one page, and returns it with its Markdown source.
	build := func(p page) (*resource, []byte, error) {
		src, err := fs.ReadFile(fsys, "content/pages/"+p.File+".md")
		if err != nil {
			return nil, nil, err
		}
		d := render(string(src))
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
	for _, p := range pages {
		res, src, err := build(p)
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
	routes["/sitemap.xml"] = newResource([]byte(sitemap.String()), "application/xml; charset=utf-8", cacheShort)

	head, err := fs.ReadFile(fsys, "content/llms.txt")
	if err != nil {
		return nil, err
	}
	routes["/llms.txt"] = newResource([]byte(strings.Replace(string(head), "{{pages}}\n", llms.String(), 1)), "text/plain; charset=utf-8", cacheShort)
	routes["/robots.txt"] = newResource([]byte("User-agent: *\nAllow: /\n\nSitemap: "+siteURL+"sitemap.xml\n"),
		"text/plain; charset=utf-8", cacheShort)
	routes["/healthz"] = newResource([]byte("ok\n"), "text/plain; charset=utf-8", "no-store")

	notFound, _, err := build(notFoundPage)
	if err != nil {
		return nil, err
	}
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
