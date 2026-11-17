// Command site serves https://gopatina.org: one page, its Markdown version,
// llms.txt, robots.txt, sitemap.xml and a health check, all from files
// embedded in the binary.
//
// It is built with gopatina's default mode (gopatina build, which runs go
// build), so it is an ordinary Go program; GoPatina's Rust modes are not built
// yet.
//
// Usage:
//
//	site              serve on $GOPATINA_SITE_ADDR (default 127.0.0.1:8590)
//	site healthcheck  exit 0 if the running server answers /healthz
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	envAddr     = "GOPATINA_SITE_ADDR"
	defaultAddr = "127.0.0.1:8590"
)

// Replaceable in tests.
var (
	exit   = os.Exit
	listen = func(srv *http.Server) error { return srv.ListenAndServe() }
	logf   = log.Printf
)

func main() {
	exit(run(os.Args[1:], os.Stderr))
}

// addr returns the address to listen on.
func addr() string {
	if a := os.Getenv(envAddr); a != "" {
		return a
	}
	return defaultAddr
}

// run starts the server, or runs the health check, and returns the exit
// status.
func run(args []string, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "healthcheck" {
		return healthcheck(addr(), stderr)
	}
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: site [healthcheck]")
		return 2
	}
	h, err := newHandler(siteFS)
	if err != nil {
		fmt.Fprintln(stderr, "site:", err)
		return 1
	}
	srv := &http.Server{
		Addr:              addr(),
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	logf("site: listening on %s", srv.Addr)
	if err := listen(srv); err != nil {
		fmt.Fprintln(stderr, "site:", err)
		return 1
	}
	return 0
}

// healthcheck asks the server at addr for /healthz.
func healthcheck(addr string, stderr io.Writer) int {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		fmt.Fprintln(stderr, "healthcheck:", err)
		return 1
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(stderr, "healthcheck: status", resp.StatusCode)
		return 1
	}
	return 0
}
