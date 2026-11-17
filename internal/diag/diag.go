// Package diag holds the diagnostics GoPatina reports: errors in Go terms,
// pointing at Go source, and the notes that explain each automatic fix.
package diag

import (
	"fmt"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
)

// A Diagnostic is one message about a position in the Go source.
type Diagnostic struct {
	Pos   token.Position
	Msg   string   // the first line, in Go terms
	Notes []string // further lines, printed indented below Msg
}

// Errorf returns a diagnostic at pos.
func Errorf(pos token.Position, format string, args ...any) Diagnostic {
	return Diagnostic{Pos: pos, Msg: fmt.Sprintf(format, args...)}
}

// With returns d with the notes appended.
func (d Diagnostic) With(notes ...string) Diagnostic {
	d.Notes = append(append([]string(nil), d.Notes...), notes...)
	return d
}

// Format renders d the way the go command prints compiler errors:
// "./main.go:14:9: message", with the file name relative to dir when the
// file is inside it, followed by its notes, each indented by four spaces.
func (d Diagnostic) Format(dir string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s:%d:%d: %s\n", RelPath(dir, d.Pos.Filename), d.Pos.Line, d.Pos.Column, d.Msg)
	for _, n := range d.Notes {
		fmt.Fprintf(&b, "    %s\n", n)
	}
	return b.String()
}

// RelPath returns file relative to dir, prefixed with "./", as the go
// command prints it, or file unchanged when it is not inside dir.
func RelPath(dir, file string) string {
	rel, err := filepath.Rel(dir, file)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return file
	}
	return "." + string(filepath.Separator) + rel
}

// Sort orders diagnostics by file, line and column, then by message, so the
// output does not depend on the order in which they were found.
func Sort(ds []Diagnostic) {
	sort.SliceStable(ds, func(i, j int) bool {
		a, b := ds[i].Pos, ds[j].Pos
		if a.Filename != b.Filename {
			return a.Filename < b.Filename
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		return ds[i].Msg < ds[j].Msg
	})
}

// Error is a list of diagnostics used as an error value.
type Error struct {
	Dir   string // the directory file names are printed relative to
	Diags []Diagnostic
}

// Error renders every diagnostic, sorted, one after the other.
func (e *Error) Error() string {
	ds := append([]Diagnostic(nil), e.Diags...)
	Sort(ds)
	var b strings.Builder
	for _, d := range ds {
		b.WriteString(d.Format(e.Dir))
	}
	return strings.TrimSuffix(b.String(), "\n")
}
