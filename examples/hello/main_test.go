package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

const want = "Hello, Gopher! Hello, Gopher!\nFerris 3\n"

func TestRun(t *testing.T) {
	var buf bytes.Buffer
	run(&buf)
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestMainWritesToStdout(t *testing.T) {
	if got := captureStdout(t, main); got != want {
		t.Errorf("main printed %q, want %q", got, want)
	}
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdout")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = file
	f()
	os.Stdout = old
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
