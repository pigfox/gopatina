package emit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/borrow"
	"github.com/pigfox/gopatina/internal/load"
	"github.com/pigfox/gopatina/internal/subset"
)

func TestCrateName(t *testing.T) {
	for dir, want := range map[string]string{
		"/x/hello":     "hello",
		"/x/My App.v2": "my_app_v2",
		"/x/2fast":     "app_2fast",
		"/x/go-tool":   "go-tool",
	} {
		if got := CrateName(dir); got != want {
			t.Errorf("CrateName(%q) = %q, want %q", dir, got, want)
		}
	}
}

func TestWrite(t *testing.T) {
	c := &Crate{Name: "x", Files: []File{{"src/main.rs", []byte("fn main() {}\n")}}}
	dir := filepath.Join(t.TempDir(), "crate")
	if err := c.Write(dir); err != nil {
		t.Fatal(err)
	}
	// A crate gopatina wrote can be written again.
	if err := c.Write(dir); err != nil {
		t.Fatal(err)
	}
	// An empty directory is fine.
	if err := c.Write(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	// A directory with other files is refused.
	other := t.TempDir()
	os.WriteFile(filepath.Join(other, "notes.txt"), nil, 0o644)
	if err := c.Write(other); err == nil || !strings.Contains(err.Error(), "was not written by gopatina") {
		t.Errorf("err = %v", err)
	}
	// A file in the way.
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, nil, 0o644)
	if err := c.Write(file); err == nil {
		t.Error("Write over a file succeeded")
	}
	// A file where a directory must go.
	blocked := t.TempDir()
	os.WriteFile(filepath.Join(blocked, "src"), nil, 0o644)
	os.WriteFile(filepath.Join(blocked, ManifestName), nil, 0o644)
	if err := c.Write(blocked); err == nil {
		t.Error("Write with src as a file succeeded")
	}
	// A directory where a file must go.
	clash := t.TempDir()
	os.MkdirAll(filepath.Join(clash, "src", "main.rs"), 0o755)
	os.WriteFile(filepath.Join(clash, ManifestName), nil, 0o644)
	if err := c.Write(clash); err == nil {
		t.Error("Write with src/main.rs as a directory succeeded")
	}
}

func TestNames(t *testing.T) {
	for in, want := range map[string]string{"ScaleBy": "scale_by", "HTTPServer": "http_server", "x2Y": "x2_y", "ab": "ab"} {
		if got := snake(in); got != want {
			t.Errorf("snake(%q) = %q, want %q", in, got, want)
		}
	}
	if camel("_") != "T" || camel("my_type") != "MyType" {
		t.Errorf("camel: %q %q", camel("_"), camel("my_type"))
	}
	if ident("type") != "r#type" || ident("self") != "self_" || ident("x") != "x" {
		t.Error("ident")
	}
	if itoa(0) != "0" || itoa(42) != "42" {
		t.Error("itoa")
	}
}

func TestRTModule(t *testing.T) {
	if rtModule(nil) != "" {
		t.Error("an empty runtime module is not empty")
	}
	s := rtModule(map[string]bool{"Slice": true})
	for _, want := range []string{"pub struct Slice", "pub trait Show", "pub struct Float", "use std::fmt;"} {
		if !strings.Contains(s, want) {
			t.Errorf("runtime module does not contain %q", want)
		}
	}
}

func TestLayout(t *testing.T) {
	recv := &call{head: "f", open: "(", close: ")"}
	long := strings.Repeat("a", 40)
	c := &call{recv: recv, head: ".m", open: "(", close: ")", args: []node{text(long), text(long)}}
	got := lay(c, 4, 4)
	want := "f().m(\n        " + long + ",\n        " + long + ",\n    )"
	if got != want {
		t.Errorf("lay =\n%s\nwant\n%s", got, want)
	}
	a := &addrOf{"&", text("x")}
	if a.prec() != precUnary || a.flat() != "&x" {
		t.Error("addrOf")
	}
}

// FuzzEmit runs the whole pipeline on Go sources. Whatever the source, it
// must not panic, and a package it accepts must emit the same crate twice.
func FuzzEmit(f *testing.F) {
	for _, dir := range programs(f, "golden") {
		src, err := os.ReadFile(filepath.Join(dir, "main.go"))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(string(src))
	}
	f.Fuzz(func(t *testing.T, src string) {
		pkg, err := load.Source("/fuzz/app", "main.go", src, "go1.25")
		if err != nil {
			return
		}
		if len(subset.Check(pkg)) > 0 {
			return
		}
		res, ds := borrow.Check(pkg)
		if len(ds) > 0 {
			return
		}
		opt := Options{Target: goldenTarget, Version: "fuzz"}
		a, b := Emit(pkg, res, opt), Emit(pkg, res, opt)
		if string(a.Manifest()) != string(b.Manifest()) {
			t.Fatalf("emission is not deterministic:\n%s\n%s", a.Manifest(), b.Manifest())
		}
	})
}
