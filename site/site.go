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

//go:embed templates/index.html content static
var files embed.FS

// siteFS is where the server reads its files from; replaceable in tests.
var siteFS fs.FS = files

// Site-wide facts, used by the page, the Markdown version and the sitemap.
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
	"Content-Security-Policy":      "default-src 'none'; style-src 'self'; img-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
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
	CSS         string
	Favicon     string
	LastUpdated string
	URL         string
}

// newHandler builds every response from the embedded files and returns the
// site's handler.
func newHandler(fsys fs.FS) (http.Handler, error) {
	routes := map[string]*resource{}

	static := map[string]string{}
	for _, f := range []struct{ name, ctype string }{
		{"style.css", "text/css; charset=utf-8"},
		{"favicon.svg", "image/svg+xml"},
	} {
		body, err := fs.ReadFile(fsys, "static/"+f.name)
		if err != nil {
			return nil, err
		}
		path := "/static/" + hashedName(f.name, body)
		static[f.name] = path
		routes[path] = newResource(body, f.ctype, cacheImmutable)
	}

	tmpl, err := template.ParseFS(fsys, "templates/index.html")
	if err != nil {
		return nil, err
	}
	var page bytes.Buffer
	data := pageData{CSS: static["style.css"], Favicon: static["favicon.svg"], LastUpdated: lastUpdated, URL: siteURL}
	if err := tmpl.Execute(&page, data); err != nil {
		return nil, err
	}
	routes["/"] = newResource(page.Bytes(), "text/html; charset=utf-8", cacheShort)

	for _, f := range []struct{ path, file, ctype string }{
		{"/index.md", "content/index.md", "text/markdown; charset=utf-8"},
		{"/llms.txt", "content/llms.txt", "text/plain; charset=utf-8"},
	} {
		body, err := fs.ReadFile(fsys, f.file)
		if err != nil {
			return nil, err
		}
		routes[f.path] = newResource(body, f.ctype, cacheShort)
	}

	routes["/robots.txt"] = newResource([]byte("User-agent: *\nAllow: /\n\nSitemap: "+siteURL+"sitemap.xml\n"),
		"text/plain; charset=utf-8", cacheShort)
	routes["/sitemap.xml"] = newResource([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>`+siteURL+`</loc><lastmod>`+lastUpdated+`</lastmod></url>
</urlset>
`), "application/xml; charset=utf-8", cacheShort)
	routes["/healthz"] = newResource([]byte("ok\n"), "text/plain; charset=utf-8", "no-store")

	notFound := newResource([]byte("404 page not found\n"), "text/plain; charset=utf-8", "no-store")

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
		res, ok := routes[r.URL.Path]
		status := http.StatusOK
		if !ok {
			res, status = notFound, http.StatusNotFound
		}
		serve(w, r, res, status)
	}), nil
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
