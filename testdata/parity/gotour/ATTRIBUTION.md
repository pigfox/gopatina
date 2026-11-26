# A Tour of Go

These programs are from [A Tour of Go](https://go.dev/tour), copied
without change from the Go website's repository,
https://go.googlesource.com/website at commit
0eef789c9f9a7cc074eab1b3fb2d96baea282546 (2026-11-18): each
`_content/tour/<path>.go` is `<path>.go.txt` here. The `.txt` suffix keeps
them out of this repository's Go tooling (one, flowcontrol/for-continued,
is deliberately not gofmt-formatted); they are data that `gopatina-parity`
measures GoPatina against (docs/QUALITY.md). The tour marks each file
`//go:build OMIT`; the harness drops that line in its copy.

Copyright 2009 The Go Authors, under the BSD-style licence in
[LICENSE](LICENSE), the website repository's own.
