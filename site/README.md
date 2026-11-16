# gopatina.org

The server for [gopatina.org](https://gopatina.org): one responsive page
plus `/index.md` (the page as Markdown), `/llms.txt`, `/robots.txt`,
`/sitemap.xml` and `/healthz`. Everything is embedded in the binary. It is
plain Go (`net/http`, `html/template`, `embed`; no third-party modules), with
no JavaScript on the page.

The server is a Go program **built with gopatina's default mode**
(`gopatina build`, which runs `go build`). GoPatina's Rust modes are not
built yet, and the page says so.

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
site-check http://127.0.0.1:8590/
npx -y lighthouse@13.5.0 http://127.0.0.1:8590/ --quiet --chrome-flags="--headless=new"
```

## Editing the page

The page is `templates/index.html`; its Markdown twin is `content/index.md`.
Keep them in step: the tests check that every FAQ question and answer in the
page's FAQPage structured data appears, word for word, on the page and in
`index.md`. Update `lastUpdated` in `site.go` and `dateModified` in the
JSON-LD together (also tested).
