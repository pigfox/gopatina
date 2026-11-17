package diag

import (
	"go/token"
	"path/filepath"
	"testing"
)

func TestFormat(t *testing.T) {
	dir := filepath.FromSlash("/work/app")
	d := Errorf(token.Position{Filename: filepath.Join(dir, "main.go"), Line: 3, Column: 7}, "x is %s", "bad").With("first note", "second note")
	want := "." + string(filepath.Separator) + "main.go:3:7: x is bad\n    first note\n    second note\n"
	if got := d.Format(dir); got != want {
		t.Errorf("Format = %q, want %q", got, want)
	}
}

func TestRelPath(t *testing.T) {
	dir := filepath.FromSlash("/work/app")
	tests := map[string]string{
		filepath.Join(dir, "sub", "a.go"):     "." + string(filepath.Separator) + filepath.Join("sub", "a.go"),
		filepath.FromSlash("/elsewhere/b.go"): filepath.FromSlash("/elsewhere/b.go"),
		filepath.FromSlash("/work"):           filepath.FromSlash("/work"),
		"relative.go":                         "relative.go",
	}
	for file, want := range tests {
		if got := RelPath(dir, file); got != want {
			t.Errorf("RelPath(%q) = %q, want %q", file, got, want)
		}
	}
}

func TestErrorSorts(t *testing.T) {
	pos := func(file string, line, col int) token.Position {
		return token.Position{Filename: file, Line: line, Column: col}
	}
	e := &Error{Dir: "/", Diags: []Diagnostic{
		{Pos: pos("/b.go", 1, 1), Msg: "b"},
		{Pos: pos("/a.go", 2, 1), Msg: "a2"},
		{Pos: pos("/a.go", 1, 5), Msg: "a1c5"},
		{Pos: pos("/a.go", 1, 2), Msg: "z"},
		{Pos: pos("/a.go", 1, 2), Msg: "y"},
	}}
	want := "./a.go:1:2: y\n./a.go:1:2: z\n./a.go:1:5: a1c5\n./a.go:2:1: a2\n./b.go:1:1: b"
	if filepath.Separator != '/' {
		t.Skip("paths in this test are Unix paths")
	}
	if got := e.Error(); got != want {
		t.Errorf("Error() =\n%s\nwant\n%s", got, want)
	}
}
