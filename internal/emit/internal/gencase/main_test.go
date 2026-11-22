package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	out := filepath.Join(t.TempDir(), "casetables.go")
	if c := run([]string{out}); c != 0 {
		t.Fatalf("exit %d", c)
	}
	data, _ := os.ReadFile(out)
	if !strings.Contains(string(data), "const UnicodeVersion = ") {
		t.Errorf("output: %.200s", data)
	}
	if c := run(nil); c != 2 {
		t.Errorf("no arguments: exit %d", c)
	}
	if c := run([]string{filepath.Join(out, "no", "such")}); c != 1 {
		t.Errorf("unwritable: exit %d", c)
	}
	old, oldArgs := exit, os.Args
	t.Cleanup(func() { exit, os.Args = old, oldArgs })
	got := -1
	exit = func(c int) { got = c }
	os.Args = []string{"gencase", filepath.Join(t.TempDir(), "x.go")}
	main()
	if got != 0 {
		t.Errorf("main: exit %d", got)
	}
}
