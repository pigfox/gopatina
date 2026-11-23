// Package interop tests the rust-interop example as its CI does: the
// generated crate matches its Go source, and the Rust application, built
// with plain cargo, prints what the Go original prints on every input.
package interop

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func needRust(t *testing.T) {
	if os.Getenv("GOPATINA_RUST_TESTS") != "1" {
		t.Skip("set GOPATINA_RUST_TESTS=1 to build and run the Rust workspace")
	}
}

func command(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

// TestCrateIsRegenerated checks that the committed crate is exactly what
// gopatina build -emit-rust writes from go/rules today.
func TestCrateIsRegenerated(t *testing.T) {
	needRust(t)
	out := filepath.Join(t.TempDir(), "rules")
	command(t, filepath.Join("..", ".."), "go", "run", "./cmd/gopatina", "build", "-emit-rust", "-o", out, "./examples/rust-interop/go/rules")
	err := filepath.WalkDir(out, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(out, path)
		want, err := os.ReadFile(filepath.Join("rules", rel))
		got, _ := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("rules/%s differs from a fresh gopatina build -emit-rust: regenerate it", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRustMatchesGo builds the Rust application with cargo and the Go
// original with go, runs both on every input pair, and requires the same
// standard output, standard error and exit status.
func TestRustMatchesGo(t *testing.T) {
	needRust(t)
	command(t, ".", "cargo", "test", "--quiet", "--locked")
	command(t, ".", "cargo", "build", "--quiet", "--locked", "--release")
	goBin := filepath.Join(t.TempDir(), "quote-go")
	command(t, ".", "go", "build", "-o", goBin, "./go/cmd/quote")
	rustBin := filepath.Join("target", "release", "quote")
	for _, args := range [][]string{
		{"shop.rules", "small.order"},
		{"shop.rules", "big.order"},
		{"shop.rules", "empty.order"},
		{"shop.rules", "bad.order"},
		{"bad.rules", "small.order"},
		{"over.rules", "small.order"},
		{"shop.rules"},
	} {
		var paths []string
		for _, a := range args {
			paths = append(paths, filepath.Join("testdata", a))
		}
		g, r := runBin(goBin, paths), runBin(rustBin, paths)
		if g != r {
			t.Errorf("%v: Go and Rust differ:\ngo:   %+v\nrust: %+v", args, g, r)
		}
	}
}

type result struct {
	stdout, stderr string
	code           int
}

func runBin(bin string, args []string) result {
	cmd := exec.Command(bin, args...)
	var out, errs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errs
	err := cmd.Run()
	code := 0
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	}
	return result{out.String(), errs.String(), code}
}
