package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"html"
	"io"
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

func page(t *testing.T) string {
	t.Helper()
	rec := get(t, handler(t), http.MethodGet, "/", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d", rec.Code)
	}
	return rec.Body.String()
}

func TestRoutes(t *testing.T) {
	h := handler(t)
	tests := []struct {
		path, ctype, cache, contains string
	}{
		{"/", "text/html; charset=utf-8", cacheShort, "<h1>Go<span"},
		{"/index.md", "text/markdown; charset=utf-8", cacheShort, "# GoPatina"},
		{"/llms.txt", "text/plain; charset=utf-8", cacheShort, "> GoPatina is an open-source"},
		{"/robots.txt", "text/plain; charset=utf-8", cacheShort, "Sitemap: https://gopatina.org/sitemap.xml"},
		{"/sitemap.xml", "application/xml; charset=utf-8", cacheShort, "<lastmod>" + lastUpdated + "</lastmod>"},
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
}

func TestStaticAssetsAreHashedAndImmutable(t *testing.T) {
	h := handler(t)
	body := page(t)
	for _, re := range []string{`href="(/static/style\.[0-9a-f]{10}\.css)"`, `href="(/static/favicon\.[0-9a-f]{10}\.svg)"`} {
		m := regexp.MustCompile(re).FindStringSubmatch(body)
		if m == nil {
			t.Fatalf("page has no link matching %s", re)
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

func TestGzip(t *testing.T) {
	rec := get(t, handler(t), http.MethodGet, "/", true)
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
	if string(plain) != page(t) {
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
	if rec := get(t, h, http.MethodGet, "/nope", false); rec.Code != http.StatusNotFound {
		t.Errorf("GET /nope = %d", rec.Code)
	}
	rec := get(t, h, http.MethodPost, "/", false)
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Errorf("POST / = %d, Allow %q", rec.Code, rec.Header().Get("Allow"))
	}
}

func TestPageSEO(t *testing.T) {
	body := page(t)
	title := regexp.MustCompile(`<title>(.*?)</title>`).FindStringSubmatch(body)
	if title == nil || len(title[1]) < 30 || len(title[1]) > 60 {
		t.Errorf("title %q must be 30 to 60 characters", title)
	}
	if n := strings.Count(body, "<h1"); n != 1 {
		t.Errorf("page has %d h1 elements, want 1", n)
	}
	for _, want := range []string{
		`<link rel="canonical" href="https://gopatina.org/">`,
		`<meta name="description"`,
		`<meta property="og:title"`,
		`<meta name="twitter:card"`,
		`<time datetime="` + lastUpdated + `">`,
		"Go developers get Rust's performance and safety without the painful Rust learning curve.",
		"GoPatina is an experiment by",
		"built with gopatina's default mode",
		"No official test-server run has happened yet.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	// Markdown link syntax left in the HTML shows a raw URL, which cannot
	// wrap and widens the page on a phone (scripts/site-overflow.mjs).
	if strings.Contains(body, "](http") {
		t.Error("page contains a Markdown link; write it as an HTML link")
	}
	if strings.Contains(body, "<script>") || strings.Contains(body, "<script src") {
		t.Error("page must not contain executable scripts")
	}
	text := regexp.MustCompile(`(?s)<script.*?</script>|<[^>]+>`).ReplaceAllString(body, " ")
	if n := len(strings.Fields(text)); n < 200 {
		t.Errorf("page has %d words of text, want at least 200", n)
	}
}

type faqDoc struct {
	Graph []struct {
		Type         any    `json:"@type"`
		DateModified string `json:"dateModified"`
		MainEntity   []struct {
			Name   string `json:"name"`
			Answer struct {
				Text string `json:"text"`
			} `json:"acceptedAnswer"`
		} `json:"mainEntity"`
	} `json:"@graph"`
}

func TestStructuredDataMatchesPage(t *testing.T) {
	body := page(t)
	m := regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`).FindStringSubmatch(body)
	if m == nil {
		t.Fatal("no JSON-LD")
	}
	var doc faqDoc
	if err := json.Unmarshal([]byte(m[1]), &doc); err != nil {
		t.Fatalf("JSON-LD does not parse: %v", err)
	}
	md, err := files.ReadFile("content/index.md")
	if err != nil {
		t.Fatal(err)
	}
	visible := html.UnescapeString(body[strings.Index(body, "<body>"):])
	types, questions := map[string]bool{}, 0
	for _, node := range doc.Graph {
		switch v := node.Type.(type) {
		case string:
			types[v] = true
		case []any:
			for _, s := range v {
				types[s.(string)] = true
			}
		}
		if node.DateModified != "" && node.DateModified != lastUpdated {
			t.Errorf("dateModified = %q, want %q", node.DateModified, lastUpdated)
		}
		for _, q := range node.MainEntity {
			questions++
			for _, s := range []string{q.Name, q.Answer.Text} {
				if !strings.Contains(visible, s) {
					t.Errorf("FAQ text not on the page: %q", s)
				}
				if !bytes.Contains(md, []byte(s)) {
					t.Errorf("FAQ text not in index.md: %q", s)
				}
			}
		}
	}
	for _, want := range []string{"Organization", "WebSite", "WebPage", "SoftwareSourceCode", "SoftwareApplication", "FAQPage"} {
		if !types[want] {
			t.Errorf("JSON-LD has no %s", want)
		}
	}
	if questions < 5 {
		t.Errorf("FAQPage has %d questions", questions)
	}
}

func TestNewHandlerErrors(t *testing.T) {
	good := fstest.MapFS{}
	for _, name := range []string{"static/style.css", "static/favicon.svg", "templates/index.html", "content/index.md", "content/llms.txt"} {
		b, err := files.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		good[name] = &fstest.MapFile{Data: b}
	}
	without := func(name string) fstest.MapFS {
		m := fstest.MapFS{}
		for k, v := range good {
			if k != name {
				m[k] = v
			}
		}
		return m
	}
	badTemplate := without("templates/index.html")
	badTemplate["templates/index.html"] = &fstest.MapFile{Data: []byte("{{.NoSuchField}}")}

	if _, err := newHandler(good); err != nil {
		t.Fatalf("complete file set: %v", err)
	}
	for name, fsys := range map[string]fstest.MapFS{
		"missing stylesheet": without("static/style.css"),
		"missing template":   without("templates/index.html"),
		"broken template":    badTemplate,
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
