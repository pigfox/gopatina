# gopatina.org

The server for [gopatina.org](https://gopatina.org): a few responsive pages,
each at its own address (`/`, `/how-it-works`, `/get-started`, `/evidence`,
`/benchmarks`, `/correctness`, `/roadmap`, `/faq`, and `/sponsor`, kept out
of search engines until it has content), each also served as Markdown
(`/index.md` for the home page, `/faq.md` and so on), a 404 page,
`/llms.txt`, `/robots.txt`, `/sitemap.xml` and `/healthz`. Everything is
embedded in the binary. It is plain Go (`net/http`, `html/template`,
`embed`; no third-party modules).

There is one script, on the home page only: `static/anchors.js`, a few lines
of vanilla JavaScript that send the old one-page links (`/#faq` and the
like) to their new pages, since a server never sees the part after `#`.
Addresses have no trailing slash; `/faq/` and `/index.html` answer with a
301 to `/faq` and `/`.

The server is a Go program **built with gopatina's default mode**
(`gopatina build`, which runs `go build`). It uses `net/http` and `embed`,
which are outside the phase 1 subset GoPatina's Rust modes accept.

## Run it locally

From the repository root:

```sh
go run ./site                                   # serves http://127.0.0.1:8590/
GOPATINA_SITE_ADDR=127.0.0.1:9000 go run ./site # another address
```

Or build it the way the container does:

```sh
go install github.com/pigfox/gopatina/cmd/gopatina@v0.0.1
CGO_ENABLED=0 gopatina build -trimpath -ldflags="-s -w" -o gopatina-site ./site
./gopatina-site
```

## Run it in Docker

From the repository root (Docker Compose v2):

```sh
CREATED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)" docker compose -f site/docker-compose.yml up -d --build
curl -s http://127.0.0.1:8590/healthz           # ok
CREATED_AT=unused docker compose -f site/docker-compose.yml down --rmi local
```

The image installs a pinned gopatina (`GOPATINA_VERSION`, default `v0.0.1`),
builds a static binary with it, and runs it on distroless `static:nonroot`
with a read-only filesystem, no capabilities, and a built-in health check
(`gopatina-site healthcheck`). The container, image and network are named
`gopatina-site` and carry the `org.pigfox.*` labels. The port is bound to
`127.0.0.1:8590` only: TLS and HSTS belong to the reverse proxy in front.

## Checks

```sh
go test -race -cover ./site       # 100% coverage, enforced by CI
for p in / /how-it-works /get-started /evidence /benchmarks /correctness /roadmap /faq; do
  site-check http://127.0.0.1:8590$p
  npx -y lighthouse@13.5.0 http://127.0.0.1:8590$p --quiet --chrome-flags="--headless=new"
  node scripts/site-overflow.mjs "$(command -v google-chrome)" http://127.0.0.1:8590$p
done
```

The tests check every page: its own title (30 to 60 characters) and
description (70 to 160), one h1, headings in order, its canonical URL, Open
Graph and Twitter tags, structured data (with a BreadcrumbList on inner
pages), that every link within the site leads to a page, and that every
sitemap URL returns 200 and names itself as canonical. CI also loads every
page in Chrome at phone widths and fails if one scrolls sideways.

## Editing the pages

Each page is written once, in Markdown, in `content/pages/<name>.md`: the
server renders it to HTML (`markdown.go`, a small renderer for exactly what
the pages use) inside `templates/page.html`, and serves the source as the
page's Markdown version. The list of pages, with each one's address, title,
description and kind, is in `pages.go`; the structured data is built from it
and from the page itself: the FAQ page's questions are its `###` headings
and its answers the paragraphs under them, and the get-started page's
HowTo steps are its numbered list. When a page changes, update its
`Updated` date in `pages.go`; the sitemap and the structured data use it.
