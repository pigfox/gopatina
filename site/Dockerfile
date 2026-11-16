# syntax=docker/dockerfile:1
#
# gopatina.org's server, built with gopatina itself.
# Build context: the repository root (see site/docker-compose.yml).

# --- build: install a pinned gopatina, then build the site with it -----------
FROM golang:1.27-alpine AS build
ARG GOPATINA_VERSION=v0.0.1
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
RUN go install github.com/pigfox/gopatina/cmd/gopatina@${GOPATINA_VERSION}

WORKDIR /src
COPY go.mod ./
COPY site/ site/
# gopatina's default mode: passes through to go build. A static binary.
RUN gopatina build -ldflags="-s -w" -o /out/gopatina-site ./site

# --- run: no shell, no package manager, non-root ----------------------------
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/gopatina-site /gopatina-site
USER nonroot:nonroot
ENV GOPATINA_SITE_ADDR=0.0.0.0:8590
EXPOSE 8590
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD ["/gopatina-site", "healthcheck"]
ENTRYPOINT ["/gopatina-site"]
